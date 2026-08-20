# 前端全站视觉重设计 · Neon Solid

> 项目：`gpt2api/web/` · Vue 3 + Element Plus + Vite + TypeScript + SCSS
> 日期：2026-04-24
> 作者：Dennis (jiji262) × Claude Code

## 1. 背景与目标

### 1.1 当前状态
- 技术栈：Vue 3 + Element Plus 2.7 + Vite + TypeScript + Pinia + SCSS
- 视觉风格：**典型 Element Plus 管理后台模板** — 蓝/绿 accent、深色 sidebar(`#1f2330`) + 浅色主区(`#f5f7fa`)、圆角 8px、弱阴影、14px 正文
- 页面总数：25 个 Vue 文件（Landing 1 + Auth 2 + Personal 6 + Admin 12 + Errors 2 + Layouts 2）

### 1.2 要解决的问题
现有视觉太"模板化" — 与竞品 SaaS 后台几乎同质，缺少品牌记忆点，与 gpt2api 本身的 AI 图像生成能力在气质上不匹配。

### 1.3 目标
1. **品牌区分度**：让人一眼记住"那个有调色盘 hero 的 GPT API 网关"
2. **专业 + 生动并存**：保留后台工具的理性感，但通过配色和插画注入产品个性
3. **不改功能、不改信息架构**：纯视觉层重设计 — 路由、权限、业务逻辑、API 调用全部保持不动
4. **可维护性**：新的设计系统要比现有 SCSS 更有条理（tokens + 组件化），未来加页面成本更低

### 1.4 非目标（Out of Scope）
- ❌ 不新增/删除页面
- ❌ 不改 API / 数据模型 / 路由权限
- ❌ 不做移动端适配（desktop-only，≥1024px 最佳体验；保留现有 flex 布局的自然降级）
- ❌ 不替换 Element Plus（用 theme override + 必要时外层自定义组件）
- ❌ 不动 README 业务截图（截图 stale 属后续 `/document-release` 范畴）

## 2. 视觉决策摘要（Brainstorming 阶段已定）

| 决策 | 选择 | 备注 |
|------|------|------|
| 整体风格 | **Neon Glow** | 深紫蓝底 + 多彩 accent，气质接近 Linear / Vercel AI |
| 暗色范围 | **② 混合** | Landing + Login/Register 全暗；Personal + Admin 亮底 + 深紫 sidebar |
| 字体 | **Space Grotesk × 思源黑体 + JetBrains Mono** | 中英文同字族方向一致，代码统一 mono |
| 插图风格 | **多色 Neon 涂鸦** | 纯描边（1.8-2.2px）、多色分层、无填充 |
| 配色 | **6 纯色无渐变** | Hot Pink / Electric Cyan / Acid Yellow / Royal Purple / Hot Orange / Neon Green |

详细 mood board / 对比过程见 `.superpowers/brainstorm/20262-1777035564/content/` 下 HTML 文件。

## 3. 设计系统（Design System）

### 3.1 颜色 Tokens

```scss
// src/styles/tokens.scss

// ===== 品牌色（Brand / Accent）=====
$c-pink:    #FF3D94;  // 主色 Hot Pink - 用于主按钮、默认 accent、logo mark
$c-cyan:    #00D9FF;  // Electric Cyan - 用于次要操作、图表轴线、info 标签
$c-yellow:  #FFD600;  // Acid Yellow - 用于警示、特殊强调、hover
$c-purple:  #A855F7;  // Royal Purple - 用于管理员角色、sidebar 文字高亮
$c-orange:  #FF6B35;  // Hot Orange - 用于 admin 角色、danger
$c-green:   #00E676;  // Neon Green - 用于 success、active 状态、充值成功

// ===== 中性色（Neutrals）=====
$n-space:   #0A0718;  // Deep Space - Landing 主背景
$n-ink-p:   #130A24;  // Ink Purple - 暗色面板（sidebar）/ 暗色卡片底色
$n-ink:     #1A1A2E;  // Ink - 亮色主区的文字主色
$n-cloud:   #FAFAFB;  // Cloud - 亮色主区背景
$n-paper:   #FFFFFF;  // Paper - 亮色卡片底

// ===== 灰阶（Gray Scale，衍生）=====
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

// ===== 暗色文字色阶 =====
$dark-text-1: #FFFFFF;          // 主标题
$dark-text-2: #D8D1EC;          // 正文
$dark-text-3: #B0A7C9;          // 辅助文字
$dark-text-4: #7E7593;          // disabled / 说明

// ===== 深色面板描边 =====
$dark-border: rgba(255,255,255,0.08);
$dark-border-strong: rgba(255,255,255,0.15);
```

