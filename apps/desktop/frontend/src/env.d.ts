/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}

// Side-effect module (registers the YAML Monarch tokenizer); ships no typings.
declare module 'monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution'

// Monaco editor contributions: side-effect modules (register actions/keybindings).
declare module 'monaco-editor/esm/vs/editor/contrib/*'
declare module 'monaco-editor/esm/vs/base/browser/ui/codicons/codiconStyles'
