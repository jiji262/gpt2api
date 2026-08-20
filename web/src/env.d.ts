/// <reference types="vite/client" />
/// <reference types="vite-svg-loader" />

interface ImportMetaEnv {
  readonly VITE_APP_TITLE: string
  readonly VITE_API_BASE: string
}
interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

// `*.svg?component` / `*.svg?url` / `*.svg?raw` / `*.svg?skipsvgo` 由 vite-svg-loader
// 自带 types 提供（node_modules/vite-svg-loader/index.d.ts）。
// 这里只覆盖 `vite/client` 默认把 bare `*.svg` 当作 string URL 的行为 ——
// 由于 vite.config.ts 设了 `defaultImport: 'component'`，bare import 实为 Vue 组件。
declare module '*.svg' {
  import type { FunctionalComponent, SVGAttributes } from 'vue'
  const component: FunctionalComponent<SVGAttributes>
  export default component
}