**SCSS vs CSS 变量的分工**：
- **SCSS 变量**（上面 `$c-pink` 这种）：编译期使用，用于组件 `.scss` 文件内部计算（如 `lighten()`, `darken()`, 数学运算）和 Element Plus override 编译
- **CSS 变量**（下面 `--c-pink` 这种）：运行时使用，供动态主题切换和 Element Plus 消费

CSS 变量导出：

```scss
// src/styles/tokens.scss 下半部分
:root {
  --c-pink: #{$c-pink};
  --c-cyan: #{$c-cyan};
  --c-yellow: #{$c-yellow};
  --c-purple: #{$c-purple};
  --c-orange: #{$c-orange};
  --c-green: #{$c-green};
  --n-space: #{$n-space};
  --n-ink-p: #{$n-ink-p};
  --n-ink: #{$n-ink};
  --n-cloud: #{$n-cloud};
  --n-paper: #{$n-paper};

  // 语义别名(常用组件消费)
  --app-bg: var(--n-cloud);
  --panel-bg: var(--n-paper);
  --sidebar-bg: var(--n-ink-p);
  --text-primary: var(--n-ink);
  --text-muted: #{$gray-500};
  --border: #{$gray-200};
  --accent: var(--c-pink);
}

// 暗色区——Landing / Login / Register / Errors / Sidebar 的祖先或自身挂 dark-area
.dark-area {
  --app-bg: var(--n-space);
  --panel-bg: var(--n-ink-p);
  --text-primary: #{$dark-text-1};
  --text-muted: #{$dark-text-3};
  --border: #{$dark-border};
}
```

**配色使用规则**：
- 每个页面/区块挑 **1 个"当前色"** 作为主 accent（kicker、边框、按钮、下划线都用同一个）
- 6 色在不同区块之间轮换分配（Feature Card 依次 pink / cyan / yellow，KPI 卡依次 pink / cyan / yellow / purple）
- **永远不混用两色做渐变**。单一色相内做深浅可以（如 `#FF3D94` hover → `#E82A80`）
- 不用 `box-shadow` 做 glow 光晕；阴影只用灰色做深度

### 3.2 字体 Tokens

**字体家族**
```scss
$f-sans: "Space Grotesk", "Noto Sans SC", -apple-system, BlinkMacSystemFont, "PingFang SC", sans-serif;
$f-mono: "JetBrains Mono", "SF Mono", Menlo, Consolas, monospace;
```

**字号梯度**（所有 px 都向上跳一档，符合"字体可以大一些"）

| Token | Size | Line | Weight | 用处 |
|-------|------|------|--------|------|
| `--fs-hero` | 80px | 0.98 | 800 | Landing H1 |
| `--fs-h1` | 48px | 1.05 | 800 | Section title |
| `--fs-h2` | 32px | 1.1 | 800 | 页面大标题（admin page title） |
| `--fs-h3` | 24px | 1.15 | 800 | Feature card title / panel title |
| `--fs-h4` | 18px | 1.2 | 700 | 小面板标题 |
| `--fs-lead` | 19px | 1.55 | 400 | Hero tagline |
| `--fs-body` | 16px | 1.6 | 400 | 正文（比原 14px 大一档） |
| `--fs-ui` | 14px | 1.5 | 500 | 表格、表单控件 |
| `--fs-sm` | 13px | 1.5 | 500 | 说明性小字 |
| `--fs-xs` | 12px | 1.5 | 700 | Label / kicker / crumb（带 letter-spacing） |

**字体加载**：用 `@fontsource/*` 自托管，不走 Google Fonts CDN。

```bash
pnpm add @fontsource/space-grotesk @fontsource-variable/noto-sans-sc @fontsource/jetbrains-mono
```

