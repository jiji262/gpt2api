# UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 gpt2api/web 所有 25 个 Vue 页面从 Element Plus 默认模板切换到 Neon Solid 设计系统（深紫 Landing + 亮白 Admin + 6 纯色 + Space Grotesk × 思源黑体 + 多色手绘线稿），纯视觉重设计，不改业务逻辑。

**Architecture:** 分层实施 — 先建立 SCSS tokens + Element Plus 主题 override + 7 个共享组件作为基础，然后从 Layout 开始向外辐射到 Landing / Auth / Personal / Admin。所有业务逻辑（`<script setup>` / API 调用 / 路由 / Pinia store）保持不动，只重写 `<template>` 结构和 `<style scoped>` 样式。

**Tech Stack:** Vue 3 Composition API · Element Plus 2.7 · Vite 5 · TypeScript 5.3 · SCSS · @fontsource (自托管字体) · vite-svg-loader (SVG 作 Vue 组件)

**Spec:** [docs/superpowers/specs/2026-04-24-ui-redesign-design.md](../specs/2026-04-24-ui-redesign-design.md)

**Test 策略说明：** 本项目无 Vitest/Jest 测试基建，本次 UI 重设计不新增测试框架。每个任务的"验证"步骤 = `vue-tsc --noEmit` 类型检查 + 关键任务 `pnpm build` 构建校验 + 浏览器人工视觉检查（`pnpm dev` 或 `/browse`）。Definition of Done 条目（见 spec §10.3）在 Phase 8 集中验证。

**工作目录约定：** 所有 pnpm 命令都在 `gpt2api/web/` 下运行。用 `pnpm -C web ...` 从仓库根执行。

---

## Phase 0 · 依赖与配置

### Task 0.1: 安装字体 + SVG loader，更新 Vite 和 TS 配置

**Files:**
- Modify: `web/package.json`
- Modify: `web/vite.config.ts`
- Modify: `web/src/env.d.ts`

- [ ] **Step 1: 安装依赖**

```bash
cd 本仓库/web
pnpm add @fontsource/space-grotesk @fontsource-variable/noto-sans-sc @fontsource/jetbrains-mono
pnpm add -D vite-svg-loader
```

Expected: `package.json` 多出 4 个依赖；`pnpm-lock.yaml` 更新。

- [ ] **Step 2: 更新 vite.config.ts**

打开 `web/vite.config.ts`，在第 1 行 import 块追加：

```ts
import svgLoader from 'vite-svg-loader'
```

在 `plugins: [ vue(), ...]` 数组末尾追加一行：

```ts
svgLoader({ defaultImport: 'component' }),
```

完整 plugins 块应为：

```ts
plugins: [
  vue(),
  AutoImport({
    imports: ['vue', 'vue-router', 'pinia', '@vueuse/core'],
    resolvers: [ElementPlusResolver()],
    dts: 'src/auto-imports.d.ts',
  }),
  Components({
    resolvers: [ElementPlusResolver()],
    dts: 'src/components.d.ts',
  }),
  svgLoader({ defaultImport: 'component' }),
],
```

- [ ] **Step 3: 更新 env.d.ts 声明 SVG 作为 Vue 组件导入**

打开 `web/src/env.d.ts`，末尾追加：

```ts
declare module '*.svg?component' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent
  export default component
}

declare module '*.svg' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent
  export default component
}
```

- [ ] **Step 4: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors. (字体包暂未被引用，TS 不会报；vite-svg-loader 类型已在 env.d.ts 里声明。)

- [ ] **Step 5: 提交**

```bash
cd 本仓库
git add web/package.json web/pnpm-lock.yaml web/vite.config.ts web/src/env.d.ts
git commit -m "chore(web): 安装字体 + svg-loader 依赖，配置 Vite 支持 SVG 作为 Vue 组件"
```

---

## Phase 1 · 设计系统基础

### Task 1.1: 写设计 tokens

**Files:**
- Create: `web/src/styles/tokens.scss`

- [ ] **Step 1: 创建 tokens.scss**

写入以下完整内容到 `web/src/styles/tokens.scss`：

```scss
// ==============================================================
// Neon Solid Design Tokens
// --------------------------------------------------------------
// 该文件被其他 SCSS 通过 `@use './tokens' as *;` 消费（给 SCSS 变量），
// 同时以 :root CSS 自定义属性形式暴露给运行时和 Element Plus。
// ==============================================================

// ===== 品牌色（6 纯色）=====
$c-pink:    #FF3D94;  // 主 accent / 主按钮
$c-cyan:    #00D9FF;  // 次 accent / info
$c-yellow:  #FFD600;  // warning / 强调
$c-purple:  #A855F7;  // operation / focus
$c-orange:  #FF6B35;  // danger / admin tag
$c-green:   #00E676;  // success / active

// 主按钮 hover（darken 一档）
$c-pink-hover:   #E82A80;
$c-cyan-hover:   #00B8D9;
$c-yellow-hover: #E6C100;
$c-purple-hover: #9333EA;
$c-orange-hover: #E85A28;
$c-green-hover:  #00C766;

// ===== 中性色 =====
$n-space:   #0A0718;  // Landing 主背景（最深）
$n-ink-p:   #130A24;  // Sidebar / 暗色面板
$n-ink:     #1A1A2E;  // 亮色区正文
$n-cloud:   #FAFAFB;  // 亮色主区背景
$n-paper:   #FFFFFF;  // 亮色卡片底

// 灰阶
$gray-50:   #FAFAFB;
$gray-100:  #F4F4F5;
$gray-200:  #E5E5EA;
$gray-300:  #D4D4D8;
$gray-400:  #A1A1AA;
$gray-500:  #71717A;
$gray-600:  #52525B;
$gray-700:  #3F3F46;
$gray-800:  #27272A;
$gray-900:  #18181B;

// 暗色文字色阶
$dark-text-1: #FFFFFF;
$dark-text-2: #D8D1EC;
$dark-text-3: #B0A7C9;
$dark-text-4: #7E7593;

// 暗色描边
$dark-border:        rgba(255, 255, 255, 0.08);
$dark-border-strong: rgba(255, 255, 255, 0.15);

// ===== 字体 =====
$f-sans: "Space Grotesk", "Noto Sans SC Variable", "Noto Sans SC", -apple-system, BlinkMacSystemFont, "PingFang SC", sans-serif;
$f-mono: "JetBrains Mono", "SF Mono", Menlo, Consolas, monospace;

// ===== 形状 =====
$r-sm:   6px;
$r-md:  10px;
$r-lg:  14px;
$r-xl:  20px;
$r-pill: 999px;

$bw: 1.5px;

// ===== 阴影 =====
$sh-1: 0 1px 2px rgba(15, 15, 30, 0.04);
$sh-2: 0 4px 16px rgba(15, 15, 30, 0.08);

// ==============================================================
// CSS 变量导出
// ==============================================================
:root {
  // 颜色
  --c-pink:   #{$c-pink};
  --c-cyan:   #{$c-cyan};
  --c-yellow: #{$c-yellow};
  --c-purple: #{$c-purple};
  --c-orange: #{$c-orange};
  --c-green:  #{$c-green};

  --c-pink-hover:   #{$c-pink-hover};
  --c-cyan-hover:   #{$c-cyan-hover};
  --c-yellow-hover: #{$c-yellow-hover};
  --c-purple-hover: #{$c-purple-hover};
  --c-orange-hover: #{$c-orange-hover};
  --c-green-hover:  #{$c-green-hover};

  --n-space:   #{$n-space};
  --n-ink-p:   #{$n-ink-p};
  --n-ink:     #{$n-ink};
  --n-cloud:   #{$n-cloud};
  --n-paper:   #{$n-paper};

  --gray-100: #{$gray-100};
  --gray-200: #{$gray-200};
  --gray-300: #{$gray-300};
  --gray-400: #{$gray-400};
  --gray-500: #{$gray-500};
  --gray-600: #{$gray-600};
  --gray-700: #{$gray-700};

  // 字体
  --f-sans: #{$f-sans};
  --f-mono: #{$f-mono};

  --fs-hero: 80px;
  --fs-h1:   48px;
  --fs-h2:   32px;
  --fs-h3:   24px;
  --fs-h4:   18px;
  --fs-lead: 19px;
  --fs-body: 16px;
  --fs-ui:   14px;
  --fs-sm:   13px;
  --fs-xs:   12px;

  // 形状
  --r-sm:   #{$r-sm};
  --r-md:   #{$r-md};
  --r-lg:   #{$r-lg};
  --r-xl:   #{$r-xl};
  --r-pill: #{$r-pill};
  --bw:     #{$bw};

  --sh-1: #{$sh-1};
  --sh-2: #{$sh-2};

  // 语义别名
  --app-bg:       var(--n-cloud);
  --panel-bg:     var(--n-paper);
  --sidebar-bg:   var(--n-ink-p);
  --text-primary: var(--n-ink);
  --text-muted:   #{$gray-500};
  --border:       #{$gray-200};
  --accent:       var(--c-pink);
}

// 暗色区：Landing / Login / Register / Errors / Sidebar 的祖先或自身挂 dark-area
.dark-area {
  --app-bg:       var(--n-space);
  --panel-bg:     var(--n-ink-p);
  --text-primary: #{$dark-text-1};
  --text-muted:   #{$dark-text-3};
  --border:       #{$dark-border};
}
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/styles/tokens.scss
git commit -m "feat(web): 添加 Neon Solid 设计 tokens（6 纯色 + 字体 + 间距）"
```

---

### Task 1.2: 写字体加载 fonts.scss

**Files:**
- Create: `web/src/styles/fonts.scss`

- [ ] **Step 1: 创建 fonts.scss**

写入以下完整内容到 `web/src/styles/fonts.scss`：

```scss
// ==============================================================
// 字体自托管 - 不依赖 Google Fonts CDN
// --------------------------------------------------------------
// Space Grotesk: Landing hero / 标题 / 按钮
// Noto Sans SC Variable: 中文正文 + 标题，可变字重 100-900
// JetBrains Mono: 代码块 / API Key / 等宽数字
// ==============================================================

// Space Grotesk — 只装常用字重
@import '@fontsource/space-grotesk/400.css';
@import '@fontsource/space-grotesk/500.css';
@import '@fontsource/space-grotesk/600.css';
@import '@fontsource/space-grotesk/700.css';
@import '@fontsource/space-grotesk/800.css';

// Noto Sans SC Variable - 单文件覆盖全字重
@import '@fontsource-variable/noto-sans-sc';

// JetBrains Mono — 常规 + 中等
@import '@fontsource/jetbrains-mono/400.css';
@import '@fontsource/jetbrains-mono/600.css';
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/styles/fonts.scss
git commit -m "feat(web): 引入自托管字体（Space Grotesk / Noto Sans SC / JetBrains Mono）"
```

---

### Task 1.3: 写 Element Plus 主题 override

**Files:**
- Create: `web/src/styles/element-override.scss`

- [ ] **Step 1: 创建 element-override.scss**

写入以下完整内容到 `web/src/styles/element-override.scss`：

```scss
@use './tokens' as *;

// ==============================================================
// Element Plus 核心 CSS 变量覆盖
// --------------------------------------------------------------
// 这里只改 token 级变量，让 Element Plus 内部自己算出一整套。
// 组件级的精细覆盖放在本文件后半段。
// ==============================================================

:root {
  --el-color-primary: #{$c-pink};
  --el-color-primary-light-3: #{mix($c-pink, white, 70%)};
  --el-color-primary-light-5: #{mix($c-pink, white, 50%)};
  --el-color-primary-light-7: #{mix($c-pink, white, 30%)};
  --el-color-primary-light-8: #{mix($c-pink, white, 20%)};
  --el-color-primary-light-9: #{mix($c-pink, white, 10%)};
  --el-color-primary-dark-2: #{mix($c-pink, black, 80%)};

  --el-color-success: #{$c-green};
  --el-color-warning: #{$c-yellow};
  --el-color-danger:  #{$c-orange};
  --el-color-info:    #{$c-cyan};

  --el-border-radius-base:  #{$r-md};
  --el-border-radius-small: #{$r-sm};

  --el-font-family: #{$f-sans};
  --el-font-size-base: 14px;

  --el-text-color-primary:   #{$n-ink};
  --el-text-color-regular:   #{$gray-700};
  --el-text-color-secondary: #{$gray-500};
  --el-text-color-placeholder: #{$gray-400};

  --el-bg-color:       #{$n-paper};
  --el-bg-color-page:  #{$n-cloud};
  --el-border-color:         #{$gray-200};
  --el-border-color-light:   #{$gray-100};
  --el-border-color-lighter: #{$gray-100};
}

// ==============================================================
// 组件级精细调整
// ==============================================================

// --- Button ---
.el-button {
  border-radius: $r-md;
  font-weight: 700;
  font-family: $f-sans;
  letter-spacing: 0.01em;

  &--primary:not(.is-disabled) {
    background: $c-pink;
    border-color: $c-pink;
    &:hover,
    &:focus { background: $c-pink-hover; border-color: $c-pink-hover; }
  }
  &--success:not(.is-disabled) {
    background: $c-green; border-color: $c-green; color: $n-space;
    &:hover,
    &:focus { background: $c-green-hover; border-color: $c-green-hover; }
  }
  &--warning:not(.is-disabled) {
    background: $c-yellow; border-color: $c-yellow; color: $n-ink;
    &:hover,
    &:focus { background: $c-yellow-hover; border-color: $c-yellow-hover; }
  }
  &--danger:not(.is-disabled) {
    background: $c-orange; border-color: $c-orange;
    &:hover,
    &:focus { background: $c-orange-hover; border-color: $c-orange-hover; }
  }
}

// --- Input / Select / Textarea ---
.el-input__wrapper {
  border-radius: $r-md;
  box-shadow: 0 0 0 $bw $gray-200 inset;
  transition: box-shadow 0.15s;

  &:hover { box-shadow: 0 0 0 $bw $gray-300 inset; }
  &.is-focus {
    box-shadow: 0 0 0 $bw $c-purple inset,
                0 0 0 3px rgba(168, 85, 247, 0.15);
  }
}
.el-textarea__inner {
  border-radius: $r-md;
  box-shadow: 0 0 0 $bw $gray-200 inset;
  &:hover { box-shadow: 0 0 0 $bw $gray-300 inset; }
  &:focus {
    box-shadow: 0 0 0 $bw $c-purple inset,
                0 0 0 3px rgba(168, 85, 247, 0.15);
  }
}

// --- Table ---
.el-table {
  --el-table-header-bg-color: #{$n-cloud};
  --el-table-header-text-color: #{$gray-500};
  --el-table-row-hover-bg-color: #{$n-cloud};
  --el-table-border-color: #{$gray-100};

  border-radius: $r-lg;
  border: $bw solid $gray-200;
  overflow: hidden;

  th.el-table__cell > .cell {
    font-size: 11px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    font-weight: 700;
    color: $gray-500;
  }

  td.el-table__cell { padding: 14px 12px; }
}

// --- Card / Dialog / Drawer ---
.el-card, .el-dialog, .el-drawer__body {
  border-radius: $r-lg;
}
.el-dialog__header { padding-bottom: 12px; }
.el-dialog__title { font-weight: 800; letter-spacing: -0.01em; }

// --- Tag (保留作为 fallback；业务代码优先用 StatusTag) ---
.el-tag {
  border-radius: $r-pill;
  font-weight: 700;
  border-width: $bw;
}

// --- Message / Notification ---
.el-message, .el-notification {
  border-radius: $r-md;
  box-shadow: $sh-2;
}

// --- Pagination ---
.el-pagination {
  --el-pagination-button-color: #{$gray-600};
  --el-pagination-hover-color: #{$c-pink};
  .btn-prev, .btn-next, .el-pager li {
    border-radius: $r-sm;
    font-weight: 700;
  }
  .el-pager li.is-active { background: $c-pink; color: white; }
}

// --- Menu (侧边栏用) ---
.el-menu {
  border-right: none;
  background-color: transparent;
  .el-menu-item, .el-sub-menu__title {
    border-radius: $r-md;
    margin: 2px 0;
  }
}

// --- Dropdown ---
.el-dropdown-menu { border-radius: $r-md; padding: 6px; }
.el-dropdown-menu__item { border-radius: $r-sm; }
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/styles/element-override.scss
git commit -m "feat(web): Element Plus 主题覆盖切换到 Neon Solid 色系（主色粉，次色紫）"
```

