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