**letter-spacing**：
- Hero / H1 / H2 / H3：`-0.02em` ~ `-0.035em`（字越大收越紧）
- Body：`0`
- Label / Kicker：`0.14em` ~ `0.2em`

### 3.3 形状与间距

```scss
// 圆角
--r-sm:  6px;      // tag / small badge
--r-md:  10px;     // button / input
--r-lg:  14px;     // card / panel
--r-xl:  20px;     // hero section / large feature
--r-pill: 999px;   // chip / avatar

// 边框
--bw: 1.5px;       // 所有卡片、输入框统一 1.5px

// 阴影（只有两档，不用 glow）
--sh-1: 0 1px 2px rgba(15,15,30,0.04);                  // 卡片静态
--sh-2: 0 4px 16px rgba(15,15,30,0.08);                 // hover 悬浮
```

spacing 直接用数字（4 / 8 / 12 / 16 / 20 / 24 / 32 / 40 / 48 / 60 / 80）。

### 3.4 图标 vs 插图

- **图标**：继续用 `@element-plus/icons-vue`，尺寸 16px（UI 内联）/ 18-20px（按钮）/ 24px（sidebar）
- **插图**：全部 SVG 手绘线稿，存 `src/assets/illustrations/*.svg`，通过 `vite-svg-loader` 以 Vue 组件形式 import
  - 规范：viewBox 统一 200×180 或 180×140；描边 1.8-2.2px；`stroke-linecap="round"`，`stroke-linejoin="round"`；填充全部 `fill="none"`；所有 `stroke` 写内联颜色（直接用 6 个品牌色 hex）
  - 约 15-20 个插图覆盖：hero 主插图、3 个 feature card、5 个 empty state（users / keys / recharges / audit / billing）、登录页装饰、404 / 403、dashboard 4 个 KPI 角标等

### 3.5 动效

- **按钮 hover**：`transform: translateY(-1px)` + 颜色变深一档（约 `darken(8%)`），过渡 150ms
- **卡片 hover**：`translateY(-2px)` + `--sh-2`，过渡 200ms
- **页面切换**：保留现有 `fade` transition (150ms opacity)
- **不做**：持续脉冲、粒子浮动、glow 呼吸、霓虹闪烁 — 保留专业感，避免过度

## 4. 共享组件库（`src/components/`）

### 4.1 新增 7 个组件

| 组件 | Props | 用处 |
|------|-------|------|
| `<NeonButton variant="pink\|cyan\|yellow\|purple\|green\|orange\|ink\|outline" size="sm\|md\|lg">` | 主按钮封装，替代零散 `<el-button type="primary">` |
| `<KpiCard accent="pink\|cyan\|yellow\|purple" :label :value :change> + #illustration slot` | 仪表盘指标卡 — 左 5px 纯色条 + 右下角角标插图 |
| `<StatusTag variant="pro\|free\|admin\|active\|disabled\|pink\|cyan\|yellow\|purple\|green\|orange">` | 标签 — 描边式（border + 透明底），替代 `<el-tag>` 的滥用 |
| `<PageHeader :crumb :title :accent>` | Admin / Personal 顶部：小写 crumb + 大 H2（可选 accent word 上色） |
| `<EmptyState :title :desc :illustration :action>` | 空状态，必带插图 |
| `<UserAvatar :name :size="sm\|md\|lg">` | 头像 — 按名字首字母 hash 分配 6 纯色之一 |
| `<FeatureCard accent :kicker :title :desc> + #illustration` | Landing 的 3 张 feature 卡 |

### 4.2 修改现有组件 / 布局

- `BasicLayout.vue` — Sidebar 深紫 + active 态改 3px pink 左边线；Topbar 保留 crumb，移除 dark-mode 切换按钮（见 5.2）
- `BlankLayout.vue` — 给 `class="dark-area"`，让 Login / Register 默认暗底

### 4.3 Element Plus 主题覆盖

新建 `src/styles/element-override.scss`，`@use 'tokens' as *` 先拿到 SCSS 变量，然后覆盖 Element Plus 的 `--el-*` CSS 变量：