---

### Task 1.4: 重写 global.scss

**Files:**
- Modify: `web/src/styles/global.scss`

- [ ] **Step 1: 覆盖 global.scss 完整内容**

用以下内容**完全替换** `web/src/styles/global.scss`：

```scss
@use './tokens' as *;

// ==============================================================
// Global reset & base
// --------------------------------------------------------------
// 旧的 html.dark / .page-title / .card-block / .flex-between 等
// 全部交给 tokens + 组件封装，这里只保留必要 reset。
// ==============================================================

html, body, #app {
  height: 100%;
  margin: 0;
  padding: 0;
}

html {
  color: $n-ink;
  background: $n-cloud;
  font-family: $f-sans;
  font-size: 16px;
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}

body { background: var(--app-bg); color: var(--text-primary); }

* { box-sizing: border-box; }

a { color: inherit; text-decoration: none; }

code, kbd, pre, samp { font-family: $f-mono; }

::selection { background: rgba(255, 61, 148, 0.3); color: inherit; }

// 通用两类容器（保留原有 page-container 以尽量少改 admin 模板）
.page-container { padding: 28px 32px 40px; }

// ==============================================================
// 滚动条统一风格
// ==============================================================
* {
  scrollbar-width: thin;
  scrollbar-color: rgba(144, 147, 153, 0.45) transparent;
}
*::-webkit-scrollbar { width: 8px; height: 8px; }
*::-webkit-scrollbar-track { background: transparent; }
*::-webkit-scrollbar-thumb {
  background: rgba(144, 147, 153, 0.35);
  border-radius: $r-pill;
  border: 2px solid transparent;
  background-clip: padding-box;
  transition: background .2s;
}
*::-webkit-scrollbar-thumb:hover {
  background: rgba(144, 147, 153, 0.65);
  background-clip: padding-box;
}
*::-webkit-scrollbar-corner { background: transparent; }

// 暗色区滚动条加深
.dark-area {
  scrollbar-color: rgba(255, 255, 255, 0.15) transparent;

  *::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.12);
    background-clip: padding-box;
  }
  *::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.25);
    background-clip: padding-box;
  }
}
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors. (旧的 .page-title / .card-block 被删掉，但下游组件都会重写，现阶段暂时允许引用悬空——构建仍会通过因为 CSS 名称是字符串引用。)

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/styles/global.scss
git commit -m "refactor(web): 重写 global.scss 为最小 reset + Neon Solid base"
```

---

### Task 1.5: 在 main.ts 里串起所有样式

**Files:**
- Modify: `web/src/main.ts`

- [ ] **Step 1: 调整 main.ts import 顺序**

用以下内容**完全替换** `web/src/main.ts`：

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPersist from 'pinia-plugin-persistedstate'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import * as ElementIcons from '@element-plus/icons-vue'

import App from './App.vue'
import router from './router'

// 样式加载顺序很关键：
// 1. 字体（外部资源先预热）
// 2. tokens（定义 CSS 变量）
// 3. Element Plus override（消费变量覆盖 EP）
// 4. global（消费变量做 reset + base）
import './styles/fonts.scss'
import './styles/tokens.scss'
import './styles/element-override.scss'
import './styles/global.scss'

import { useSiteStore } from './stores/site'

const app = createApp(App)

const pinia = createPinia()
pinia.use(piniaPersist)
app.use(pinia)
app.use(router)
app.use(ElementPlus, { size: 'default', locale: zhCn })

// 把 element icons 作为全局组件注册
for (const [name, comp] of Object.entries(ElementIcons)) {
  app.component(name, comp as never)
}

useSiteStore(pinia).refresh()

app.mount('#app')
```

**要点：**
- 删除了 `import 'element-plus/theme-chalk/dark/css-vars.css'`（不再用 Element 的 dark 模式）
- 新增 4 行样式 import，按顺序

- [ ] **Step 2: 构建校验**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
pnpm build
```

Expected: vue-tsc 0 errors；build 成功输出到 dist。

- [ ] **Step 3: 启 dev 看效果**

```bash
cd 本仓库/web
pnpm dev
```

手动访问 `http://localhost:5173`，确认：
- 字体是 Space Grotesk（而不是 system-ui）
- 主色按钮是粉色（不是 Element 默认蓝）
- 背景是 `#FAFAFB`

按 Ctrl+C 停止 dev。

- [ ] **Step 4: 提交**

```bash
cd 本仓库
git add web/src/main.ts
git commit -m "feat(web): main.ts 串起新样式加载顺序，移除 Element dark CSS"
```

---

## Phase 2 · 共享组件

**所有组件创建在 `web/src/components/`，unplugin-vue-components 会自动注册，无需手动 import。**

### Task 2.1: NeonButton 组件

**Files:**
- Create: `web/src/components/NeonButton.vue`

- [ ] **Step 1: 写组件**

写入以下内容到 `web/src/components/NeonButton.vue`：

```vue
<script setup lang="ts">
import { computed } from 'vue'

type Variant = 'pink' | 'cyan' | 'yellow' | 'purple' | 'green' | 'orange' | 'ink' | 'outline' | 'ghost'
type Size = 'sm' | 'md' | 'lg'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: Size
  block?: boolean
  disabled?: boolean
  tag?: 'button' | 'a'
  href?: string
}>(), {
  variant: 'pink',
  size: 'md',
  block: false,
  disabled: false,
  tag: 'button',
})

defineEmits<{ (e: 'click', ev: MouseEvent): void }>()

const classes = computed(() => [
  'neon-btn',
  `neon-btn--${props.variant}`,
  `neon-btn--${props.size}`,
  { 'is-block': props.block, 'is-disabled': props.disabled },
])
</script>

<template>
  <component
    :is="tag"
    :class="classes"
    :href="tag === 'a' ? href : undefined"
    :disabled="tag === 'button' ? disabled : undefined"
    @click="$emit('click', $event)"
  >
    <slot />
  </component>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.neon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 0;
  border-radius: $r-md;
  font-family: $f-sans;
  font-weight: 700;
  letter-spacing: 0.01em;
  cursor: pointer;
  text-decoration: none;
  transition: transform 0.15s, background 0.15s, color 0.15s, border-color 0.15s;
  user-select: none;

  &:hover:not(.is-disabled) { transform: translateY(-1px); }
  &:active:not(.is-disabled) { transform: translateY(0); }
  &.is-block { display: flex; width: 100%; }
  &.is-disabled { opacity: 0.5; cursor: not-allowed; }

  // --- sizes ---
  &--sm { height: 32px; padding: 0 14px; font-size: 13px; }
  &--md { height: 40px; padding: 0 20px; font-size: 14px; }
  &--lg { height: 48px; padding: 0 28px; font-size: 16px; }

  // --- variants ---
  &--pink   { background: $c-pink;   color: white;      &:hover:not(.is-disabled) { background: $c-pink-hover; } }
  &--cyan   { background: $c-cyan;   color: $n-space;   &:hover:not(.is-disabled) { background: $c-cyan-hover; } }
  &--yellow { background: $c-yellow; color: $n-ink;     &:hover:not(.is-disabled) { background: $c-yellow-hover; } }
  &--purple { background: $c-purple; color: white;      &:hover:not(.is-disabled) { background: $c-purple-hover; } }
  &--green  { background: $c-green;  color: $n-space;   &:hover:not(.is-disabled) { background: $c-green-hover; } }
  &--orange { background: $c-orange; color: white;      &:hover:not(.is-disabled) { background: $c-orange-hover; } }
  &--ink    { background: $n-ink;    color: white;      &:hover:not(.is-disabled) { background: $n-ink-p; } }

  &--outline {
    background: transparent;
    color: currentColor;
    border: $bw solid currentColor;
    &:hover:not(.is-disabled) { background: currentColor; color: var(--panel-bg); }
  }

  &--ghost {
    background: transparent;
    color: currentColor;
    &:hover:not(.is-disabled) { background: rgba(0, 0, 0, 0.04); }
  }
}

// 暗色区 ghost 调深
.dark-area .neon-btn--ghost:hover { background: rgba(255, 255, 255, 0.06); }
</style>
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/components/NeonButton.vue
git commit -m "feat(web): NeonButton 组件 — 9 种纯色 variant + 3 档尺寸"
```

---

### Task 2.2: StatusTag 组件

**Files:**
- Create: `web/src/components/StatusTag.vue`

- [ ] **Step 1: 写组件**

写入以下内容到 `web/src/components/StatusTag.vue`：

```vue
<script setup lang="ts">
type Variant =
  | 'pro' | 'free' | 'admin'
  | 'active' | 'disabled'
  | 'success' | 'warning' | 'danger' | 'info'
  | 'pink' | 'cyan' | 'yellow' | 'purple' | 'green' | 'orange'

withDefaults(defineProps<{
  variant?: Variant
  dot?: boolean
}>(), {
  variant: 'free',
  dot: false,
})
</script>

<template>
  <span class="status-tag" :class="`status-tag--${variant}`">
    <span v-if="dot" class="dot" />
    <slot />
  </span>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.status-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: $r-pill;
  border: $bw solid currentColor;
  background: transparent;
  line-height: 1.4;
  letter-spacing: 0.01em;
  font-family: $f-sans;

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
  }

  &--pro, &--purple    { color: $c-purple;  background: rgba(168, 85, 247, 0.08); }
  &--admin, &--orange  { color: $c-orange;  background: rgba(255, 107, 53, 0.08); }
  &--active, &--success, &--green { color: $c-green;  background: rgba(0, 230, 118, 0.08); }
  &--pink              { color: $c-pink;    background: rgba(255, 61, 148, 0.08); }
  &--cyan, &--info     { color: #0097B2;    background: rgba(0, 217, 255, 0.1); }
  &--yellow, &--warning{ color: #A68A00;    background: rgba(255, 214, 0, 0.12); }
  &--danger            { color: $c-orange;  background: rgba(255, 107, 53, 0.08); }
  &--free, &--disabled { color: $gray-500;  border-color: $gray-300; background: $gray-100; }
}

// 暗色区稍稍加亮
.dark-area .status-tag {
  &--pro, &--purple    { color: #C5A1FF; background: rgba(168, 85, 247, 0.12); }
  &--cyan, &--info     { color: $c-cyan;  background: rgba(0, 217, 255, 0.1); }
  &--yellow, &--warning{ color: $c-yellow; background: rgba(255, 214, 0, 0.1); }
  &--free, &--disabled { color: $dark-text-3; background: rgba(255, 255, 255, 0.04); border-color: $dark-border-strong; }
}
</style>
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/components/StatusTag.vue
git commit -m "feat(web): StatusTag 组件 — 15 个 variant 描边式标签"
```

---

### Task 2.3: UserAvatar 组件

**Files:**
- Create: `web/src/components/UserAvatar.vue`

- [ ] **Step 1: 写组件**

写入以下内容到 `web/src/components/UserAvatar.vue`：

```vue
<script setup lang="ts">
import { computed } from 'vue'

type Size = 'sm' | 'md' | 'lg' | 'xl'

const props = withDefaults(defineProps<{
  name?: string
  size?: Size
  // 手动指定颜色（覆盖 hash）
  color?: 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'
}>(), {
  name: '?',
  size: 'md',
})

// 简单 hash：把名字里每个 char 累加后 mod 6
function hashColor(name: string): string {
  const palette = ['pink', 'cyan', 'yellow', 'purple', 'orange', 'green']
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h + name.charCodeAt(i)) >>> 0
  return palette[h % palette.length]!
}

const initial = computed(() => (props.name || '?').trim().charAt(0).toUpperCase() || '?')
const flavor = computed(() => props.color || hashColor(props.name || 'anon'))
</script>

<template>
  <span class="avatar" :class="[`avatar--${size}`, `avatar--${flavor}`]">
    {{ initial }}
  </span>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: white;
  font-family: $f-sans;
  font-weight: 800;
  flex-shrink: 0;

  &--sm { width: 28px; height: 28px; font-size: 12px; }
  &--md { width: 38px; height: 38px; font-size: 14px; }
  &--lg { width: 52px; height: 52px; font-size: 18px; }
  &--xl { width: 72px; height: 72px; font-size: 26px; }

  &--pink   { background: $c-pink; }
  &--purple { background: $c-purple; }
  &--orange { background: $c-orange; }
  &--cyan   { background: $c-cyan;   color: $n-space; }
  &--yellow { background: $c-yellow; color: $n-ink; }
  &--green  { background: $c-green;  color: $n-space; }
}
</style>
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/components/UserAvatar.vue
git commit -m "feat(web): UserAvatar 组件 — 按首字母 hash 分配 6 色"
```

---

### Task 2.4: PageHeader 组件

**Files:**
- Create: `web/src/components/PageHeader.vue`

- [ ] **Step 1: 写组件**

写入以下内容到 `web/src/components/PageHeader.vue`：

```vue
<script setup lang="ts">
type Accent = 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'

withDefaults(defineProps<{
  crumb?: string
  title: string
  // 标题末尾被 accent color 高亮的字（如 title="用户管理" accent-word="管理"）
  accentWord?: string
  accent?: Accent
}>(), {
  accent: 'pink',
})
</script>

<template>
  <div class="page-header">
    <div class="page-header__text">
      <div v-if="crumb" class="page-header__crumb">{{ crumb }}</div>
      <h1 class="page-header__title" :class="`accent-${accent}`">
        <template v-if="accentWord && title.endsWith(accentWord)">
          {{ title.slice(0, -accentWord.length) }}<span class="page-header__accent">{{ accentWord }}</span>
        </template>
        <template v-else>{{ title }}</template>
      </h1>
    </div>
    <div class="page-header__extra"><slot name="extra" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 20px;
  margin-bottom: 24px;

  &__crumb {
    font-size: $fs-xs;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--text-muted);
    font-weight: 700;
    margin-bottom: 4px;
  }

  &__title {
    font-size: var(--fs-h2);
    line-height: 1.1;
    font-weight: 800;
    letter-spacing: -0.02em;
    margin: 0;
    color: var(--text-primary);
  }

  &__accent {
    &.accent-pink   ~ &, & { /* no-op; see below */ }
  }

  &__extra {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }
}

// 给 accent word 上色
.page-header__title.accent-pink   .page-header__accent { color: $c-pink; }
.page-header__title.accent-cyan   .page-header__accent { color: $c-cyan; }
.page-header__title.accent-yellow .page-header__accent { color: #A68A00; } // 亮底上的黄色要压深
.page-header__title.accent-purple .page-header__accent { color: $c-purple; }
.page-header__title.accent-orange .page-header__accent { color: $c-orange; }
.page-header__title.accent-green  .page-header__accent { color: $c-green; }

// 暗色区 yellow 可以用原色
.dark-area .page-header__title.accent-yellow .page-header__accent { color: $c-yellow; }
</style>
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/components/PageHeader.vue
git commit -m "feat(web): PageHeader 组件 — crumb + 大标题（可带 accent word）"
```

