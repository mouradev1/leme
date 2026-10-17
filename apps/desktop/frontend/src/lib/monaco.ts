import { watch } from 'vue'
import type * as Monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { monacoTheme, theme } from './theme'

export type MonacoApi = typeof Monaco

let loading: Promise<MonacoApi> | null = null

/**
 * Lazy, curated Monaco: core API + YAML tokenizer + the base worker (which
 * also computes diffs) + only the editing features that matter for manifests.
 * Importing 'monaco-editor' directly would pull every language, every
 * contribution and their workers (~10 MB, ~80 MB RSS in the WebView) into the
 * startup path; this is fetched the first time an editor opens.
 *
 * Schema-aware completion/validation can be layered on with `monaco-yaml`
 * fed by the cluster's /openapi/v3 schemas. Authoritative validation already
 * happens server-side via strict dry-run (see internal/resources/crud.go).
 */
export function loadMonaco(): Promise<MonacoApi> {
  loading ??= (async () => {
    const [monaco, worker] = await Promise.all([
      import('monaco-editor/esm/vs/editor/editor.api'),
      import('monaco-editor/esm/vs/editor/editor.worker?worker'),
      import('monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution'),
      // Icon font used by the find widget, folding arrows, etc. Not pulled in by
      // editor.api (only by the full 'monaco-editor' entry), so import it here.
      import('monaco-editor/esm/vs/base/browser/ui/codicons/codiconStyles'),
      // Editing features (each registers its own actions + keybindings):
      import('monaco-editor/esm/vs/editor/contrib/find/browser/findController'), // ⌘F find, ⌥⌘F replace
      import('monaco-editor/esm/vs/editor/contrib/multicursor/browser/multicursor'), // ⌘D, ⇧⌘L, ⌥click
      import('monaco-editor/esm/vs/editor/contrib/wordHighlighter/browser/wordHighlighter'),
      import('monaco-editor/esm/vs/editor/contrib/folding/browser/folding'), // fold YAML blocks
      import('monaco-editor/esm/vs/editor/contrib/bracketMatching/browser/bracketMatching'),
      import('monaco-editor/esm/vs/editor/contrib/comment/browser/comment'), // ⌘/
      import('monaco-editor/esm/vs/editor/contrib/linesOperations/browser/linesOperations'), // ⌥↑↓ move, ⇧⌥↓ copy
      import('monaco-editor/esm/vs/editor/contrib/indentation/browser/indentation'),
      import('monaco-editor/esm/vs/editor/contrib/wordOperations/browser/wordOperations'),
      import('monaco-editor/esm/vs/editor/contrib/smartSelect/browser/smartSelect'),
      import('monaco-editor/esm/vs/editor/contrib/cursorUndo/browser/cursorUndo'),
      import('monaco-editor/esm/vs/editor/contrib/clipboard/browser/clipboard'),
      import('monaco-editor/esm/vs/editor/contrib/contextmenu/browser/contextmenu'),
      import('monaco-editor/esm/vs/editor/contrib/readOnlyMessage/browser/contribution'),
    ])
    ;(globalThis as unknown as { MonacoEnvironment: object }).MonacoEnvironment = {
      getWorker: () => new worker.default(),
    }
    monaco.editor.defineTheme('leme-dark', {
      base: 'vs-dark',
      inherit: true,
      rules: [],
      colors: { 'editor.background': '#09090b' },
    })
    monaco.editor.defineTheme('leme-light', {
      base: 'vs',
      inherit: true,
      rules: [],
      colors: { 'editor.background': '#fafafa' },
    })
    // Theme is global in Monaco: follow the app theme for every open editor.
    monaco.editor.setTheme(monacoTheme())
    watch(theme, () => monaco.editor.setTheme(monacoTheme()))
    return monaco
  })()
  return loading
}
