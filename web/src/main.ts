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