---

### Task 2.5: KpiCard 组件

**Files:**
- Create: `web/src/components/KpiCard.vue`

- [ ] **Step 1: 写组件**

写入以下内容到 `web/src/components/KpiCard.vue`：

```vue
<script setup lang="ts">
type Accent = 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'

withDefaults(defineProps<{
  label: string
  value: string | number
  change?: string
  changeDir?: 'up' | 'down' | 'flat'
  accent?: Accent
}>(), {
  accent: 'pink',
  changeDir: 'flat',
})
</script>

<template>
  <div class="kpi-card" :class="`kpi-card--${accent}`">
    <div class="kpi-card__label">{{ label }}</div>
    <div class="kpi-card__value">{{ value }}</div>
    <div v-if="change" class="kpi-card__change" :class="`is-${changeDir}`">{{ change }}</div>
    <div class="kpi-card__ill"><slot name="illustration" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.kpi-card {
  position: relative;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 18px 22px 20px;
  overflow: hidden;
  transition: transform 0.2s, box-shadow 0.2s;

  &:hover { transform: translateY(-2px); box-shadow: $sh-2; }

  &::before {
    content: '';
    position: absolute;
    left: 0; top: 0; bottom: 0;
    width: 5px;
  }
  &--pink::before   { background: $c-pink; }
  &--cyan::before   { background: $c-cyan; }
  &--yellow::before { background: $c-yellow; }
  &--purple::before { background: $c-purple; }
  &--orange::before { background: $c-orange; }
  &--green::before  { background: $c-green; }

  &__label {
    font-size: $fs-xs;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: $gray-500;
    font-weight: 700;
  }

  &__value {
    font-size: 34px;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: $n-ink;
    margin: 6px 0 2px;
    line-height: 1.1;
    font-family: $f-sans;
  }

  &__change {
    font-size: $fs-sm;
    font-weight: 700;
    &.is-up   { color: $c-green; &::before { content: '↗ '; } }
    &.is-down { color: $c-orange; &::before { content: '↘ '; } }
    &.is-flat { color: $gray-500; }
  }

  &__ill {
    position: absolute;
    right: 14px;
    bottom: 10px;
    opacity: 0.9;
    pointer-events: none;
  }
}
</style>
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/components/KpiCard.vue
git commit -m "feat(web): KpiCard 组件 — 左 5px 纯色条 + 右下角插图 slot"
```

---

### Task 2.6: EmptyState 组件

**Files:**
- Create: `web/src/components/EmptyState.vue`

- [ ] **Step 1: 写组件**

写入以下内容到 `web/src/components/EmptyState.vue`：

```vue
<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  desc?: string
}>(), {})
</script>

<template>
  <div class="empty-state">
    <div class="empty-state__ill"><slot name="illustration" /></div>
    <h3 class="empty-state__title">{{ title }}</h3>
    <p v-if="desc" class="empty-state__desc">{{ desc }}</p>
    <div class="empty-state__action"><slot name="action" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 48px 24px;
  min-height: 280px;

  &__ill {
    margin-bottom: 20px;
    max-width: 180px;
    :deep(svg) { width: 100%; height: auto; }
  }
  &__title {
    font-size: var(--fs-h4);
    font-weight: 800;
    color: var(--text-primary);
    margin: 0 0 8px;
    letter-spacing: -0.01em;
  }
  &__desc {
    font-size: $fs-sm;
    color: var(--text-muted);
    line-height: 1.6;
    margin: 0 0 20px;
    max-width: 380px;
  }
  &__action {
    display: flex;
    gap: 10px;
  }
}
</style>
```

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/components/EmptyState.vue
git commit -m "feat(web): EmptyState 组件 — 带插图 slot"
```

---

### Task 2.7: FeatureCard 组件

**Files:**
- Create: `web/src/components/FeatureCard.vue`

- [ ] **Step 1: 写组件**

写入以下内容到 `web/src/components/FeatureCard.vue`：

```vue
<script setup lang="ts">
type Accent = 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'

withDefaults(defineProps<{
  accent?: Accent
  kicker?: string
  title: string
}>(), {
  accent: 'pink',
})
</script>

<template>
  <div class="feature-card" :class="`feature-card--${accent}`">
    <div v-if="kicker" class="feature-card__kicker">{{ kicker }}</div>
    <h3 class="feature-card__title"><slot name="title">{{ title }}</slot></h3>
    <p class="feature-card__desc"><slot /></p>
    <div class="feature-card__ill"><slot name="illustration" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.feature-card {
  background: $n-ink-p;
  border: $bw solid $dark-border-strong;
  border-radius: $r-xl;
  padding: 28px 26px 30px;
  position: relative;
  overflow: hidden;
  min-height: 320px;
  display: flex;
  flex-direction: column;
  color: $dark-text-1;
  transition: transform 0.2s, border-color 0.2s;

  &:hover { transform: translateY(-4px); }

  &--pink   { border-color: $c-pink;   .feature-card__kicker { color: $c-pink; } }
  &--cyan   { border-color: $c-cyan;   .feature-card__kicker { color: $c-cyan; } }
  &--yellow { border-color: $c-yellow; .feature-card__kicker { color: $c-yellow; } }
  &--purple { border-color: $c-purple; .feature-card__kicker { color: $c-purple; } }
  &--orange { border-color: $c-orange; .feature-card__kicker { color: $c-orange; } }
  &--green  { border-color: $c-green;  .feature-card__kicker { color: $c-green; } }

  &__kicker {
    font-size: $fs-xs;
    letter-spacing: 0.18em;
    text-transform: uppercase;
    font-weight: 800;
  }
  &__title {
    font-size: var(--fs-h3);
    font-weight: 800;
    margin: 12px 0 10px;
    letter-spacing: -0.01em;
    line-height: 1.15;
  }
  &__desc {
    font-size: var(--fs-ui);
    color: $dark-text-3;
    line-height: 1.6;
    margin: 0;
  }
  &__ill {
    margin-top: auto;
    padding-top: 18px;
    align-self: flex-end;
    :deep(svg) { width: 130px; height: auto; }
  }
}
</style>
```

- [ ] **Step 2: 构建校验**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
pnpm build
```

Expected: 都成功。

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/components/FeatureCard.vue
git commit -m "feat(web): FeatureCard 组件 — 暗色 + 6 主色边框 + illustration slot"
```

---

## Phase 3 · 布局

### Task 3.1: BlankLayout 加 dark-area

**Files:**
- Modify: `web/src/layouts/BlankLayout.vue`

- [ ] **Step 1: 先读一下当前内容**

```bash
cat 本仓库/web/src/layouts/BlankLayout.vue
```

- [ ] **Step 2: 用以下内容完全替换**

```vue
<script setup lang="ts">
</script>

<template>
  <div class="blank-layout dark-area">
    <router-view />
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.blank-layout {
  min-height: 100vh;
  background: $n-space;
  color: $dark-text-1;
}
</style>
```

- [ ] **Step 3: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 4: 提交**

```bash
cd 本仓库
git add web/src/layouts/BlankLayout.vue
git commit -m "refactor(web): BlankLayout 挂 dark-area，恒暗底"
```

---

### Task 3.2: BasicLayout 重写

**Files:**
- Modify: `web/src/layouts/BasicLayout.vue`

- [ ] **Step 1: 用以下内容完全替换 BasicLayout.vue**

```vue
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'
import type { MenuItem } from '@/api/auth'

const store = useUserStore()
const site = useSiteStore()
const router = useRouter()
const route = useRoute()

const siteName = computed(() => site.get('site.name', 'GPT2API'))
const siteLogo = computed(() => site.get('site.logo_url', ''))

const { menu, user, role } = storeToRefs(store)
const collapsed = ref(false)
const loadingMenu = ref(false)

const activePath = computed(() => route.path)

const titleMap = computed(() => {
  const m = new Map<string, string>()
  function walk(items: MenuItem[]) {
    for (const it of items) {
      if (it.path) m.set(it.path, it.title)
      if (it.children) walk(it.children)
    }
  }
  walk(menu.value)
  return m
})

const currentTitle = computed(
  () => titleMap.value.get(activePath.value) || (route.meta.title as string) || '',
)

// 面包屑小字（管理员 / 用户）— 取一层父级
const crumb = computed(() => {
  if (activePath.value.startsWith('/admin')) return '管理员'
  if (activePath.value.startsWith('/personal')) return '个人中心'
  return ''
})

async function loadMenu() {
  if (menu.value.length > 0) return
  loadingMenu.value = true
  try { await store.fetchMenu() } finally { loadingMenu.value = false }
}

async function logout() {
  await store.logout()
  router.replace('/login')
}

function goto(path?: string) { if (path) router.push(path) }

onMounted(loadMenu)
watch(() => store.isLoggedIn, (v) => { if (v) loadMenu() })
</script>

<template>
  <el-container class="layout-root">
    <!-- ===== Sidebar ===== -->
    <el-aside :width="collapsed ? '64px' : '240px'" class="sidebar dark-area">
      <div class="logo">
        <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
        <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
        <span v-if="!collapsed" class="logo-name">{{ siteName }}</span>
      </div>
      <el-menu
        :default-active="activePath"
        :collapse="collapsed"
        class="side-menu"
        router
      >
        <template v-for="group in menu" :key="group.key">
          <el-menu-item v-if="!group.children?.length && group.path" :index="group.path">
            <el-icon v-if="group.icon"><component :is="group.icon" /></el-icon>
            <template #title>{{ group.title }}</template>
          </el-menu-item>
          <el-sub-menu v-else-if="group.children?.length" :index="group.key">
            <template #title>
              <el-icon v-if="group.icon"><component :is="group.icon" /></el-icon>
              <span>{{ group.title }}</span>
            </template>
            <el-menu-item
              v-for="child in group.children"
              :key="child.key"
              :index="child.path!"
            >
              <el-icon v-if="child.icon"><component :is="child.icon" /></el-icon>
              <template #title>{{ child.title }}</template>
            </el-menu-item>
          </el-sub-menu>
        </template>
      </el-menu>
    </el-aside>

    <el-container>
      <!-- ===== Topbar ===== -->
      <el-header class="topbar">
        <div class="topbar__left">
          <el-button link @click="collapsed = !collapsed" class="collapse-btn">
            <el-icon :size="18"><component :is="collapsed ? 'Expand' : 'Fold'" /></el-icon>
          </el-button>
          <div class="topbar__title-block">
            <div v-if="crumb" class="topbar__crumb">{{ crumb }}</div>
            <span class="topbar__title">{{ currentTitle }}</span>
          </div>
        </div>
        <div class="topbar__right">
          <el-dropdown trigger="click" @command="(c: string) => c === 'logout' ? logout() : goto(c)">
            <span class="user-entry">
              <UserAvatar :name="user?.nickname || user?.email || 'U'" size="sm" />
              <span class="nick">{{ user?.nickname || user?.email }}</span>
              <StatusTag v-if="role === 'admin'" variant="admin">管理员</StatusTag>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="/personal/dashboard">
                  <el-icon><User /></el-icon> 个人中心
                </el-dropdown-item>
                <el-dropdown-item command="/personal/billing">
                  <el-icon><Wallet /></el-icon> 账单
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon> 退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main" v-loading="loadingMenu">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.layout-root { height: 100vh; }

.sidebar {
  background: $n-ink-p;
  transition: width 0.2s;
  overflow-x: hidden;
  border-right: 1px solid $dark-border;
}

.logo {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  color: $dark-text-1;
  font-weight: 800;
  letter-spacing: -0.01em;

  .logo-img {
    width: 32px; height: 32px; border-radius: $r-md; object-fit: contain; background: white;
  }
  .logo-mark {
    display: inline-flex;
    width: 32px; height: 32px;
    border-radius: $r-md;
    background: $c-pink;
    color: white;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    font-weight: 900;
  }
  .logo-name { font-size: 17px; }
}

.side-menu {
  border-right: none;
  padding: 8px;

  // 覆盖 Element Menu 的色系使其贴合暗紫 sidebar
  :deep(.el-menu-item),
  :deep(.el-sub-menu__title) {
    height: 44px;
    line-height: 44px;
    color: $dark-text-3;
    font-weight: 500;
    font-size: 14px;

    &:hover {
      background-color: rgba(255, 255, 255, 0.04) !important;
      color: $dark-text-1;
    }
  }

  :deep(.el-menu-item.is-active) {
    background-color: rgba(255, 61, 148, 0.12) !important;
    color: $dark-text-1;
    font-weight: 700;
    border-left: 3px solid $c-pink;
    padding-left: 17px !important; // 默认 20px，减 3px 补偿 border
  }

  :deep(.el-sub-menu .el-menu-item) {
    padding-left: 50px !important;
    &.is-active { padding-left: 47px !important; }
  }
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  background: $n-paper;
  color: $n-ink;
  border-bottom: 1px solid $gray-100;
  padding: 0 24px;

  &__left { display: flex; align-items: center; gap: 14px; }
  &__right { display: inline-flex; align-items: center; gap: 12px; }

  &__title-block { display: flex; flex-direction: column; line-height: 1.1; }
  &__crumb {
    font-size: $fs-xs;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    font-weight: 700;
    color: $gray-500;
  }
  &__title { font-size: 16px; font-weight: 700; color: $n-ink; letter-spacing: -0.01em; }

  .collapse-btn { color: $gray-600; }

  .user-entry {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    color: $n-ink;
    font-size: 14px;
    .nick { font-weight: 500; }
  }
}

.main {
  background: $n-cloud;
  padding: 0;
}