```scss
@use './tokens' as *;

:root {
  --el-color-primary: #{$c-pink};
  --el-color-primary-light-3: #{lighten($c-pink, 8%)};
  --el-color-primary-dark-2: #{darken($c-pink, 8%)};
  --el-color-success: #{$c-green};
  --el-color-warning: #{$c-yellow};
  --el-color-danger: #{$c-orange};
  --el-color-info: #{$c-cyan};

  --el-border-radius-base: 10px;
  --el-border-radius-small: 6px;

  --el-font-family: #{$f-sans};

  --el-text-color-primary: #{$n-ink};
  --el-text-color-regular: #{$gray-700};
  --el-text-color-secondary: #{$gray-500};

  --el-bg-color: #{$n-paper};
  --el-bg-color-page: #{$n-cloud};
  --el-border-color: #{$gray-200};
  --el-border-color-light: #{$gray-100};
}

// 组件级别针对性调整
.el-table {
  --el-table-header-bg-color: #{$n-cloud};
  --el-table-header-text-color: #{$gray-500};
  // 表头加 uppercase + letter-spacing 的 kicker 风格
  th.el-table__cell > .cell {
    font-size: 12px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    font-weight: 700;
  }
}

.el-button {
  border-radius: 10px;
  font-weight: 700;
  &--primary:not(.is-disabled) { background: $c-pink; border-color: $c-pink; }
}

.el-input__wrapper {
  border-radius: 10px;
  box-shadow: 0 0 0 1.5px $gray-200 inset;
  &.is-focus { box-shadow: 0 0 0 1.5px $c-purple inset, 0 0 0 3px rgba(168,85,247,0.15); }
}
```

删掉现有 `html.dark { ... }` 块（我们不再做全局 dark toggle，见 5.2）。

## 5. 页面级别方案

所有页面保留现有功能和数据流，仅替换视觉。下表按重要度排序。

### 5.1 Landing · `Home.vue`

最大改动。Hero + Features + CTA 三段结构保持，视觉彻底重做。

- Hero 区：
  - Eyebrow（紫色 ● + 大写小字）
  - H1 80px，多色拼词：「给你的 **AI**(pink)<br>一个**调色盘**(yellow)。」
  - Tagline 19px，maxw 560px
  - CTA：主按钮 pink 实色 + outline 按钮
  - 右侧 Hero 插图：三个重叠画框（pink / yellow / cyan）+ 画笔（purple）+ 散点（多色）
  - 底部 4 个统计（2,384+ 开发者 · 12M 调用 · 99.9% 成功率 · <30s P95）
- Features 区：3 张 FeatureCard，分别用 pink / cyan / yellow 作主色，每张配一张不同的小线稿插图
- 原有截图 gallery（`docs/screenshots/playground-*`）继续保留，放到 Features 之后作为 "See it in action" 区，给一个暗色 panel 包起来，大圆角 + 1px dark-border
- 底部 CTA 区：大标题 + 主按钮，简短版
- 导航：暗色透明 → 滚动后加 backdrop-blur 实色背景（保留现有 `scrolled` 逻辑）

### 5.2 Auth · `Login.vue` / `Register.vue`

两栏布局（50/50）：
- **左栏（暗色）**：Logo + 大标题「欢迎<br>回来(accent)」 + Tagline + 3 条 feature bullet + 右下角装饰小插图（钥匙 / 信件）
- **右栏（白色）**：表单居中 — 标题 + 说明 + Label(uppercase xs) + Input(1.5px border) + 主按钮 pink full-width + 底部说明

核心改动：
- Login.vue 无 toggle，但使用了 `.dark` SCSS class 做样式分支（7 处），改为默认暗底（`class="dark-area"` on root），移除 `.dark` 分支
- Register.vue 同上（3 处 `.dark` 引用）
- 把 Turnstile 小字挪到按钮下方

**关于 `ui.toggleDark()` 的决策**：
- 当前代码实际调用点（grep 确认）：
  - `views/landing/Home.vue` : `:class="{ dark: ui.isDark }"` + 一个切换按钮（第 59, 70-73 行）
  - `layouts/BasicLayout.vue` : 一个切换按钮（第 114-117 行）
  - `stores/ui.ts` : `useDark() + useToggle()` 定义
- 新方案：**移除用户可见的 dark toggle**。Landing / Auth 恒暗，Personal / Admin 恒亮。理由：
  1. 混合模式下切换不再有清晰语义（Landing 暗切什么？）
  2. 工作场景的 Admin 恒亮最友好（见 brainstorming 决策 ②）
