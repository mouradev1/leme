package main

import (
	"embed"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"leme/internal/api"
	"leme/internal/cluster"
	"leme/internal/platform/desktopinstall"
	"leme/internal/platform/events"
	"leme/internal/platform/settings"
	"leme/internal/platform/shellenv"
	"leme/internal/portforward"
	"leme/internal/stream"
	"leme/internal/terminal"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// version is set at build time: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	// Low-footprint GC: a desktop UI is mostly idle, so trade a little CPU on
	// bursts for a much smaller resident set. Soft limit (not a hard cap: the
	// app never OOMs because of it); GOGC/GOMEMLIMIT env vars still override.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(96 << 20)
	}

	// Linux: running the binary from anywhere installs it as a desktop app
	// (menu entry + icon, no root). `leme --uninstall` removes it.
	if desktopinstall.HandleFlags() {
		return
	}
	if ok, err := desktopinstall.SelfInstall(appIcon); err != nil {
		log.Printf("auto-install: %v", err)
	} else if ok {
		log.Printf("Leme instalado: já aparece no menu de aplicativos")
	}

	// Must run before kubeconfig loading: exec plugins and $KUBECONFIG come
	// from the user's shell profile, which Finder/Dock launches don't source.
	if err := shellenv.Load(3 * time.Second); err != nil {
		log.Printf("shellenv: %v (continuing with inherited env)", err)
	}

	if err := settings.MigrateLegacy(); err != nil {
		log.Printf("migrating legacy settings: %v", err)
	}

	bus := events.NewBus()
	// Extra kubeconfig files the user added via the UI (any location).
	clusters, err := cluster.NewManager(bus, settings.Load().KubeconfigPaths...)
	if err != nil {
		log.Fatal(err)
	}
	app := api.New(&api.Deps{
		Bus:       bus,
		Clusters:  clusters,
		Hub:       stream.NewHub(),
		Terminals: terminal.NewManager(bus),
		Forwards:  portforward.NewManager(bus),
		Version:   version,
	})

	// macOS routes ⌘C/⌘V/⌘X/⌘Z/⌘A to text fields through the Edit menu; without
	// it copy/paste silently fails in inputs. Standard App/Edit/Window menus.
	var appMenu *menu.Menu
	if runtime.GOOS == "darwin" {
		appMenu = menu.NewMenu()
		appMenu.Append(menu.AppMenu())
		appMenu.Append(menu.EditMenu())
		appMenu.Append(menu.WindowMenu())
	}

	err = wails.Run(&options.App{
		Menu:             appMenu,
		Title:            "Leme",
		Width:            1440,
		Height:           900,
		MinWidth:         960,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 13, G: 15, B: 20, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind:             app.Bindings(),
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "leme", // matches StartupWMClass in the .desktop entry
			// Wails' safe default (some GPU drivers render a blank webview otherwise).
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