.fade-enter-active, .fade-leave-active { transition: opacity 0.15s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
```

**关键变化：**
- `class="sidebar dark-area"` 让 sidebar 挂上暗色 token
- logo-mark 改成单色 $c-pink（原来是渐变）
- 删除 `ui.isDark` import 和 dark toggle 按钮
- 侧边栏 active 态 3px pink 左边线
- 用 `<UserAvatar>` 和 `<StatusTag>` 替代内联样式
- topbar crumb 两行布局

- [ ] **Step 2: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: 提交**

```bash
cd 本仓库
git add web/src/layouts/BasicLayout.vue
git commit -m "refactor(web): BasicLayout 切 Neon Solid — dark sidebar + pink active border + 移除 dark toggle"
```

---

### Task 3.3: 清理 stores/ui.ts 的 isDark 导出

**Files:**
- Modify: `web/src/stores/ui.ts`

- [ ] **Step 1: 确认没有其他消费点**

```bash
cd 本仓库
grep -rn "ui.isDark\|ui.toggleDark\|useUIStore" web/src --include="*.vue" --include="*.ts"
```

Expected: 只剩 `Home.vue`（下一阶段重写会删）和 `BasicLayout.vue` 已清除 toggle；其他文件无消费。若有其他输出，逐一列出再决定。

- [ ] **Step 2: 用以下内容完全替换 `web/src/stores/ui.ts`**

```ts
import { defineStore } from 'pinia'
import { useDark } from '@vueuse/core'

/**
 * UI 偏好。
 *
 * 历史上这里通过 Element Plus 的 html.dark 类切换全局 dark 模式。
 * 新的设计系统（Neon Solid）采用"固定分区"的暗/亮策略：
 *   - Landing / Auth / Errors 恒暗（布局自带 class="dark-area"）
 *   - Personal / Admin 恒亮
 * 所以用户可切换 dark 模式的入口已全部移除，但保留 useDark 自身以免其他
 * 第三方（如 Element Plus 内部组件）依赖；恒为 false。
 */
export const useUIStore = defineStore('ui', () => {
  // 强制 light，不再让用户切换
  const isDark = useDark({
    selector: 'html',
    attribute: 'class',
    valueDark: 'dark',
    valueLight: '',
    storageKey: 'gpt2api.theme',
    initialValue: 'light',
  })
  return { isDark }
})
```

**关键变化：**
- 删除 `useToggle` 导入和 `toggleDark` 导出
- 保留 `isDark` state 以免上游依赖崩，但不再暴露切换方法
- 初始值显式为 light

- [ ] **Step 3: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors（Home.vue 仍有 `ui.toggleDark()` 引用，下一任务会同步清除；如果现在报 TS 错，先把 Home.vue 里的 toggle 代码注释掉让 ts 过）。

如果 TS 报 `Property 'toggleDark' does not exist`，编辑 `Home.vue` 第 70-74 行把那个按钮 `<el-button>` 整段删掉（下一个任务 4.2 会重写整个文件，此处只是占位），然后重跑 `pnpm exec vue-tsc --noEmit`。

- [ ] **Step 4: 提交**

```bash
cd 本仓库
git add web/src/stores/ui.ts web/src/views/landing/Home.vue  # 若第 3 步改了 Home 临时去掉 toggle
git commit -m "refactor(web): 移除 UI store 的 dark toggle 暴露，恒为 light"
```

---

## Phase 4 · Landing & Auth

### Task 4.1: Hero + Feature 插图 SVG

**Files:**
- Create: `web/src/assets/illustrations/hero-palette.svg`
- Create: `web/src/assets/illustrations/feature-img2.svg`
- Create: `web/src/assets/illustrations/feature-batch.svg`
- Create: `web/src/assets/illustrations/feature-openai.svg`

- [ ] **Step 1: 创建目录并写 hero-palette.svg**

```bash
cd 本仓库/web
mkdir -p src/assets/illustrations
```

`web/src/assets/illustrations/hero-palette.svg`：

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 460 360" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="60" y="110" width="200" height="160" rx="10" stroke="#FF3D94" stroke-width="2.2"/>
  <rect x="110" y="80" width="200" height="160" rx="10" stroke="#FFD600" stroke-width="2.2"/>
  <rect x="160" y="50" width="200" height="160" rx="10" stroke="#00D9FF" stroke-width="2.2"/>
  <path d="M340 240 L405 290 L425 275 L360 220 Z" stroke="#A855F7" stroke-width="2.4"/>
  <path d="M350 235 L340 240" stroke="#A855F7" stroke-width="2"/>
  <path d="M30 90 l6 0 M33 87 l0 6" stroke="#FF3D94" stroke-width="2"/>
  <path d="M40 180 l5 0 M42.5 177.5 l0 5" stroke="#00D9FF" stroke-width="2"/>
  <path d="M420 120 l5 0 M422.5 117.5 l0 5" stroke="#FFD600" stroke-width="2"/>
  <path d="M440 200 l4 0 M442 198 l0 4" stroke="#A855F7" stroke-width="2"/>
  <path d="M20 300 l5 0 M22.5 297.5 l0 5" stroke="#FF3D94" stroke-width="2"/>
  <path d="M70 330 l4 0 M72 328 l0 4" stroke="#FFD600" stroke-width="2"/>
  <path d="M370 340 l5 0 M372.5 337.5 l0 5" stroke="#00D9FF" stroke-width="2"/>
  <path d="M15 150 Q35 120 55 135" stroke="#A855F7" stroke-width="1.8" opacity="0.6"/>
  <path d="M405 330 Q430 310 450 320" stroke="#FF3D94" stroke-width="1.8" opacity="0.6"/>
  <path d="M400 50 L405 60 L415 62 L407 68 L410 78 L400 72 L390 78 L393 68 L385 62 L395 60 Z" stroke="#FFD600" stroke-width="1.6"/>
</svg>
```

- [ ] **Step 2: feature-img2.svg**

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="30" y="30" width="120" height="80" rx="6" stroke="#FF3D94" stroke-width="2"/>
  <path d="M40 80 Q65 50 90 70 T140 55" stroke="#00D9FF" stroke-width="1.8"/>
  <circle cx="110" cy="58" r="6" stroke="#FFD600" stroke-width="1.8"/>
  <path d="M140 25 L160 5 L168 13 L148 33 Z" stroke="#A855F7" stroke-width="2"/>
  <path d="M12 65 l3 0 M13.5 63.5 l0 3" stroke="#FF3D94" stroke-width="1.8"/>
  <path d="M165 85 l3 0 M166.5 83.5 l0 3" stroke="#00D9FF" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 3: feature-batch.svg**

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="25" y="50" width="75" height="55" rx="5" stroke="#FF3D94" stroke-width="2"/>
  <rect x="55" y="35" width="75" height="55" rx="5" stroke="#FFD600" stroke-width="2"/>
  <rect x="85" y="20" width="75" height="55" rx="5" stroke="#00D9FF" stroke-width="2"/>
  <path d="M12 100 l3 0 M13.5 98.5 l0 3" stroke="#A855F7" stroke-width="1.8"/>
  <path d="M165 55 l3 0 M166.5 53.5 l0 3" stroke="#FF3D94" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 4: feature-openai.svg**

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="20" y="50" width="45" height="40" rx="4" stroke="#00D9FF" stroke-width="2"/>
  <path d="M65 65 L100 65 M65 75 L100 75" stroke="#FFD600" stroke-width="2"/>
  <rect x="100" y="50" width="60" height="40" rx="4" stroke="#FF3D94" stroke-width="2"/>
  <circle cx="115" cy="70" r="4" stroke="#A855F7" stroke-width="1.8"/>
  <circle cx="145" cy="70" r="4" stroke="#A855F7" stroke-width="1.8"/>
  <path d="M12 35 l3 0 M13.5 33.5 l0 3" stroke="#FF3D94" stroke-width="1.8"/>
  <path d="M165 110 l3 0 M166.5 108.5 l0 3" stroke="#00D9FF" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 5: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 6: 提交**

```bash
cd 本仓库
git add web/src/assets/illustrations/
git commit -m "feat(web): 添加 Hero + 3 Feature Card 的多色 Neon 线稿 SVG"
```

---

### Task 4.2: Landing Home.vue 全量重写

**Files:**
- Modify: `web/src/views/landing/Home.vue`

- [ ] **Step 1: 用以下完整内容替换 `web/src/views/landing/Home.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'
import HeroIll from '@/assets/illustrations/hero-palette.svg?component'
import FeatImg2 from '@/assets/illustrations/feature-img2.svg?component'
import FeatBatch from '@/assets/illustrations/feature-batch.svg?component'
import FeatOpenai from '@/assets/illustrations/feature-openai.svg?component'

const router = useRouter()
const user = useUserStore()
const site = useSiteStore()

const siteName = computed(() => site.get('site.name', 'GPT2API'))
const siteLogo = computed(() => site.get('site.logo_url', ''))
const allowRegister = computed(() => site.allowRegister())
const loggedIn = computed(() => user.isLoggedIn)

function goPlay() {
  if (loggedIn.value) router.push('/personal/play')
  else router.push('/login?redirect=/personal/play')
}
function goDashboard() { router.push('/personal/dashboard') }
function goLogin() { router.push('/login') }
function goRegister() { router.push('/register') }
function scrollTop() { window.scrollTo({ top: 0, behavior: 'smooth' }) }

const scrolled = ref(false)
onMounted(() => {
  const onScroll = () => { scrolled.value = window.scrollY > 24 }
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
})
</script>

<template>
  <div class="landing dark-area">
    <!-- ============= 顶部导航 ============= -->
    <header class="nav" :class="{ scrolled }">
      <div class="nav-inner">
        <a class="logo" @click="scrollTop">
          <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
          <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
          <span class="logo-name">{{ siteName }}</span>
        </a>
        <div class="nav-links">
          <a href="#features">产品</a>
          <a href="#showcase">Playground</a>
          <a @click="router.push('/personal/docs')">文档</a>
        </div>
        <div class="nav-actions">
          <template v-if="!loggedIn">
            <NeonButton variant="ghost" size="sm" @click="goLogin">登录</NeonButton>
            <NeonButton v-if="allowRegister" variant="pink" size="sm" @click="goRegister">
              免费注册 →
            </NeonButton>
          </template>
          <template v-else>
            <NeonButton variant="pink" size="sm" @click="goDashboard">进入控制台 →</NeonButton>
          </template>
        </div>
      </div>
    </header>

    <!-- ============= Hero ============= -->
    <section class="hero">
      <div class="hero__text">
        <div class="eyebrow"><span class="dot" />IMG2 · 终稿直出 · OpenAI 兼容</div>
        <h1 class="hero__title">
          给你的 <span class="c-pink">AI</span><br>
          一个<span class="c-yellow">调色盘</span>。
        </h1>
        <p class="hero__lead">
          基于 chatgpt.com 的 OpenAI 兼容 SaaS 网关。多账号池、代理池、IMG2 终稿直出、本地 2K / 4K 高清放大 — 一个 <code>base_url</code> 全部接入。
        </p>
        <div class="hero__ctas">
          <NeonButton variant="pink" size="lg" @click="goPlay">开始使用 →</NeonButton>
          <NeonButton variant="outline" size="lg" tag="a" href="/personal/docs">查看文档</NeonButton>
        </div>
        <div class="hero__stats">
          <div class="stat"><div class="num">2,384+</div><div class="desc">开发者</div></div>
          <div class="stat"><div class="num">12M</div><div class="desc">API 调用</div></div>
          <div class="stat"><div class="num">99.9%</div><div class="desc">成功率</div></div>
          <div class="stat"><div class="num">&lt; 30s</div><div class="desc">P95 延迟</div></div>
        </div>
      </div>
      <div class="hero__ill"><HeroIll /></div>
    </section>

    <!-- ============= Features ============= -->
    <section class="features" id="features">
      <div class="sec-head">
        <div class="eyebrow" style="color: var(--c-cyan)"><span class="dot c-cyan" />CORE FEATURES</div>
        <h2>三件事，<span class="c-pink">做到极致</span>。</h2>
        <p>IMG2 终稿直出 · 批量多比例 · OpenAI 零改造接入。每一项都是我们反复打磨的核心能力。</p>
      </div>
      <div class="feat-grid">
        <FeatureCard accent="pink" kicker="IMG2 PROTOCOL" title="终稿直出，不悄悄重试">
          全面对齐 <code>picture_v2</code> 正式协议，SSE 够数即返回，60s 短轮询补齐。出错第一时间暴露给调用方。
          <template #illustration><FeatImg2 /></template>
        </FeatureCard>
        <FeatureCard accent="cyan" kicker="BATCH & RATIOS" title="10 种比例，N 张同出">
          21:9 / 16:9 / 4:3 / 1:1 / 9:16 一键切换。一次调用批量返回，同 prompt 可出多变体。
          <template #illustration><FeatBatch /></template>
        </FeatureCard>
        <FeatureCard accent="yellow" kicker="OPENAI COMPAT" title="改一行 base_url，即刻接入">
          <code>/v1/images/generations</code> · <code>/v1/images/edits</code> 原样对齐官方 SDK。切网关只需一行代码。
          <template #illustration><FeatOpenai /></template>
        </FeatureCard>
      </div>
    </section>

    <!-- ============= Showcase（保留原截图 gallery） ============= -->
    <section class="showcase" id="showcase">
      <div class="sec-head">
        <div class="eyebrow" style="color: var(--c-yellow)"><span class="dot c-yellow" />SEE IT IN ACTION</div>
        <h2>在线体验<span class="c-cyan">截图</span></h2>
        <p>Playground 可直接出图，无须写代码。</p>
      </div>
      <div class="shot-grid">
        <div class="shot">
          <img src="/docs/screenshots/playground-batch.png" alt="Playground · IMG2 批量出图" />
        </div>
      </div>
    </section>

    <!-- ============= CTA ============= -->
    <section class="cta">
      <h2>准备好开始了吗？</h2>
      <p>几分钟内注册账号，生成第一个 API Key，开始调用 IMG2。</p>
      <div class="cta-ctas">
        <NeonButton v-if="!loggedIn && allowRegister" variant="pink" size="lg" @click="goRegister">
          免费注册 →
        </NeonButton>
        <NeonButton v-else-if="loggedIn" variant="pink" size="lg" @click="goDashboard">
          进入控制台 →
        </NeonButton>
        <NeonButton variant="outline" size="lg" tag="a" href="/personal/docs">查看文档</NeonButton>
      </div>
    </section>

    <!-- ============= Footer ============= -->
    <footer class="footer">
      <div class="footer-inner">
        <div class="footer-brand">
          <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
          <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
          <span class="logo-name">{{ siteName }}</span>
        </div>
        <div class="footer-copy">© {{ new Date().getFullYear() }} {{ siteName }} · OpenAI-Compatible Gateway</div>
      </div>
    </footer>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.landing {
  min-height: 100vh;
  background: $n-space;
  color: $dark-text-1;
  font-family: $f-sans;
}

// ====== NAV ======
.nav {
  position: sticky;
  top: 0;
  z-index: 20;
  backdrop-filter: blur(12px);
  background: transparent;
  transition: background 0.15s, border-color 0.15s;
  border-bottom: 1px solid transparent;

  &.scrolled {
    background: rgba(10, 7, 24, 0.85);
    border-bottom-color: $dark-border;
  }

  .nav-inner {
    max-width: 1200px;
    margin: 0 auto;
    padding: 18px 32px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 32px;
  }

  .logo {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    font-weight: 800;
    font-size: 18px;
    letter-spacing: -0.01em;
    color: $dark-text-1;
    .logo-img { width: 28px; height: 28px; border-radius: $r-md; object-fit: contain; background: white; }
    .logo-mark {
      width: 28px; height: 28px; border-radius: $r-md;
      background: $c-pink; color: white;
      display: inline-flex; align-items: center; justify-content: center;
      font-weight: 900;
    }
  }

  .nav-links {
    display: flex;
    gap: 28px;
    font-size: 14px;
    font-weight: 500;
    color: $dark-text-3;
    a { cursor: pointer; color: inherit; &:hover { color: $dark-text-1; } }
  }

  .nav-actions { display: inline-flex; gap: 10px; align-items: center; }
}

// ====== HERO ======
.hero {
  max-width: 1200px;
  margin: 0 auto;
  padding: 80px 32px 80px;
  display: grid;
  grid-template-columns: 1.3fr 1fr;
  gap: 60px;
  align-items: center;

  &__text { max-width: 640px; }
  &__title {
    font-size: var(--fs-hero);
    font-weight: 800;
    letter-spacing: -0.035em;
    line-height: 0.98;
    margin: 20px 0 28px;
    .c-pink   { color: $c-pink; }
    .c-yellow { color: $c-yellow; }
  }
  &__lead {
    font-size: var(--fs-lead);
    line-height: 1.55;
    color: $dark-text-2;
    margin: 0 0 36px;
    code { font-family: $f-mono; background: rgba(255,255,255,0.06); padding: 1px 6px; border-radius: $r-sm; font-size: 0.9em; }
  }
  &__ctas { display: flex; gap: 12px; flex-wrap: wrap; }

  &__stats {
    display: flex;
    gap: 40px;
    margin-top: 60px;
    flex-wrap: wrap;
    .stat {
      border-left: 2px solid rgba(255, 61, 148, 0.5);
      padding-left: 14px;
      .num {
        font-size: 26px; font-weight: 800; letter-spacing: -0.02em; color: $dark-text-1;
      }
      .desc { font-size: $fs-sm; color: $dark-text-3; margin-top: 2px; }
    }
  }

  &__ill :deep(svg) { width: 100%; height: auto; }
}

.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: $fs-xs;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  font-weight: 800;
  color: $c-pink;
  .dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: currentColor;
  }
  .dot.c-cyan   { background: $c-cyan; }
  .dot.c-yellow { background: $c-yellow; }
}

// ====== FEATURES ======
.features {
  max-width: 1200px;
  margin: 0 auto;
  padding: 80px 32px;
}
.sec-head {
  text-align: center;
  margin-bottom: 48px;
  h2 {
    font-size: var(--fs-h1);
    font-weight: 800;
    letter-spacing: -0.03em;
    line-height: 1.05;
    margin: 10px 0 14px;
    .c-pink { color: $c-pink; }
    .c-cyan { color: $c-cyan; }
  }
  p {
    color: $dark-text-3;
    font-size: 17px;
    max-width: 600px;
    margin: 0 auto;
    line-height: 1.6;
  }
  .eyebrow { justify-content: center; }
}

.feat-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 22px; }

// ====== SHOWCASE ======
.showcase {
  max-width: 1200px;
  margin: 0 auto;
  padding: 60px 32px;

  .shot-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 20px;
    img {
      width: 100%;
      border-radius: $r-xl;
      border: $bw solid $dark-border;
      display: block;
    }
  }
}

// ====== CTA ======
.cta {
  max-width: 900px;
  margin: 40px auto 80px;
  padding: 60px 32px;
  background: $n-ink-p;
  border: $bw solid $dark-border-strong;
  border-radius: $r-xl;
  text-align: center;

  h2 {
    font-size: var(--fs-h1);
    font-weight: 800;
    letter-spacing: -0.03em;
    margin: 0 0 14px;
    color: $dark-text-1;
  }
  p {
    color: $dark-text-3;
    font-size: 17px;
    margin: 0 0 32px;
  }
  .cta-ctas { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
}

// ====== FOOTER ======
.footer {
  border-top: $bw solid $dark-border;
  padding: 32px;
  .footer-inner {
    max-width: 1200px;
    margin: 0 auto;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 20px;
    flex-wrap: wrap;
  }
  .footer-brand {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-weight: 800;
    color: $dark-text-1;
    .logo-mark {
      width: 24px; height: 24px; border-radius: $r-sm;
      background: $c-pink; color: white;
      display: inline-flex; align-items: center; justify-content: center;
      font-size: 12px; font-weight: 900;
    }
  }
  .footer-copy { color: $dark-text-3; font-size: $fs-sm; }
}

// 响应式微调
@media (max-width: 900px) {
  .hero { grid-template-columns: 1fr; gap: 40px; padding: 60px 24px; }
  .feat-grid { grid-template-columns: 1fr; }
  .sec-head h2 { font-size: 36px; }
  .hero__title { font-size: 56px; }
}
</style>
```

- [ ] **Step 2: 构建校验**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
pnpm build
```

Expected: 都成功。

- [ ] **Step 3: 启 dev 人工验证**

```bash
cd 本仓库/web
pnpm dev
```

打开 `http://localhost:5173/`，确认：
- Hero 大字 80px，「AI」是粉色，「调色盘」是黄色
- 三张 FeatureCard 边框分别是 pink / cyan / yellow
- 每张卡有一个多色 SVG 插图
- 滚动到顶部，nav 背景从透明变成半透明深紫
- 按钮 hover 有 translateY(-1px)
- 没有任何渐变（视觉上）
- Console 无报错

Ctrl+C 停止。

- [ ] **Step 4: 提交**

```bash
cd 本仓库
git add web/src/views/landing/Home.vue
git commit -m "refactor(web): Landing Home 全量重写 — Neon Solid + 6 纯色 + SVG 插图"
```

---

### Task 4.3: Login.vue 全量重写

**Files:**
- Modify: `web/src/views/auth/Login.vue`

- [ ] **Step 1: 先读原文件确认保留的业务逻辑**

```bash
cat 本仓库/web/src/views/auth/Login.vue | head -60
```

- [ ] **Step 2: 用以下内容完全替换 Login.vue**

```vue
<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import type { FormInstance } from 'element-plus'
import { ElMessage } from 'element-plus'
import { useRouter, useRoute } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'

const router = useRouter()
const route = useRoute()
const store = useUserStore()
const site = useSiteStore()

const siteName = computed(() => site.get('site.name', 'GPT2API'))
const siteDesc = computed(() =>
  site.get('site.description', '基于 chatgpt.com 的 OpenAI 兼容网关 · IMG2 终稿直出 · 批量出图'),
)
const siteLogo = computed(() => site.get('site.logo_url', ''))
const allowRegister = computed(() => site.allowRegister())

const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  email: '',
  password: '',
})

const rules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email' as const, message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '至少 6 位', trigger: 'blur' },
  ],
}

async function onSubmit() {
  if (!formRef.value) return
  const ok = await formRef.value.validate().catch(() => false)
  if (!ok) return
  loading.value = true
  try {
    await store.login(form.email, form.password)
    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || '/personal/dashboard'
    router.replace(redirect)
  } catch {
    // 错误已由 axios 拦截器 toast
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <!-- 左栏 · 暗色品牌 -->
    <div class="auth-brand dark-area">
      <div class="brand-logo">
        <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
        <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
        <span class="logo-name">{{ siteName }}</span>
      </div>
      <h1 class="brand-title">
        欢迎<br><span class="c-pink">回来</span>。
      </h1>
      <p class="brand-sub">{{ siteDesc }}</p>
      <ul class="brand-features">
        <li>多账号池 · 多代理池 · 高并发调度</li>
        <li>RBAC 权限 · 全链路审计</li>
        <li>积分钱包 · 预扣结算 · 用量透明</li>
      </ul>
      <!-- 装饰线稿 -->
      <svg class="brand-deco" width="180" height="120" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="55" cy="70" r="22" stroke="#FF3D94" stroke-width="2"/>
        <circle cx="55" cy="70" r="8" stroke="#FF3D94" stroke-width="2"/>
        <path d="M77 70 L160 70 M140 70 L140 85 M120 70 L120 80" stroke="#00D9FF" stroke-width="2"/>
        <path d="M10 35 l4 0 M12 33 l0 4" stroke="#FFD600" stroke-width="1.8"/>
        <path d="M145 30 l3 0 M146.5 28.5 l0 3" stroke="#A855F7" stroke-width="1.8"/>
      </svg>
    </div>

    <!-- 右栏 · 白色表单 -->
    <div class="auth-form">
      <div class="auth-form__inner">
        <h2 class="auth-form__title">登录</h2>
        <p v-if="allowRegister" class="auth-form__sub">
          还没账号？<a class="link" @click="router.push('/register')">立即注册 →</a>
        </p>
        <p v-else class="auth-form__sub">请使用管理员分配的账号登录</p>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          size="large"
          label-position="top"
          class="auth-form__form"
          @submit.prevent="onSubmit"
        >
          <el-form-item label="邮箱" prop="email">
            <el-input v-model="form.email" placeholder="you@example.com" autocomplete="email" />
          </el-form-item>
          <el-form-item label="密码" prop="password">
            <el-input v-model="form.password" type="password" show-password placeholder="至少 6 位" autocomplete="current-password" />
          </el-form-item>
          <NeonButton variant="pink" size="lg" :block="true" :disabled="loading" @click="onSubmit">
            {{ loading ? '登录中…' : '登录 →' }}
          </NeonButton>
        </el-form>

        <p class="auth-form__foot">受 Cloudflare Turnstile 保护</p>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.auth-page {
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 100vh;
}

.auth-brand {
  background: $n-ink-p;
  color: $dark-text-1;
  padding: 72px 60px;
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;

  .brand-logo {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-weight: 800;
    font-size: 18px;
    margin-bottom: 40px;
    .logo-img { width: 32px; height: 32px; border-radius: $r-md; object-fit: contain; background: white; }
    .logo-mark {
      width: 32px; height: 32px; border-radius: $r-md;
      background: $c-pink; color: white;
      display: inline-flex; align-items: center; justify-content: center;
      font-size: 16px; font-weight: 900;
    }
  }

  .brand-title {
    font-size: var(--fs-h1);
    font-weight: 800;
    letter-spacing: -0.03em;
    line-height: 1.02;
    margin: 0 0 18px;
    .c-pink { color: $c-pink; }
  }
  .brand-sub {
    color: $dark-text-2;
    line-height: 1.6;
    font-size: 15px;
    margin: 0 0 32px;
    max-width: 400px;
  }
  .brand-features {
    list-style: none;
    padding: 0;
    margin: 0;
    li {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 0;
      font-size: 14px;
      color: $dark-text-3;
      &::before {
        content: '';
        width: 6px; height: 6px; border-radius: 50%;
        background: $c-pink;
        flex-shrink: 0;
      }
      &:nth-child(2)::before { background: $c-cyan; }
      &:nth-child(3)::before { background: $c-yellow; }
    }
  }
  .brand-deco {
    position: absolute;
    right: 32px;
    bottom: 32px;
    opacity: 0.85;
  }
}

.auth-form {
  background: $n-paper;
  padding: 72px 60px;
  display: flex;
  align-items: center;
  justify-content: center;

  &__inner { width: 100%; max-width: 400px; }
  &__title {
    font-size: var(--fs-h2);
    font-weight: 800;
    margin: 0 0 4px;
    letter-spacing: -0.02em;
    color: $n-ink;
  }
  &__sub {
    color: $gray-500;
    margin: 0 0 32px;
    font-size: $fs-sm;
    .link { color: $c-pink; font-weight: 700; cursor: pointer; &:hover { color: $c-pink-hover; } }
  }
  &__form { margin-bottom: 20px; }
  &__foot {
    font-size: $fs-xs;
    color: $gray-400;
    margin: 28px 0 0;
    text-align: center;
    letter-spacing: 0.04em;
  }

  // el-form-item label override
  :deep(.el-form-item__label) {
    font-size: $fs-xs;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: $gray-600;
    padding-bottom: 8px;
  }
}

@media (max-width: 900px) {
  .auth-page { grid-template-columns: 1fr; }
  .auth-brand { padding: 48px 32px; min-height: 360px; }
  .auth-form { padding: 48px 32px; }
}
</style>
```

- [ ] **Step 3: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 4: 人工验证**

```bash
pnpm dev
```

打开 `http://localhost:5173/login`，确认：
- 左栏暗紫底 + 粉色 logo + 大字「欢迎回来」
- 右栏白底 + 粉色主按钮
- 输入框 focus 是紫色 ring
- 无任何渐变

Ctrl+C 停止。

- [ ] **Step 5: 提交**

```bash
cd 本仓库
git add web/src/views/auth/Login.vue
git commit -m "refactor(web): Login 两栏暗亮分明，粉色主按钮，线稿钥匙装饰"
```

---

### Task 4.4: Register.vue 全量重写

**Files:**
- Modify: `web/src/views/auth/Register.vue`

- [ ] **Step 1: 先读 Register 当前文件了解它的 fields**

```bash
cat 本仓库/web/src/views/auth/Register.vue | head -80
```

- [ ] **Step 2: 保留原有 fields 和业务逻辑，替换模板和样式为与 Login 同构**

整体结构和 Login.vue 一致。差异：
- 标题改「加入 <span class="c-yellow">GPT2API</span>。」
- Brand features 换成「多账号池 · 零代码迁移」「IMG2 终稿直出 · 批量出图」「积分计费 · 用量透明」（或 Login 同款）
- 装饰 SVG 改一个「拥抱」/「信件」的线稿
- 表单字段按原文件保留（email, password, password_confirm, invite_code 等）

按原文件的 `<script setup>` 保留 **全部 ref / reactive / async 函数**，只重写 `<template>` 和 `<style>`。

给个 Register 专用的装饰 SVG（信件）放在 brand 里：

```vue
<svg class="brand-deco" width="180" height="120" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="30" y="40" width="120" height="80" rx="6" stroke="#FFD600" stroke-width="2"/>
  <path d="M30 50 L90 90 L150 50" stroke="#FF3D94" stroke-width="2"/>
  <path d="M155 30 L175 30 M165 20 L165 40" stroke="#00D9FF" stroke-width="2"/>
  <path d="M10 100 l3 0 M11.5 98.5 l0 3" stroke="#A855F7" stroke-width="1.8"/>
</svg>
```

标题片段：

```vue
<h1 class="brand-title">
  加入<br><span class="c-yellow">GPT2API</span>。
</h1>
```

样式里 `.c-yellow { color: $c-yellow; }`。

**关键：** 保留你从原文件读到的 `form`, `rules`, `onSubmit`, `formRef`, `loading`, `route`, `router`, `store`, `site`, `siteLogo`, `siteName` 等业务逻辑，只替换 template 结构和样式。`<el-form>` 内 `<el-form-item>` 字段名和校验与原一致。

按钮改用：
```vue
<NeonButton variant="pink" size="lg" :block="true" :disabled="loading" @click="onSubmit">
  {{ loading ? '提交中…' : '注册 →' }}
</NeonButton>
```

- [ ] **Step 3: 验证**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 4: 人工验证**

```bash
pnpm dev
```

`http://localhost:5173/register` 打开，验证字段完整、表单可正常提交、视觉与 Login 一致风格但色调黄/cyan 区分。

Ctrl+C 停止。

- [ ] **Step 5: 提交**

```bash
cd 本仓库
git add web/src/views/auth/Register.vue
git commit -m "refactor(web): Register 与 Login 同构，黄色主题标题，信件线稿"
```

---

## Phase 5 · Personal（6 页）

### Task 5.1: Dashboard.vue 重写

**Files:**
- Modify: `web/src/views/personal/Dashboard.vue`
- Create: `web/src/assets/illustrations/kpi-wallet.svg`
- Create: `web/src/assets/illustrations/kpi-chart.svg`
- Create: `web/src/assets/illustrations/kpi-check.svg`
- Create: `web/src/assets/illustrations/kpi-chain.svg`

- [ ] **Step 1: 创建 4 个 KPI 角标 SVG**

`web/src/assets/illustrations/kpi-wallet.svg`：
```xml
<svg xmlns="http://www.w3.org/2000/svg" width="50" height="40" viewBox="0 0 60 50" fill="none" stroke-linecap="round">
  <path d="M5 40 L5 15 L20 5 L35 15 L35 40 Z" stroke="#FF3D94" stroke-width="1.8"/>
  <circle cx="45" cy="20" r="8" stroke="#A855F7" stroke-width="1.6"/>
</svg>
```

`web/src/assets/illustrations/kpi-chart.svg`：
```xml
<svg xmlns="http://www.w3.org/2000/svg" width="50" height="40" viewBox="0 0 60 50" fill="none" stroke-linecap="round">
  <path d="M5 40 L15 25 L25 35 L35 15 L50 30" stroke="#00D9FF" stroke-width="2"/>
  <circle cx="35" cy="15" r="3" fill="#00D9FF"/>
</svg>
```

`web/src/assets/illustrations/kpi-check.svg`：
```xml
<svg xmlns="http://www.w3.org/2000/svg" width="50" height="40" viewBox="0 0 60 50" fill="none" stroke-linecap="round">
  <circle cx="30" cy="25" r="18" stroke="#FFD600" stroke-width="2"/>
  <path d="M20 25 L27 32 L42 17" stroke="#FFD600" stroke-width="2"/>
</svg>
```

`web/src/assets/illustrations/kpi-chain.svg`：
```xml
<svg xmlns="http://www.w3.org/2000/svg" width="50" height="40" viewBox="0 0 60 50" fill="none" stroke-linecap="round">
  <circle cx="15" cy="25" r="8" stroke="#A855F7" stroke-width="1.8"/>
  <path d="M23 25 L50 25" stroke="#A855F7" stroke-width="2"/>
  <path d="M45 25 L45 32 M40 25 L40 30" stroke="#A855F7" stroke-width="2"/>
</svg>
```

- [ ] **Step 2: 读原 Dashboard.vue 了解 script 逻辑和模板结构**

```bash
cat 本仓库/web/src/views/personal/Dashboard.vue
```

- [ ] **Step 3: 保留 `<script setup>` 完整逻辑，替换 `<template>` 和 `<style>`**

关键改动：
- 顶部用 `<PageHeader crumb="个人中心" title="总览" accent-word="览" accent="pink">` 替代原标题
- 4 张原有统计卡改成 `<KpiCard>` 组件：
  - 余额 → accent="pink" + `<KpiWallet />` 插图
  - 今日调用 → accent="cyan" + `<KpiChart />`
  - 成功率 → accent="yellow" + `<KpiCheck />`
  - 活跃 Key → accent="purple" + `<KpiChain />`
- 原 SVG 折线图保留数据绑定逻辑（`chartWrap`, `chartW`, `padT`, `padB` 等），但 path 的 `stroke` 颜色改为 `$c-pink`；填充由 `url(#g)` 改为直接 `fill="rgba(255,61,148,0.08)"`
- 两栏 Panel 包装改为 `.panel { background: $n-paper; border: $bw solid $gray-200; border-radius: $r-lg; padding: 22px 24px; }`
- 问候语大字改为 `<h2 class="greet">下午好，<span class="accent">{{ user.nickname }}</span> 👋</h2>` — accent 是 `$c-pink`，不是渐变
- 任何原来的 `linear-gradient` 全删

插图 import 和 `<KpiCard>` 使用示例：

```vue
<script setup lang="ts">
// ... 保留原有所有 import 和逻辑
import KpiWallet from '@/assets/illustrations/kpi-wallet.svg?component'
import KpiChart from '@/assets/illustrations/kpi-chart.svg?component'
import KpiCheck from '@/assets/illustrations/kpi-check.svg?component'
import KpiChain from '@/assets/illustrations/kpi-chain.svg?component'
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="个人中心" title="总览" accent-word="览" accent="pink">
      <template #extra>
        <NeonButton variant="outline" @click="loadAll">
          <el-icon><Refresh /></el-icon> 刷新
        </NeonButton>
      </template>
    </PageHeader>

    <h2 class="greet">下午好，<span class="greet-accent">{{ user?.nickname || user?.email }}</span> 👋</h2>
    <p class="greet-sub">你当前余额 <b>{{ balance }}</b>，近 14 天共调用 API <b>{{ monthOverall?.requests ?? 0 }}</b> 次。</p>

    <div class="kpi-grid">
      <KpiCard label="余额" :value="balance" :change="`冻结 ${frozen}`" accent="pink">
        <template #illustration><KpiWallet /></template>
      </KpiCard>
      <KpiCard label="今日调用" :value="todayOverall?.requests ?? 0" :change="`${successRate(todayOverall)} 成功`" change-dir="up" accent="cyan">
        <template #illustration><KpiChart /></template>
      </KpiCard>
      <KpiCard label="成功率" :value="successRate(monthOverall)" :change="`${monthOverall?.failures ?? 0} 次失败`" accent="yellow">
        <template #illustration><KpiCheck /></template>
      </KpiCard>
      <KpiCard label="活跃 Key" :value="`${keyActive} / ${keyTotal}`" :change="`${keyTotal - keyActive} 个已禁用`" accent="purple">
        <template #illustration><KpiChain /></template>
      </KpiCard>
    </div>

    <div class="two-col">
      <div class="panel">
        <h4>近 14 天调用趋势</h4>
        <div class="chart-wrap" ref="chartWrap">
          <!-- 保留原 SVG 生成逻辑，但改用纯色 fill -->
          <svg class="chart" :width="chartW" :height="chartH">
            <path :d="areaPath" fill="rgba(255,61,148,0.08)" />
            <path :d="linePath" stroke="#FF3D94" stroke-width="2.5" fill="none" />
            <!-- 数据点保持原逻辑 -->
          </svg>
        </div>
      </div>
      <div class="panel">
        <h4>我的 API Keys</h4>
        <!-- 原 keys 列表结构保留，改用 StatusTag -->
      </div>
    </div>

    <!-- 最近日志 / 账变 panel 继续保留原结构 -->
  </div>
</template>
```

完整样式（关键片段）：

```scss
@use '@/styles/tokens' as *;

.greet {
  font-size: var(--fs-h2);
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0 0 6px;
  color: $n-ink;
  .greet-accent { color: $c-pink; }
}
.greet-sub { color: $gray-500; margin: 0 0 24px; font-size: 15px; b { color: $n-ink; } }

.kpi-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 28px; }

.two-col { display: grid; grid-template-columns: 2fr 1fr; gap: 16px; }

.panel {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 22px 24px;
  h4 { margin: 0 0 14px; font-size: 16px; font-weight: 800; letter-spacing: -0.01em; color: $n-ink; }
}

// 删掉所有 linear-gradient 引用

@media (max-width: 1100px) {
  .kpi-grid { grid-template-columns: repeat(2, 1fr); }
  .two-col { grid-template-columns: 1fr; }
}
```

- [ ] **Step 4: 构建校验**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
pnpm build
```

Expected: 都成功。

- [ ] **Step 5: 人工验证**

```bash
pnpm dev
```

登录后访问 `http://localhost:5173/personal/dashboard`，确认：
- 顶部标题「总**览**」（览 = pink）
- 4 张 KPI 卡，左边 5px 纯色条按 pink/cyan/yellow/purple 排列
- 每张卡右下角有 SVG 线稿
- 折线图纯色粉 + 8% 粉面积填充
- 无任何渐变

- [ ] **Step 6: 提交**

```bash
cd 本仓库
git add web/src/assets/illustrations/kpi-*.svg web/src/views/personal/Dashboard.vue
git commit -m "refactor(web): Dashboard 用 KpiCard + PageHeader，4 色 KPI 分配，纯色折线图"
```

---

### Task 5.2: ApiKeys.vue 重写

**Files:**
- Modify: `web/src/views/personal/ApiKeys.vue`
- Create: `web/src/assets/illustrations/empty-keys.svg`

- [ ] **Step 1: 创建空态插图**

`web/src/assets/illustrations/empty-keys.svg`：
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <circle cx="55" cy="70" r="26" stroke="#00D9FF" stroke-width="2"/>
  <circle cx="55" cy="70" r="10" stroke="#00D9FF" stroke-width="2"/>
  <path d="M81 70 L160 70" stroke="#FF3D94" stroke-width="2"/>
  <path d="M140 70 L140 88 M120 70 L120 82" stroke="#FF3D94" stroke-width="2"/>
  <path d="M10 40 l4 0 M12 38 l0 4" stroke="#FFD600" stroke-width="1.8"/>
  <path d="M165 100 l4 0 M167 98 l0 4" stroke="#A855F7" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 2: 读原文件**

```bash
cat 本仓库/web/src/views/personal/ApiKeys.vue
```

- [ ] **Step 3: 替换 template 和 style（script 保持不变）**

顶部：
```vue
<PageHeader crumb="个人中心" title="API Keys" accent-word="Keys" accent="cyan">
  <template #extra>
    <NeonButton variant="pink" @click="openCreateDlg">+ 生成新 Key</NeonButton>
  </template>
</PageHeader>
```

搜索条和 Key 表格：用 `class="filter-bar"` 容器 + `<el-input>` + `<el-table>`（交给 element override 统一样式）。

Key 状态 tag 用 `<StatusTag>`：
```vue
<StatusTag :variant="row.enabled ? 'active' : 'disabled'" dot>
  {{ row.enabled ? '启用' : '禁用' }}
</StatusTag>
```

空态（表为空时）：
```vue
<EmptyState
  v-if="!loading && rows.length === 0"
  title="还没有 API Key"
  desc="生成第一个 Key 开始调用 OpenAI 兼容接口。"
>
  <template #illustration><EmptyKeys /></template>
  <template #action>
    <NeonButton variant="pink" @click="openCreateDlg">生成 Key →</NeonButton>
  </template>
</EmptyState>
```

对应 import `import EmptyKeys from '@/assets/illustrations/empty-keys.svg?component'`。

样式 filter-bar：
```scss
.filter-bar {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 18px 22px;
  margin-bottom: 16px;
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}
```

- [ ] **Step 4: 验证**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
```

访问 `/personal/keys` 确认表格、StatusTag、空态、生成 Dialog 都工作，粉色主按钮。

- [ ] **Step 5: 提交**

```bash
git add web/src/assets/illustrations/empty-keys.svg web/src/views/personal/ApiKeys.vue
git commit -m "refactor(web): ApiKeys 表格 + 空态 + cyan accent"
```

---

### Task 5.3: Usage.vue 重写

**Files:**
- Modify: `web/src/views/personal/Usage.vue`

- [ ] **Step 1: 读原文件**

```bash
cat 本仓库/web/src/views/personal/Usage.vue
```

- [ ] **Step 2: 替换 template 和 style（保留 script 业务逻辑）**

顶部：
```vue
<PageHeader crumb="个人中心" title="使用记录" accent-word="记录" accent="purple">
  <template #extra>
    <el-date-picker v-model="range" type="daterange" />
    <NeonButton variant="outline" @click="refresh">
      <el-icon><Refresh /></el-icon> 刷新
    </NeonButton>
  </template>
</PageHeader>
```

3 张 KpiCard（总调用 · 总消耗 · 失败率），accent 分别 purple / pink / orange。

折线/柱状图：保留原 SVG 渲染逻辑，所有 `stroke` 和 `fill` 颜色从渐变改成 `$c-purple` 单色 + 透明度。若原代码用 `<linearGradient>` 定义，全部替换成 `fill="rgba(168,85,247,0.08)"` + `stroke="#A855F7"`。

明细表：用 `<el-table>`（element override 生效）；模型字段用 `<StatusTag variant="cyan">`；错误码字段用 `<StatusTag variant="danger">`。

- [ ] **Step 3: 验证**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
```

`/personal/usage` 过 golden path：筛时间、看 KPI、看图表、翻表格。

- [ ] **Step 4: 提交**

```bash
git add web/src/views/personal/Usage.vue
git commit -m "refactor(web): Usage 切紫色 accent，KPI + 纯色图表 + StatusTag 明细"
```

---

### Task 5.4: Billing.vue 重写

**Files:**
- Modify: `web/src/views/personal/Billing.vue`
- Create: `web/src/assets/illustrations/empty-recharges.svg`

- [ ] **Step 1: 创建空态插图**

`web/src/assets/illustrations/empty-recharges.svg`：
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="30" y="40" width="120" height="70" rx="8" stroke="#00E676" stroke-width="2"/>
  <path d="M30 62 L150 62" stroke="#00E676" stroke-width="2"/>
  <path d="M42 84 L60 84 M42 94 L72 94" stroke="#A855F7" stroke-width="1.8"/>
  <circle cx="130" cy="90" r="10" stroke="#FF3D94" stroke-width="1.8"/>
  <path d="M126 90 L130 94 L134 86" stroke="#FF3D94" stroke-width="1.8"/>
  <path d="M10 100 l4 0 M12 98 l0 4" stroke="#FFD600" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 2: 读原文件**

```bash
cat 本仓库/web/src/views/personal/Billing.vue
```

- [ ] **Step 3: 替换 template / style，保留 script**

顶部：
```vue
<PageHeader crumb="个人中心" title="账单与充值" accent-word="充值" accent="green">
  <template #extra>
    <NeonButton variant="green" @click="openRechargeDlg">+ 充值</NeonButton>
  </template>
</PageHeader>
```

余额卡：
```vue
<div class="balance-card">
  <div class="balance-card__label">当前余额</div>
  <div class="balance-card__value">{{ balance }}</div>
  <div class="balance-card__sub">冻结 {{ frozen }} · 已消耗 {{ used }}</div>
</div>
```

样式：
```scss
.balance-card {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-xl;
  padding: 32px 40px;
  margin-bottom: 24px;
  position: relative;
  overflow: hidden;

  &::before {
    content: '';
    position: absolute; left: 0; top: 0; bottom: 0; width: 6px;
    background: $c-green;
  }

  &__label { font-size: $fs-xs; letter-spacing: 0.14em; text-transform: uppercase; color: $gray-500; font-weight: 700; }
  &__value { font-size: 56px; font-weight: 800; letter-spacing: -0.03em; color: $n-ink; margin: 8px 0 4px; line-height: 1; }
  &__sub { font-size: $fs-sm; color: $gray-500; }
}
```

账变流水 `<el-table>` 用 element override；状态列 StatusTag 四档（pending=warning / paid=success / refunded=info / failed=danger）。

空态同上用 `<EmptyState>` + `<EmptyRecharges />`。

- [ ] **Step 4: 验证**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
```

- [ ] **Step 5: 提交**

```bash
git add web/src/assets/illustrations/empty-recharges.svg web/src/views/personal/Billing.vue
git commit -m "refactor(web): Billing 大余额卡 + 绿色 accent + 充值流水"
```

---

### Task 5.5: OnlinePlay.vue 重写

**Files:**
- Modify: `web/src/views/personal/OnlinePlay.vue`

- [ ] **Step 1: 读原文件**

```bash
cat 本仓库/web/src/views/personal/OnlinePlay.vue
```

（此页代码较长，约 500 行；包含 Chat/Text2Img/Img2Img 三个 tab，以及 prompt 表单和结果展示。）

- [ ] **Step 2: 策略：只改外层容器 + tab + 按钮 + 结果卡样式**

外层容器：
```vue
<div class="page-container">
  <PageHeader crumb="个人中心" title="在线体验" accent-word="体验" accent="yellow">
    <template #extra>
      <span class="credit-chip">余额 <b>{{ balance }}</b></span>
    </template>
  </PageHeader>

  <el-tabs v-model="activeTab" class="play-tabs">
    <!-- 保留原 el-tab-pane 结构 -->
  </el-tabs>
</div>
```

Tab 样式覆盖：
```scss
:deep(.play-tabs .el-tabs__item) {
  font-weight: 700;
  font-size: 15px;
  &.is-active { color: $c-yellow-hover; }  // 亮底压深的黄
}
:deep(.play-tabs .el-tabs__active-bar) { background: $c-yellow-hover; height: 3px; }
```

Prompt 表单区块（左）：维持原结构，但 `<el-button type="primary" @click="sendImage">` 改成 `<NeonButton variant="pink" @click="sendImage">生成 →</NeonButton>`。

结果展示区块（右）：
```scss
.result-panel {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 20px 22px;
  min-height: 400px;
}
.result-img-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
  img {
    width: 100%;
    border-radius: $r-md;
    border: $bw solid $gray-200;
    transition: transform 0.2s;
    &:hover { transform: scale(1.02); }
  }
}
```

Chat 消息气泡：保留原结构，`.bubble-user` 底色改 `rgba(255, 214, 0, 0.12)`；`.bubble-assistant` 底色改 `$gray-100`。原渐变背景（如有）全删。

.credit-chip:
```scss
.credit-chip {
  font-size: $fs-sm;
  padding: 6px 14px;
  background: $gray-100;
  border-radius: $r-pill;
  color: $gray-600;
  b { color: $n-ink; font-weight: 800; }
}
```

- [ ] **Step 3: grep 确认无残留 linear-gradient**

```bash
grep -n "linear-gradient\|radial-gradient" 本仓库/web/src/views/personal/OnlinePlay.vue
```

Expected: 0 matches.

- [ ] **Step 4: 验证 + dev**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
```