- 具体清理：
  1. Home.vue 删 `:class="{ dark: ui.isDark }"` 和 toggle 按钮
  2. BasicLayout.vue 删 theme-btn 按钮块
  3. stores/ui.ts 保留文件（其他功能如果有会继续用），删掉 `isDark` / `toggleDark` 暴露
  4. 全部组件里搜 `ui.isDark` 和 `ui.toggleDark` 确认无残留（grep 确认当前只这两处）

### 5.3 Layouts · `BasicLayout.vue`

Sidebar（240px，收起 64px）：
- 背景 `$n-ink-p` (#130A24)
- Logo：28px pink 纯色方块 + 白色"GPT2API"
- 分组标题（「个人」/「管理」）：xs 紫色 uppercase
- Menu item：
  - 默认：14px 浅紫 `$dark-text-3`
  - hover：白字 + `rgba(255,255,255,0.03)` 底
  - **active**：白字 + `rgba(255,61,148,0.12)` 底 + 3px pink 左边线（`padding-left: 7px` 补偿）
- Icon：18px，跟当前文字色

Topbar（56px）：
- 白底 + 下边线
- 左：collapse 按钮 + crumb（kicker 小字）+ 大 H2 标题（可带 accent word）
- 右：用户头像（UserAvatar 组件）+ 昵称 + 角色 tag（StatusTag "admin"）
- **移除** dark-mode 切换按钮

Main：白底（`$n-cloud`）+ 顶部 router-view 容器 padding 28px 32px 40px。

### 5.4 Personal（6 页）

所有页面走 `BasicLayout`，都是亮底 + 纯色 accent。

| 页面 | 主要视觉模式 | accent 主色 |
|------|--------------|-------------|
| `Dashboard.vue` | Greeting + 4 KpiCard + 两栏 Panel（趋势图 + Keys 列表） | pink（主页主色） |
| `ApiKeys.vue` | PageHeader + 工具条 + StatusTag 表格 + 新建 Dialog | cyan |
| `Usage.vue` | PageHeader + 时间筛选 + KpiCard × 3 + 折线图 + 明细表格 | purple |
| `Billing.vue` | PageHeader + 余额大卡 + 充值按钮 + 账变流水表 | green |
| `OnlinePlay.vue` | PageHeader + 左右两栏：prompt 表单 + 图片输出网格 | yellow |
| `ApiDocs.vue` | PageHeader + 左目录 + 右代码块（JetBrains Mono + 紫粉青语法高亮） | pink |

### 5.5 Admin（12 页）

通用模式：`<PageHeader>` + `<FilterBar>` + `<el-table>`（主题覆盖样式）+ 分页 + 操作 Dialog。

| 页面 | accent | 特殊说明 |
|------|--------|----------|
| `Users.vue` | pink | 行内 UserAvatar + role StatusTag + 3 操作链接（编辑/调账/重置/删除） |
| `Credits.vue` | green | 调账 Dialog 里金额用大 h3 + 纯色 |
| `Recharges.vue` | yellow | 状态 StatusTag 四档（pending/paid/refunded/failed） |
| `Accounts.vue` | cyan | GPT 账号池 — 健康度用纯色圆点 |
| `Proxies.vue` | purple | 代理池同上 |
| `Models.vue` | pink | 模型列表 + 启用开关 |
| `Groups.vue` | cyan | 分组管理 — 权限矩阵用 tag |
| `UsageStats.vue` | purple | 大图表区（折线 + 饼图） |
| `AdminKeys.vue` | yellow | 全站 key 明细 |
| `Audit.vue` | orange | 审计日志 — 大表格，严肃感强 |
| `Backup.vue` | green | 操作卡片 + 历史备份列表 |
| `Settings.vue` | pink | Tab 切换 + 表单组 |

### 5.6 Errors · `Error403.vue` / `Error404.vue`

独立全屏（不套 BasicLayout），暗底：
- 居中大数字（200px，pink 纯色）
- H2 标题（「没有权限」/「找不到页面」）
- 说明 + 回首页按钮
- 下方一个和错误类型相关的小插图（403 锁 / 404 迷路指南针）

## 6. 文件改动清单

### 6.1 新增文件

```
web/src/styles/
  ├── tokens.scss                  (新，颜色 / 字体 / 间距变量)
  ├── element-override.scss        (新，Element Plus 主题覆盖)
  ├── fonts.scss                   (新，@fontsource 引入)
  └── global.scss                  (重写，只留基础 reset + scrollbar)

web/src/components/
  ├── NeonButton.vue               (新)
  ├── KpiCard.vue                  (新)
  ├── StatusTag.vue                (新)
  ├── PageHeader.vue               (新)
  ├── EmptyState.vue               (新)
  ├── UserAvatar.vue               (新)
  ├── FeatureCard.vue              (新)
  └── Placeholder.vue              (保留)

web/src/assets/illustrations/      (新，15-20 个 svg)
  ├── hero-palette.svg
  ├── feature-img2.svg
  ├── feature-batch.svg
  ├── feature-openai.svg
  ├── empty-users.svg
  ├── empty-keys.svg
  ├── empty-recharges.svg
  ├── empty-audit.svg
  ├── empty-billing.svg
  ├── auth-key.svg
  ├── error-403.svg
  ├── error-404.svg
  ├── kpi-wallet.svg
  ├── kpi-chart.svg
  ├── kpi-check.svg
  └── kpi-chain.svg
```

### 6.2 修改文件

所有 25 个 Vue 文件（Landing + Auth + Personal + Admin + Errors + Layouts）。改动仅限 `<template>` 和 `<style>`，`<script setup>` 业务逻辑保持不动。

### 6.3 依赖改动

`web/package.json` 新增：
```json
"@fontsource/space-grotesk": "^5.0.0",
"@fontsource-variable/noto-sans-sc": "^5.0.0",
"@fontsource/jetbrains-mono": "^5.0.0",
"vite-svg-loader": "^5.1.0"
```

`web/vite.config.ts` 加入 `vite-svg-loader` 插件。

## 7. 实施阶段（按此顺序执行）

### Phase 0 · 清理 & 依赖
1. 卸载一些不再使用的样式依赖（如果有）
2. 装新字体 + svg-loader
3. 在 `main.ts` 引入 fonts.scss

### Phase 1 · 设计 tokens + Element 主题
1. 写 `tokens.scss`（颜色、字体、间距）
2. 写 `element-override.scss`
3. 重写 `global.scss`（只留 reset + scrollbar，删掉 `.page-title` `.card-block` 等旧 class 或保留兼容）
4. 验证：把 `Users.vue` 这种大量用 Element 组件的页面打开，看主题色是否已生效

### Phase 2 · 共享组件
1. 按组件库清单逐个实现 7 个新组件
2. 每个组件写最小示例页面（开发用）

### Phase 3 · 布局 & 公开页
1. `BasicLayout.vue`（新 sidebar + topbar）
2. `BlankLayout.vue`（给 dark-area class）
3. `Home.vue`（Landing 全量重做）
4. `Login.vue` + `Register.vue`

### Phase 4 · Personal
Dashboard → ApiKeys → Usage → Billing → OnlinePlay → ApiDocs（按使用频率）

### Phase 5 · Admin
Users → Credits → Recharges → Accounts → Proxies → Models → Groups → UsageStats → AdminKeys → Audit → Backup → Settings

### Phase 6 · Errors + 插图补齐
Error403 / Error404 + 所有 EmptyState 的 SVG

### Phase 7 · QA & Polish
- 全站手工过一遍（每页截屏）
- 用 `/browse` 或 `/qa` 跑一轮
- 对暗色 / 亮色 contrast 做可读性 check
- 收尾：移除旧注释 / 死代码

## 8. 验证与测试策略

- **单元测试**：无（Vue 组件没有现存单测基建，本次只改视觉，不引入）
- **视觉 QA**：每个 Phase 结束在本地 `pnpm dev` 人工过一遍，golden path 为
  - 未登录打开 `/` → 看 Landing → 点「免费注册」→ Register 成功 → 自动跳 Dashboard
  - 登录后 → Dashboard → API Keys 生成 → OnlinePlay 出一张图 → Billing 看账变
  - 管理员登录 → Users 改用户 → Credits 调账 → Settings 改配置
- **构建校验**：`pnpm build` 必须 `vue-tsc --noEmit` 通过 + Vite 打包成功
- **兼容性**：Chrome / Safari / Firefox 最新两版；不支持 IE
- **可访问性**：确保 accent color 在各自背景下 WCAG AA（4.5:1）
  - Hot Pink 在 Deep Space 上 contrast = 5.2 ✓
  - Electric Cyan 在 Deep Space 上 contrast = 10.8 ✓
  - Hot Pink 在 Cloud 上 contrast = 3.8 ✗（不做大面积正文用，只做 accent / button 背景）

## 9. 风险与应对

| 风险 | 应对 |
|------|------|
| Element Plus 版本升级破坏主题覆盖 | 锁版本 ^2.7.2，升级时重测 |
| 字体加载慢导致 FOUT | `@fontsource` 自托管 + `font-display: swap` |
| 手绘 SVG 耗时超预期 | 优先做必要的 7-8 个（hero + 3 feature + 3-4 empty state），其他迭代补 |
| 配色在某个页面打架 | 每页只用 1 个主色 + 中性色；6 色之间轮换分配到不同区块 |
| 现有页面 `<el-table>` 太多，改样式容易漏 | 主题 override 走 `.el-table` 全局选择器，一次搞定 |
| 现有代码里 31 处 `linear-gradient` / `.dark` 引用（grep 确认）散在 13 个文件 | Phase 1 先把 `global.scss` 里的 `html.dark { ... }` 块删掉，Phase 3-6 按页面清理时逐个消除；最后用 grep 校验（Definition of Done 条目） |
| `ui.isDark` 只在 Home.vue (4 处) 和 BasicLayout.vue (1 处) 消费 | Phase 3 随着 layout + landing 重写一并清掉，`stores/ui.ts` 导出也收敛 |

## 10. 附录

### 10.1 Brainstorming 过程文件

```
.superpowers/brainstorm/20262-1777035564/content/
  ├── visual-direction.html       (4 风格 mood board → 选 B)
  ├── admin-scope.html            (3 暗色策略 → 选 ②)
  ├── typography.html             (3 字体组合 → 选 A Space Grotesk)
  ├── illustrations.html          (3 插图风格 → 选 B 多色涂鸦)
  ├── composed-preview.html       (第一版成品，渐变版)
  └── solid-palette.html          (纯色版，最终选定)
```

### 10.2 示例：一个页面从旧到新的对照（Users.vue 片段）

**旧**：
```vue
<el-table :data="rows">
  <el-table-column prop="email" label="邮箱" />
  <el-table-column label="角色">
    <template #default="{ row }">
      <el-tag :type="row.role === 'admin' ? 'warning' : ''" size="small">
        {{ row.role }}
      </el-tag>
    </template>
  </el-table-column>
  ...
</el-table>
```

**新**：
```vue
<el-table :data="rows" class="neon-table">
  <el-table-column label="用户">
    <template #default="{ row }">
      <div class="u-cell">
        <UserAvatar :name="row.nickname || row.email" size="md" />
        <div>
          <div class="u-email">{{ row.email }}</div>
          <div class="u-sub">ID #{{ row.id }} · {{ row.nickname }}</div>
        </div>
      </div>
    </template>
  </el-table-column>
  <el-table-column label="角色">
    <template #default="{ row }">
      <StatusTag :variant="row.role === 'admin' ? 'admin' : (row.role === 'pro' ? 'pro' : 'free')">
        {{ row.role }}
      </StatusTag>
    </template>
  </el-table-column>
  ...
</el-table>
```

CSS 靠 element-override.scss 全局接管，不在组件内重复写。

### 10.3 Definition of Done

- [ ] 25 个页面全部按新视觉呈现
- [ ] 7 个新组件完成且至少在 2 个页面消费
- [ ] `pnpm build` 无错
- [ ] `vue-tsc --noEmit` 通过
- [ ] 全站 Landing → Login → Personal → Admin 走通 golden path
- [ ] Chrome + Safari 桌面最新版视觉无明显问题
- [ ] CSS bundle 未显著变大（+30% 以内）
- [ ] 没有残留 `linear-gradient` 关键字（除 animation 可能的个别场景外）
- [ ] 所有 `html.dark` 相关样式已删除或迁移
