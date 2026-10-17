//go:build !linux

package desktopinstall

// On macOS/Windows the app is installed by its bundle/installer.

func HandleFlags() bool { return false }

func SelfInstall([]byte) (bool, error) { return false, nil }
