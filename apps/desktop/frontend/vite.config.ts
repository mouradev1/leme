import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: {
    target: 'es2022', // WKWebView (macOS 12+), WebView2, WebKitGTK 2.40+
    chunkSizeWarningLimit: 4000, // monaco is big, but lazy-loaded
    rollupOptions: {
      output: {
        // Keep monaco/xterm out of the startup chunk: they're only fetched
        // the first time a YAML editor or terminal tab is opened.
        manualChunks(id) {
          // Vite's lazy-import helper must not land inside a lazy chunk, or the
          // startup bundle would statically import (and load) all of Monaco.
          if (id.includes('vite/preload-helper') || id.includes('commonjsHelpers')) return 'runtime'
          if (id.includes('node_modules/monaco-editor/')) return 'monaco'
          if (id.includes('node_modules/@xterm/')) return 'xterm'
        },
      },
    },
  },
})