访问 `/personal/play`，跑一条 text2img prompt 确认出图、查看网格、切 tab。

- [ ] **Step 5: 提交**

```bash
git add web/src/views/personal/OnlinePlay.vue
git commit -m "refactor(web): OnlinePlay 切 Neon Solid — 黄色 tab accent + 结果网格"
```

---

### Task 5.6: ApiDocs.vue 重写

**Files:**
- Modify: `web/src/views/personal/ApiDocs.vue`

- [ ] **Step 1: 读原文件**

```bash
cat 本仓库/web/src/views/personal/ApiDocs.vue
```

- [ ] **Step 2: 重写 template + style**

顶部：
```vue
<PageHeader crumb="个人中心" title="接口文档" accent-word="文档" accent="pink" />
```

两栏：左目录，右内容。代码块样式：

```scss
.code-block {
  background: $n-ink;
  color: $dark-text-1;
  padding: 20px 24px;
  border-radius: $r-lg;
  font-family: $f-mono;
  font-size: 13px;
  line-height: 1.7;
  overflow-x: auto;
  border: $bw solid $dark-border-strong;

  .kw  { color: $c-pink; }    // 关键字
  .str { color: $c-green; }    // 字符串
  .com { color: $dark-text-4; font-style: italic; } // 注释
  .fn  { color: $c-cyan; }     // 函数名
  .num { color: $c-yellow; }   // 数字
}

.toc {
  position: sticky;
  top: 84px;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 16px 18px;
  font-size: $fs-sm;

  li { padding: 6px 8px; border-radius: $r-sm; cursor: pointer;
    &:hover { background: $gray-100; }
    &.active { background: rgba(255, 61, 148, 0.08); color: $c-pink; font-weight: 700; border-left: 3px solid $c-pink; padding-left: 5px; }
  }
}
```

- [ ] **Step 3: 验证**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
```

`/personal/docs` 确认代码块样式、目录高亮。

- [ ] **Step 4: 提交**

```bash
git add web/src/views/personal/ApiDocs.vue
git commit -m "refactor(web): ApiDocs 代码块暗色 + 四色语法高亮"
```

---

## Phase 6 · Admin（12 页）

**策略：** Users.vue 作完整示范，其余 11 页按相同模式批量改造，每页仅记录差异。

### Task 6.1: Users.vue 作为 Admin exemplar

**Files:**
- Modify: `web/src/views/admin/Users.vue`
- Create: `web/src/assets/illustrations/empty-users.svg`

- [ ] **Step 1: 创建空态插图**

`web/src/assets/illustrations/empty-users.svg`：
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <circle cx="55" cy="55" r="18" stroke="#FF3D94" stroke-width="2"/>
  <path d="M30 110 Q55 85 80 110" stroke="#FF3D94" stroke-width="2"/>
  <circle cx="115" cy="65" r="14" stroke="#00D9FF" stroke-width="2"/>
  <path d="M95 110 Q115 90 140 110" stroke="#00D9FF" stroke-width="2"/>
  <path d="M155 30 l4 0 M157 28 l0 4" stroke="#FFD600" stroke-width="1.8"/>
  <path d="M12 95 l3 0 M13.5 93.5 l0 3" stroke="#A855F7" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 2: 读原 Users.vue**

```bash
wc -l 本仓库/web/src/views/admin/Users.vue
cat 本仓库/web/src/views/admin/Users.vue
```

- [ ] **Step 3: 保留完整 `<script setup>`，替换 template + style**

Template 骨架：
```vue
<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 用户" title="用户管理" accent-word="管理" accent="pink">
      <template #extra>
        <NeonButton variant="pink" @click="createDlg = true">+ 新增用户</NeonButton>
      </template>
    </PageHeader>

    <!-- 筛选条 -->
    <div class="filter-bar">
      <el-input v-model="filter.q" placeholder="搜索邮箱 / 昵称 / ID" style="width:220px" clearable />
      <el-select v-model="filter.role" placeholder="角色" style="width:140px" clearable>
        <el-option label="全部" value="" />
        <el-option label="管理员" value="admin" />
        <el-option label="普通用户" value="user" />
      </el-select>
      <el-select v-model="filter.status" placeholder="状态" style="width:140px" clearable>
        <el-option label="启用" value="active" />
        <el-option label="禁用" value="disabled" />
      </el-select>
      <el-select v-model="filter.group_id" placeholder="分组" style="width:160px" clearable>
        <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
      </el-select>
      <NeonButton variant="pink" size="sm" @click="fetchList">搜索</NeonButton>
      <NeonButton variant="ghost" size="sm" @click="resetFilter">重置</NeonButton>
    </div>

    <!-- 表格 -->
    <el-table :data="rows" v-loading="loading" class="user-table">
      <el-table-column label="用户" min-width="240">
        <template #default="{ row }">
          <div class="u-cell">
            <UserAvatar :name="row.nickname || row.email" size="md" />
            <div class="u-meta">
              <div class="u-email">{{ row.email }}</div>
              <div class="u-sub">ID #{{ row.id }}<template v-if="row.nickname"> · {{ row.nickname }}</template></div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="角色" width="110">
        <template #default="{ row }">
          <StatusTag :variant="row.role === 'admin' ? 'admin' : (row.role === 'pro' ? 'pro' : 'free')">
            {{ row.role }}
          </StatusTag>
        </template>
      </el-table-column>
      <el-table-column label="余额" width="130">
        <template #default="{ row }">{{ formatCredit(row.credit_balance) }}</template>
      </el-table-column>
      <el-table-column label="分组" width="120">
        <template #default="{ row }">{{ groupName(row.group_id) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <StatusTag :variant="row.status === 'active' ? 'active' : 'disabled'" dot>
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </StatusTag>
        </template>
      </el-table-column>
      <el-table-column label="最后登录" width="160">
        <template #default="{ row }">
          <span class="u-sub">{{ row.last_login_at ? formatDateTime(row.last_login_at) : '—' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" align="right" width="200">
        <template #default="{ row }">
          <div class="op">
            <a class="op-link" @click="openEdit(row)">编辑</a>
            <a class="op-link" @click="openAdjust(row)">调账</a>
            <a class="op-link danger" @click="openReset(row)">重置</a>
            <a class="op-link danger" @click="onDelete(row)">删除</a>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- 空态（total === 0 时） -->
    <EmptyState
      v-if="!loading && total === 0"
      title="没有匹配的用户"
      desc="换个筛选条件，或点击新增创建第一个用户。"
    >
      <template #illustration><EmptyUsers /></template>
    </EmptyState>

    <!-- 分页 -->
    <div class="pagination-bar">
      <span class="pg-info">共 <b>{{ total }}</b> 位用户</span>
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="filter.limit"
        :total="total"
        layout="prev, pager, next"
        @current-change="onPageChange"
      />
    </div>

    <!-- Dialogs: editDlg / pwdDlg / adjustDlg / logsDlg 保留原结构 -->
    <el-dialog v-model="editDlg" title="编辑用户" width="420">
      <!-- 原有 el-form 内容保留 -->
    </el-dialog>
    <!-- ... -->
  </div>
</template>
```

Script 顶部需要追加一个 import：
```ts
import EmptyUsers from '@/assets/illustrations/empty-users.svg?component'
```

Script 的其他所有业务逻辑（filter / fetchList / openEdit / onSaveEdit / openReset / onResetSubmit / openAdjust / onAdjustSubmit / onDelete / 流水 logsDlg 等）保持原样。

如果原来没有 currentPage 计算属性，照这样加（Element 分页给的是 current-change 回调）：
```ts
const currentPage = computed({
  get: () => Math.floor(filter.offset / filter.limit) + 1,
  set: () => {}, // 通过 onPageChange 改
})
function onPageChange(p: number) {
  filter.offset = (p - 1) * filter.limit
  fetchList()
}
```

Style：
```scss
<style scoped lang="scss">
@use '@/styles/tokens' as *;

.filter-bar {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 18px 22px;
  margin-bottom: 16px;
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.user-table { /* element override 已处理基本样式 */ }

.u-cell { display: flex; align-items: center; gap: 12px; }
.u-meta { display: flex; flex-direction: column; gap: 2px; }
.u-email { font-weight: 700; color: $n-ink; font-size: 14px; }
.u-sub { font-size: 12px; color: $gray-500; }

.op { display: flex; gap: 4px; justify-content: flex-end; }
.op-link {
  color: $c-pink; font-size: 13px; font-weight: 700;
  padding: 4px 10px; border-radius: $r-sm; cursor: pointer;
  &:hover { background: rgba(255, 61, 148, 0.08); }
  &.danger { color: $c-orange; &:hover { background: rgba(255, 107, 53, 0.08); } }
}

.pagination-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
  font-size: $fs-sm;
  color: $gray-500;
  .pg-info b { color: $n-ink; }
}
</style>
```

- [ ] **Step 4: grep 残留**

```bash
grep -n "linear-gradient\|html\.dark\|ui\.isDark" 本仓库/web/src/views/admin/Users.vue
```

Expected: 0 matches.

- [ ] **Step 5: 构建 + dev 验证**

```bash
pnpm exec vue-tsc --noEmit
pnpm build
pnpm dev
```

以管理员身份访问 `/admin/users`，过 golden path：筛选 → 编辑 → 调账 → 重置 → 删除（可选）→ 翻页 → 空态。

- [ ] **Step 6: 提交**

```bash
git add web/src/assets/illustrations/empty-users.svg web/src/views/admin/Users.vue
git commit -m "refactor(web): Admin Users 作 exemplar — PageHeader + UserAvatar + StatusTag + filter-bar"
```

---

### Task 6.2: Credits + Recharges

**Files:**
- Modify: `web/src/views/admin/Credits.vue`
- Modify: `web/src/views/admin/Recharges.vue`

按 Users.vue 的 exemplar 模式应用到这两页，差异如下：

- [ ] **Step 1: Credits.vue 改造**

```bash
cat 本仓库/web/src/views/admin/Credits.vue | head -60
```

顶部：
```vue
<PageHeader crumb="管理员 / 积分" title="积分管理" accent-word="管理" accent="green">
  <template #extra>
    <NeonButton variant="green" @click="batchAdjust">批量调账</NeonButton>
  </template>
</PageHeader>
```

流水类型列用 `<StatusTag>`：
- `recharge` → `<StatusTag variant="success" dot>充值</StatusTag>`
- `deduct` → `<StatusTag variant="warning">扣款</StatusTag>`
- `adjust` → `<StatusTag variant="info">调账</StatusTag>`
- `refund` → `<StatusTag variant="purple">退款</StatusTag>`

金额正数用 `$c-green`，负数用 `$c-orange`：
```vue
<span :class="row.delta >= 0 ? 'delta-up' : 'delta-down'">
  {{ row.delta >= 0 ? '+' : '' }}{{ formatCredit(row.delta) }}
</span>
```
```scss
.delta-up { color: $c-green; font-weight: 700; font-family: $f-mono; }
.delta-down { color: $c-orange; font-weight: 700; font-family: $f-mono; }
```

- [ ] **Step 2: Recharges.vue 改造**

```bash
cat 本仓库/web/src/views/admin/Recharges.vue | head -60
```

顶部：
```vue
<PageHeader crumb="管理员 / 充值" title="充值订单" accent-word="订单" accent="yellow" />
```

状态列 4 档 StatusTag：
- `pending` → `<StatusTag variant="warning" dot>待支付</StatusTag>`
- `paid`    → `<StatusTag variant="success" dot>已支付</StatusTag>`
- `refunded`→ `<StatusTag variant="info">已退款</StatusTag>`
- `failed`  → `<StatusTag variant="danger" dot>失败</StatusTag>`

- [ ] **Step 3: 验证 + 提交**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
# 验证 /admin/credits 和 /admin/recharges
git add web/src/views/admin/Credits.vue web/src/views/admin/Recharges.vue
git commit -m "refactor(web): Admin Credits (green) + Recharges (yellow) 切 Neon Solid"
```

---

### Task 6.3: Accounts + Proxies

**Files:**
- Modify: `web/src/views/admin/Accounts.vue`
- Modify: `web/src/views/admin/Proxies.vue`

- [ ] **Step 1: Accounts.vue**

```bash
cat 本仓库/web/src/views/admin/Accounts.vue | head -60
```

顶部：
```vue
<PageHeader crumb="管理员 / 账号" title="GPT 账号" accent-word="账号" accent="cyan">
  <template #extra>
    <NeonButton variant="pink" @click="openImport">+ 导入账号</NeonButton>
  </template>
</PageHeader>
```

健康度列：
```vue
<template #default="{ row }">
  <StatusTag :variant="healthVariant(row.health)" dot>
    {{ healthLabel(row.health) }}
  </StatusTag>
</template>
```

script 顶部加：
```ts
function healthVariant(h: string): 'active' | 'warning' | 'danger' | 'disabled' {
  if (h === 'ok') return 'active'
  if (h === 'warn') return 'warning'
  if (h === 'error') return 'danger'
  return 'disabled'
}
function healthLabel(h: string): string {
  return ({ ok: '健康', warn: '告警', error: '异常', unknown: '未知' } as Record<string, string>)[h] || h
}
```

若原字段不是 health 而是 status，按实际字段名改，但模式相同。

- [ ] **Step 2: Proxies.vue** 同上，accent 改 purple，`<StatusTag>` 映射代理 up/down。

- [ ] **Step 3: 验证 + 提交**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
git add web/src/views/admin/Accounts.vue web/src/views/admin/Proxies.vue
git commit -m "refactor(web): Admin Accounts (cyan) + Proxies (purple) + 健康度 StatusTag"
```

---

### Task 6.4: Models + Groups

**Files:**
- Modify: `web/src/views/admin/Models.vue`
- Modify: `web/src/views/admin/Groups.vue`

- [ ] **Step 1: Models.vue**

```bash
cat 本仓库/web/src/views/admin/Models.vue | head -60
```

顶部：
```vue
<PageHeader crumb="管理员 / 模型" title="模型配置" accent-word="配置" accent="pink">
  <template #extra>
    <NeonButton variant="pink" @click="openCreate">+ 新增模型</NeonButton>
  </template>
</PageHeader>
```

启用开关列：保留 `<el-switch>`（element override 已处理；主色为 pink）。

类型列（chat / image）：
```vue
<StatusTag :variant="row.type === 'chat' ? 'cyan' : 'yellow'">{{ row.type }}</StatusTag>
```

- [ ] **Step 2: Groups.vue**

```bash
cat 本仓库/web/src/views/admin/Groups.vue | head -60
```

顶部：
```vue
<PageHeader crumb="管理员 / 分组" title="用户分组" accent-word="分组" accent="cyan">
  <template #extra>
    <NeonButton variant="cyan" @click="openCreate">+ 新增分组</NeonButton>
  </template>
</PageHeader>
```

权限矩阵：保留 `<el-checkbox>` 组，按分类布局，加 section 标题。

- [ ] **Step 3: 验证 + 提交**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
git add web/src/views/admin/Models.vue web/src/views/admin/Groups.vue
git commit -m "refactor(web): Admin Models (pink) + Groups (cyan) 切 Neon Solid"
```

---

### Task 6.5: UsageStats + AdminKeys

**Files:**
- Modify: `web/src/views/admin/UsageStats.vue`
- Modify: `web/src/views/admin/AdminKeys.vue`

- [ ] **Step 1: UsageStats.vue — 大图表区**

```bash
cat 本仓库/web/src/views/admin/UsageStats.vue | head -60
```

顶部：
```vue
<PageHeader crumb="管理员 / 统计" title="用量统计" accent-word="统计" accent="purple">
  <template #extra>
    <el-date-picker v-model="range" type="daterange" />
  </template>
</PageHeader>
```

图表（多折线按模型）：每条线用不同纯色，轮换 `[$c-pink, $c-cyan, $c-yellow, $c-purple, $c-orange, $c-green]`。SVG 渲染里用数组索引映射颜色；**任何 `<linearGradient>` 定义整块删除，fill 用 `rgba(...)` 同色 8% 替代**。

饼图用同样的 6 色轮换，片与片之间留 2px `$n-paper` 分隔线。

- [ ] **Step 2: AdminKeys.vue — 全局 Keys 明细**

```bash
cat 本仓库/web/src/views/admin/AdminKeys.vue | head -60
```

顶部：
```vue
<PageHeader crumb="管理员 / Keys" title="全局 Keys" accent-word="Keys" accent="yellow" />
```

主要是大表格，按 Users exemplar 模式。所属用户列用 `<UserAvatar>` + email。

- [ ] **Step 3: 验证 + 提交**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
git add web/src/views/admin/UsageStats.vue web/src/views/admin/AdminKeys.vue
git commit -m "refactor(web): Admin UsageStats (purple 6 色图表) + AdminKeys (yellow)"
```

---

### Task 6.6: Audit + Backup + Settings

**Files:**
- Modify: `web/src/views/admin/Audit.vue`
- Modify: `web/src/views/admin/Backup.vue`
- Modify: `web/src/views/admin/Settings.vue`

- [ ] **Step 1: Audit.vue — 严肃日志**

```bash
cat 本仓库/web/src/views/admin/Audit.vue | head -60
```

```vue
<PageHeader crumb="管理员 / 审计" title="审计日志" accent-word="日志" accent="orange" />
```

操作级别列 StatusTag：info / warn / danger 三档。

- [ ] **Step 2: Backup.vue**

```bash
cat 本仓库/web/src/views/admin/Backup.vue | head -60
```

```vue
<PageHeader crumb="管理员 / 备份" title="数据备份" accent-word="备份" accent="green" />
```

两张操作卡（备份 / 恢复）：
```scss
.op-card {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 24px 28px;
  &--primary { border-left: 5px solid $c-green; }
  &--danger  { border-left: 5px solid $c-orange; }
  h4 { font-size: 18px; font-weight: 800; margin: 0 0 8px; letter-spacing: -0.01em; }
  p { color: $gray-500; margin: 0 0 16px; font-size: 14px; line-height: 1.6; }
}
```

历史备份列表用 `<el-table>`。

- [ ] **Step 3: Settings.vue**

```bash
cat 本仓库/web/src/views/admin/Settings.vue | head -60
```

```vue
<PageHeader crumb="管理员 / 设置" title="系统设置" accent-word="设置" accent="pink" />
```

`<el-tabs>` 样式同 OnlinePlay：active 粉色 + 3px bottom bar。表单内 `<el-form>`。

- [ ] **Step 4: 验证 + 提交**

```bash
pnpm exec vue-tsc --noEmit
pnpm build
pnpm dev
# 分别访问 /admin/audit /admin/backup /admin/settings 过一遍
git add web/src/views/admin/Audit.vue web/src/views/admin/Backup.vue web/src/views/admin/Settings.vue
git commit -m "refactor(web): Admin Audit (orange) + Backup (green ops-card) + Settings (pink tabs)"
```

---

## Phase 7 · Errors + 剩余插图

### Task 7.1: Error403 + Error404 重写

**Files:**
- Modify: `web/src/views/Error403.vue`
- Modify: `web/src/views/Error404.vue`
- Create: `web/src/assets/illustrations/error-404.svg`
- Create: `web/src/assets/illustrations/error-403.svg`

- [ ] **Step 1: error-404.svg（迷路指南针）**

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 180" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <circle cx="100" cy="90" r="60" stroke="#FF3D94" stroke-width="2.4"/>
  <circle cx="100" cy="90" r="8" stroke="#00D9FF" stroke-width="2"/>
  <path d="M100 30 L108 80 L100 90 L92 80 Z" stroke="#FF3D94" stroke-width="2" fill="#FF3D94" fill-opacity="0.15"/>
  <path d="M100 150 L108 100 L100 90 L92 100 Z" stroke="#A855F7" stroke-width="2"/>
  <path d="M40 90 L50 90 M150 90 L160 90 M100 30 L100 40 M100 140 L100 150" stroke="#00D9FF" stroke-width="2"/>
  <path d="M20 40 l3 0 M21.5 38.5 l0 3" stroke="#FFD600" stroke-width="2"/>
  <path d="M175 150 l3 0 M176.5 148.5 l0 3" stroke="#FFD600" stroke-width="2"/>
</svg>
```

- [ ] **Step 2: error-403.svg（锁）**

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 180" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="55" y="80" width="90" height="70" rx="8" stroke="#FF3D94" stroke-width="2.4"/>
  <path d="M70 80 V55 Q70 32 100 32 Q130 32 130 55 V80" stroke="#FF3D94" stroke-width="2.4"/>
  <circle cx="100" cy="115" r="8" stroke="#00D9FF" stroke-width="2"/>
  <path d="M100 123 L100 135" stroke="#00D9FF" stroke-width="2"/>
  <path d="M20 50 l3 0 M21.5 48.5 l0 3" stroke="#FFD600" stroke-width="2"/>
  <path d="M175 140 l3 0 M176.5 138.5 l0 3" stroke="#A855F7" stroke-width="2"/>
</svg>
```

- [ ] **Step 3: Error404.vue 全量替换**

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
import Err404Ill from '@/assets/illustrations/error-404.svg?component'
const router = useRouter()
</script>

<template>
  <div class="err-page dark-area">
    <div class="err-ill"><Err404Ill /></div>
    <div class="err-code">404</div>
    <h1 class="err-title">找不到页面</h1>
    <p class="err-desc">你要去的地方可能被搬家了，或者链接已失效。</p>
    <NeonButton variant="pink" size="lg" @click="router.replace('/')">回到首页 →</NeonButton>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.err-page {
  min-height: 100vh;
  background: $n-space;
  color: $dark-text-1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  text-align: center;
}
.err-ill { margin-bottom: 28px; :deep(svg) { width: 180px; height: 180px; } }
.err-code {
  font-size: 120px;
  font-weight: 900;
  color: $c-pink;
  letter-spacing: -0.05em;
  line-height: 1;
  margin-bottom: 14px;
}
.err-title {
  font-size: var(--fs-h1);
  font-weight: 800;
  letter-spacing: -0.03em;
  margin: 0 0 12px;
}
.err-desc {
  color: $dark-text-3;
  font-size: 17px;
  margin: 0 0 32px;
  max-width: 400px;
}
</style>
```

- [ ] **Step 4: Error403.vue 用同模板，数字改 403，图换 Err403Ill，文案「没有权限」**

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
import Err403Ill from '@/assets/illustrations/error-403.svg?component'
const router = useRouter()
</script>

<template>
  <div class="err-page dark-area">
    <div class="err-ill"><Err403Ill /></div>
    <div class="err-code">403</div>
    <h1 class="err-title">没有权限</h1>
    <p class="err-desc">你暂时无法访问此页面。如有必要请联系管理员分配权限。</p>
    <NeonButton variant="pink" size="lg" @click="router.replace('/')">回到首页 →</NeonButton>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

/* 和 Error404 同样的样式 — 直接复制一份（DRY 在这种 2 文件场景下 premature） */
.err-page { min-height: 100vh; background: $n-space; color: $dark-text-1;
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 40px 20px; text-align: center; }
.err-ill { margin-bottom: 28px; :deep(svg) { width: 180px; height: 180px; } }
.err-code { font-size: 120px; font-weight: 900; color: $c-pink;
  letter-spacing: -0.05em; line-height: 1; margin-bottom: 14px; }
.err-title { font-size: var(--fs-h1); font-weight: 800; letter-spacing: -0.03em; margin: 0 0 12px; }
.err-desc { color: $dark-text-3; font-size: 17px; margin: 0 0 32px; max-width: 400px; }
</style>
```

- [ ] **Step 5: 验证 + 提交**

```bash
pnpm exec vue-tsc --noEmit
pnpm dev
# 浏览器手动访问不存在的路径触发 404；或强行访问无权限路径触发 403
git add web/src/assets/illustrations/error-*.svg web/src/views/Error404.vue web/src/views/Error403.vue
git commit -m "refactor(web): 404/403 暗底 + 大粉数字 + 线稿插图"
```

---

### Task 7.2: 其余 Empty State 插图

**Files:**
- Create: `web/src/assets/illustrations/empty-audit.svg`
- Create: `web/src/assets/illustrations/empty-billing.svg`

- [ ] **Step 1: empty-audit.svg（纸卷 + 放大镜）**

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <rect x="40" y="30" width="90" height="80" rx="6" stroke="#FF6B35" stroke-width="2"/>
  <path d="M50 48 L115 48 M50 60 L115 60 M50 72 L100 72 M50 84 L115 84 M50 96 L95 96" stroke="#FF6B35" stroke-width="1.4"/>
  <circle cx="135" cy="85" r="16" stroke="#A855F7" stroke-width="2"/>
  <path d="M147 97 L160 110" stroke="#A855F7" stroke-width="2.4"/>
  <path d="M10 40 l3 0 M11.5 38.5 l0 3" stroke="#FFD600" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 2: empty-billing.svg（空钱包）**

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <path d="M30 50 L30 110 L150 110 L150 60 L130 60 L130 50 Z" stroke="#00E676" stroke-width="2"/>
  <circle cx="135" cy="85" r="8" stroke="#00E676" stroke-width="2"/>
  <path d="M30 70 L130 70" stroke="#00E676" stroke-width="1.5"/>
  <path d="M60 90 L90 90" stroke="#FF3D94" stroke-width="1.8"/>
  <path d="M12 90 l3 0 M13.5 88.5 l0 3" stroke="#A855F7" stroke-width="1.8"/>
  <path d="M165 50 l3 0 M166.5 48.5 l0 3" stroke="#FFD600" stroke-width="1.8"/>
</svg>
```

- [ ] **Step 3: 把它们接到对应页面**

- `Audit.vue` 表为空时 `<EmptyState>` 用 `empty-audit.svg`
- `Billing.vue` 账变流水为空时用 `empty-billing.svg`（覆盖原占位）

- [ ] **Step 4: 验证 + 提交**

```bash
pnpm exec vue-tsc --noEmit
git add web/src/assets/illustrations/empty-audit.svg web/src/assets/illustrations/empty-billing.svg \
        web/src/views/admin/Audit.vue web/src/views/personal/Billing.vue
git commit -m "feat(web): 追加 audit + billing 空态插图"
```

---

## Phase 8 · 最终 QA & 收尾

### Task 8.1: 全站 grep 清零校验

**Files:** (只读 + 必要时清理)

- [ ] **Step 1: 检查无 linear-gradient / radial-gradient 残留**

```bash
cd 本仓库
grep -rn "linear-gradient\|radial-gradient" web/src --include="*.vue" --include="*.scss" --include="*.css"
```

Expected: **0 matches**（除了可能存在的 Element Plus 内部默认样式 — 那些通过我们的 override 已被覆盖）。

如果有匹配，逐个定位并替换成纯色 / 低透明度 fill。

- [ ] **Step 2: 检查无 html.dark / .dark block 残留**

```bash
grep -rn "html\.dark\|^\s*\.dark\s*{" web/src --include="*.vue" --include="*.scss" --include="*.css"
```

Expected: **0 matches**（只允许 `.dark-area` 保留，那是我们的新 class）。

- [ ] **Step 3: 检查 ui.isDark / ui.toggleDark 消费点都已清**

```bash
grep -rn "ui\.isDark\|ui\.toggleDark\|toggleDark\|isDark" web/src --include="*.vue" --include="*.ts"
```

Expected: 只剩 `stores/ui.ts` 内部 `isDark` 定义和返回；其他文件应为 0 匹配。

- [ ] **Step 4: 检查所有 SVG 插图使用情况**

```bash
ls web/src/assets/illustrations/
grep -rn "illustrations/" web/src --include="*.vue"
```

Expected: 每个 SVG 至少被一个页面引用；若发现未使用，删掉或在 Phase 7 追加引用点。

- [ ] **Step 5: 完整构建**

```bash
cd 本仓库/web
pnpm exec vue-tsc --noEmit
pnpm build
```

Expected: 都成功；观察 dist 下 CSS bundle 相对原来的大小（日志会打印）。

- [ ] **Step 6: 提交（如有清理改动）**

```bash
cd 本仓库
git status
# 如有待清理的改动：
git add <修改的文件>
git commit -m "chore(web): UI 重设计最终清理 — 无残留 linear-gradient/html.dark"
```

---

### Task 8.2: 浏览器 golden path 验证

**Files:** 无代码改动

- [ ] **Step 1: 启 dev**

```bash
cd 本仓库/web
pnpm dev
```

- [ ] **Step 2: 用 /browse 或手工过 golden path**

**未登录场景：**
1. 打开 `http://localhost:5173/` — Landing hero 粉黄多色标题 + 3 色 FeatureCard
2. 点「免费注册」→ `/register` — 两栏暗亮分明
3. 点「立即登录」→ `/login` — 同构，粉色主按钮
4. 登录成功 → 自动跳 `/personal/dashboard`

**个人用户场景：**
5. Dashboard — 4 色 KPI 卡 + 折线图纯粉
6. API Keys — 生成 Key → StatusTag active
7. Usage — 紫色图表
8. Billing — 大绿色余额卡
9. OnlinePlay — tab 切换 + 黄色 active bar
10. ApiDocs — 暗色代码块

**管理员场景（切到 admin 账号）：**
11. Users — UserAvatar 6 色 + StatusTag role
12. Credits — 调账 Dialog
13. Recharges — 4 档状态 StatusTag
14. Accounts / Proxies / Models / Groups / UsageStats / AdminKeys — 都通
15. Audit / Backup / Settings — 都通

**错误页：**
16. 访问不存在路径 → 404 暗底大粉字
17. 权限不足 → 403

- [ ] **Step 3: 浏览器 console 无报错**

打开 DevTools Console，全站过一遍，无红色错误。

- [ ] **Step 4: 截图留档（可选）**

```bash
# 每个页面截图保存到 docs/superpowers/verification/2026-04-24/
mkdir -p 本仓库/docs/superpowers/verification/2026-04-24
# 手工截图保存
```

- [ ] **Step 5: 对照 Definition of Done**

去 spec §10.3 Definition of Done 逐条核对：
- [ ] 25 个页面全部按新视觉呈现
- [ ] 7 个新组件完成且至少在 2 个页面消费
- [ ] `pnpm build` 无错
- [ ] `vue-tsc --noEmit` 通过
- [ ] 全站 Landing → Login → Personal → Admin 走通 golden path
- [ ] Chrome + Safari 桌面最新版视觉无明显问题
- [ ] CSS bundle 未显著变大（+30% 以内）
- [ ] 没有残留 `linear-gradient` 关键字
- [ ] 所有 `html.dark` 相关样式已删除或迁移

- [ ] **Step 6: 如果有新截图或 verification 文档，提交**

```bash
cd 本仓库
git add docs/superpowers/verification/
git commit -m "docs(web): UI 重设计验收截图留档 2026-04-24"
```

---

## 交付与后续

- [ ] **PR 整理：** 所有 commit 挂在 `main` 或一个 `feature/ui-redesign` 分支，累计约 30+ commits
- [ ] **发布文档：** 跑 `/document-release` 更新 README 里的截图（旧的 playground-batch.png 等可保留，新增的 Landing / Admin 截图）
- [ ] **后续迭代：** 暗色 toggle 回加、移动端适配、更多插图、主题 preset 切换（不同主色方案）均可作单独 issue

---

**Note:** 本 plan 的每个任务都自包含，subagent 可独立执行。执行时务必按顺序，跨 Phase 时确认前置已 commit。
