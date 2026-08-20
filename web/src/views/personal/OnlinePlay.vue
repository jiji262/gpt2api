<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useUserStore } from '@/stores/user'
import { formatCredit } from '@/utils/format'
import {
  listMyModels,
  streamPlayChat,
  playGenerateImage,
  type SimpleModel,
  type PlayChatMessage,
  type PlayImageData,
} from '@/api/me'
import { ENABLE_CHAT_MODEL } from '@/config/feature'

// ----------------------------------------------------
// 用户 / 模型
// ----------------------------------------------------
const userStore = useUserStore()
const { user } = storeToRefs(userStore)

const balance = computed(() => formatCredit(user.value?.credit_balance))

const models = ref<SimpleModel[]>([])
const chatModels = computed(() => models.value.filter((m) => m.type === 'chat'))
const imageModels = computed(() => models.value.filter((m) => m.type === 'image'))

const selectedChatModel = ref('')
const selectedImageModel = ref('')

const currentChatDesc = computed(
  () => chatModels.value.find((m) => m.slug === selectedChatModel.value)?.description || '',
)
const currentImageDesc = computed(
  () => imageModels.value.find((m) => m.slug === selectedImageModel.value)?.description || '',
)

onMounted(async () => {
  try {
    await userStore.fetchMe()
  } catch {
    /* ignore */
  }
  try {
    const m = await listMyModels()
    // feature flag 关闭时,前端直接把 chat 类型的模型从列表过滤掉,
    // 保证 chatModels / imageModels / selectedChatModel 等下游 state 都不会
    // 拿到 chat 模型(即便模板里还有残留引用)。
    models.value = ENABLE_CHAT_MODEL
      ? m.items
      : m.items.filter((x) => x.type !== 'chat')
    const firstChat = m.items.find((x) => x.type === 'chat')
    const firstImage = m.items.find((x) => x.type === 'image')
    if (firstChat) selectedChatModel.value = firstChat.slug
    if (firstImage) selectedImageModel.value = firstImage.slug
  } catch {
    // 静默;错误拦截器已提示
  }
})

// ----------------------------------------------------
// Tabs
// ----------------------------------------------------
const activeTab = ref<'chat' | 'text2img' | 'img2img'>(
  ENABLE_CHAT_MODEL ? 'chat' : 'text2img',
)

// ====================================================
// 对话(Chat)
// ====================================================
interface UIMessage {
  id: number
  role: 'user' | 'assistant' | 'system'
  content: string
  pending?: boolean
  error?: boolean
  at: number
}

let uid = 0

const systemPrompt = ref('你是一个友好、博学、回答精准的中文助手。回答中若涉及代码请使用 Markdown 代码块。')
const temperature = ref(0.7)
const chatInput = ref('')
const chatMsgs = ref<UIMessage[]>([])
const chatSending = ref(false)
const chatAbort = ref<AbortController | null>(null)
const chatScroll = ref<HTMLElement | null>(null)
const inputRef = ref<any>(null)

const suggestions = [
  { icon: '💡', title: '向我解释', sub: '量子纠缠到底是什么?' },
  { icon: '✍️', title: '帮我写作', sub: '一段 200 字的产品发布文案' },
  { icon: '🧑‍💻', title: '写段代码', sub: 'Go 实现令牌桶限流器' },
  { icon: '🌏', title: '中英互译', sub: '把上面这段翻译为英文' },
]

function useSuggestion(s: typeof suggestions[number]) {
  chatInput.value = `${s.title}:${s.sub}`
  nextTick(() => inputRef.value?.focus?.())
}

async function scrollChat(force = false) {
  await nextTick()
  const el = chatScroll.value
  if (!el) return
  if (force) {
    el.scrollTop = el.scrollHeight
    return
  }
  const gap = el.scrollHeight - el.scrollTop - el.clientHeight
  if (gap < 180) el.scrollTop = el.scrollHeight
}

async function sendChat() {
  if (chatSending.value) return
  const text = chatInput.value.trim()
  if (!text) return
  if (!selectedChatModel.value) {
    ElMessage.warning('请选择一个文字模型')
    return
  }
  const now = Date.now()
  chatMsgs.value.push({ id: ++uid, role: 'user', content: text, at: now })
  chatInput.value = ''
  const assistant: UIMessage = { id: ++uid, role: 'assistant', content: '', pending: true, at: now }
  chatMsgs.value.push(assistant)
  await scrollChat(true)

  const history: PlayChatMessage[] = []
  if (systemPrompt.value.trim()) {
    history.push({ role: 'system', content: systemPrompt.value.trim() })
  }
  for (const m of chatMsgs.value.slice(0, -1)) {
    if (m.error) continue
    history.push({ role: m.role as 'user' | 'assistant' | 'system', content: m.content })
  }

  chatSending.value = true
  chatAbort.value = new AbortController()
  try {
    await streamPlayChat(
      { model: selectedChatModel.value, messages: history, temperature: temperature.value },
      (delta) => {
        assistant.content += delta
        assistant.pending = false
        scrollChat()
      },
      chatAbort.value.signal,
    )
    assistant.pending = false
    if (!assistant.content) assistant.content = '(无输出)'
  } catch (err: unknown) {
    assistant.pending = false
    assistant.error = true
    const msg = err instanceof Error ? err.message : String(err)
    assistant.content = assistant.content || `请求失败:${msg}`
    ElMessage.error(msg)
  } finally {
    chatSending.value = false
    chatAbort.value = null
    scrollChat()
    userStore.fetchMe().catch(() => {})
  }
}

function stopChat() {
  chatAbort.value?.abort()
}

function resetChat() {
  if (chatSending.value) stopChat()
  chatMsgs.value = []
}

async function regenerate() {
  if (chatSending.value) return
  // 去掉最后一条 assistant,把最后一条 user 重发
  let lastUserIdx = -1
  for (let i = chatMsgs.value.length - 1; i >= 0; i--) {
    if (chatMsgs.value[i].role === 'user') { lastUserIdx = i; break }
  }
  if (lastUserIdx < 0) return
  const lastUserText = chatMsgs.value[lastUserIdx].content
  chatMsgs.value = chatMsgs.value.slice(0, lastUserIdx)
  chatInput.value = lastUserText
  await sendChat()
}

function copyText(s: string) {
  try {
    navigator.clipboard.writeText(s)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败')
  }
}

onBeforeUnmount(() => chatAbort.value?.abort())

// ---------- 轻量 markdown 渲染(代码块 / 行内代码 / 粗体 / 链接) ----------
function escapeHtml(s: string) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function renderMarkdown(raw: string): string {
  if (!raw) return ''
  const parts: string[] = []
  const blocks = raw.split(/```/g) // ``` 成对切分
  for (let i = 0; i < blocks.length; i++) {
    const chunk = blocks[i]
    if (i % 2 === 1) {
      // 代码块:首行可能是 lang
      const nl = chunk.indexOf('\n')
      let lang = ''
      let code = chunk
      if (nl >= 0) {
        const head = chunk.slice(0, nl).trim()
        if (/^[a-zA-Z0-9+_\-]{1,20}$/.test(head)) {
          lang = head
          code = chunk.slice(nl + 1)
        }
      }
      parts.push(
        `<pre class="mdk-pre" data-lang="${escapeHtml(lang || '')}"><code>${escapeHtml(
          code.replace(/\n$/, ''),
        )}</code></pre>`,
      )
    } else {
      // 行内元素
      let html = escapeHtml(chunk)
      // 行内代码 `xxx`
      html = html.replace(/`([^`\n]+)`/g, '<code class="mdk-ic">$1</code>')
      // 粗体 **xxx**
      html = html.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>')
      // 自动链接
      html = html.replace(
        /(https?:\/\/[\w\-._~:/?#\[\]@!$&'()*+,;=%]+)/g,
        '<a href="$1" target="_blank" rel="noopener">$1</a>',
      )
      // 换行
      html = html.replace(/\n/g, '<br />')
      parts.push(html)
    }
  }
  return parts.join('')
}

// ====================================================
// 文生图(Text2Img)
// ====================================================

// 10 档比例:对应上游 chatgpt.com 实际靠 prompt 第一行 "Make the aspect ratio X:Y , "
// 控制画面比例。OpenAI 兼容 size 仅作占位,按宽高比就近映射到官方支持的三档。
interface RatioOpt {
  label: string      // 中文名:方形 / 宽屏 / 竖版 …
  ratio: string      // 比例文本:1:1 / 21:9 …
  w: number          // 宽
  h: number          // 高
  size: string       // 发给后端的 OpenAI size
}
const RATIOS: readonly RatioOpt[] = [
  { label: '方形',   ratio: '1:1',  w: 1,  h: 1,  size: '1024x1024' },
  { label: '横屏',   ratio: '5:4',  w: 5,  h: 4,  size: '1792x1024' },
  { label: '故事',   ratio: '9:16', w: 9,  h: 16, size: '1024x1792' },
  { label: '超宽屏', ratio: '21:9', w: 21, h: 9,  size: '1792x1024' },
  { label: '宽屏',   ratio: '16:9', w: 16, h: 9,  size: '1792x1024' },
  { label: '横屏',   ratio: '4:3',  w: 4,  h: 3,  size: '1792x1024' },
  { label: '宽幅',   ratio: '3:2',  w: 3,  h: 2,  size: '1792x1024' },
  { label: '标准',   ratio: '4:5',  w: 4,  h: 5,  size: '1024x1792' },
  { label: '竖版',   ratio: '3:4',  w: 3,  h: 4,  size: '1024x1792' },
  { label: '竖版',   ratio: '2:3',  w: 2,  h: 3,  size: '1024x1792' },
] as const

// 预览小框的尺寸(按比例缩放后的 CSS px),保证所有档都落在 ≤36x36 的方格内。
function ratioBoxStyle(r: RatioOpt) {
  const MAX = 36
  const ar = r.w / r.h
  const bw = ar >= 1 ? MAX : Math.round(MAX * ar)
  const bh = ar >= 1 ? Math.round(MAX / ar) : MAX
  return { width: `${bw}px`, height: `${bh}px` }
}
// 下拉里 / select prefix 用的更小一档(≤16x16)。
function ratioBoxStyleSmall(r: RatioOpt) {
  const MAX = 16
  const ar = r.w / r.h
  const bw = ar >= 1 ? MAX : Math.round(MAX * ar)
  const bh = ar >= 1 ? Math.round(MAX / ar) : MAX
  return { width: `${bw}px`, height: `${bh}px` }
}

// 统一的 prompt 前缀同步工具:
// - 若第一行已经是 "Make the aspect ratio X:Y ,",就把 X:Y 换成新的 ratio
// - 否则把 "Make the aspect ratio {ratio} , " 插到最前面
// - 用户手动删掉这行后不会再自动补回(只有再次切换比例时才重新插入)
const RATIO_PREFIX_RE = /^\s*Make the aspect ratio\s+\S+\s*,\s*/i
function applyRatioPrefix(prompt: string, ratio: string): string {
  const prefix = `Make the aspect ratio ${ratio} , `
  const lines = prompt.split(/\r?\n/)
  if (lines.length > 0 && RATIO_PREFIX_RE.test(lines[0])) {
    lines[0] = lines[0].replace(RATIO_PREFIX_RE, prefix)
    return lines.join('\n')
  }
  return prefix + prompt
}

const t2iPrompt = ref('')
const t2iRatio = ref<string>('1:1')
const t2iSize = computed(() =>
  RATIOS.find((r) => r.ratio === t2iRatio.value)?.size ?? '1024x1024',
)
const currentT2iRatio = computed<RatioOpt>(
  () => RATIOS.find((r) => r.ratio === t2iRatio.value) ?? RATIOS[0],
)
const t2iN = ref(1)
// 本地高清放大档位(空=原图 / '2k' / '4k')。
// 仅在图片代理 URL 首次请求时触发 decode + Catmull-Rom + PNG 编码,
// 进程内 LRU 缓存命中后毫秒级返回。
type UpscaleLevel = '' | '2k' | '4k'
const t2iUpscale = ref<UpscaleLevel>('')

// 切换比例时,实时把 prompt 第一行同步成新的 "Make the aspect ratio X:Y , "
watch(t2iRatio, (nv) => {
  t2iPrompt.value = applyRatioPrefix(t2iPrompt.value, nv)
})
const t2iSending = ref(false)
const t2iResult = ref<PlayImageData[]>([])
const t2iError = ref('')
const t2iAbort = ref<AbortController | null>(null)

const imgExamples = [
  '赛博朋克城市夜景,霓虹雨夜,电影感光影,8k',
  '一只金色胖柴犬穿西装坐在办公桌前,油画质感',
  '极简几何海报,蓝橙配色,主体是一只展翅的鹤',
  '童话风格蘑菇屋,黄昏光线,柔和景深',
]

// 点击示例 prompt 时,自动把当前比例的前缀拼到最前面,保持和 ratio 同步
function useT2iExample(p: string) {
  t2iPrompt.value = applyRatioPrefix(p, t2iRatio.value)
}

async function sendText2Img() {
  const prompt = t2iPrompt.value.trim()
  if (!prompt) {
    ElMessage.warning('请输入描述词 prompt')
    return
  }
  if (!selectedImageModel.value) {
    ElMessage.warning('请选择一个图片模型')
    return
  }
  t2iSending.value = true
  t2iError.value = ''
  t2iResult.value = []
  t2iAbort.value = new AbortController()
  try {
    const resp = await playGenerateImage(
      {
        model: selectedImageModel.value,
        prompt,
        n: t2iN.value,
        size: t2iSize.value,
        upscale: t2iUpscale.value || undefined,
      },
      t2iAbort.value.signal,
    )
    t2iResult.value = resp.data || []
    if (t2iResult.value.length === 0) {
      t2iError.value = '未产出图片,请重试或更换描述'
    } else {
      ElMessage.success(`生成成功,共 ${t2iResult.value.length} 张`)
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    t2iError.value = msg
    ElMessage.error(msg)
  } finally {
    t2iSending.value = false
    t2iAbort.value = null
    userStore.fetchMe().catch(() => {})
  }
}

function stopText2Img() {
  t2iAbort.value?.abort()
}

// 预览 viewer
const previewVisible = ref(false)
const previewList = ref<string[]>([])
const previewIndex = ref(0)
function openPreview(urls: string[], idx: number) {
  previewList.value = urls
  previewIndex.value = idx
  previewVisible.value = true
}
function downloadUrl(url: string) {
  const a = document.createElement('a')
  a.href = url
  a.target = '_blank'
  a.rel = 'noopener'
  a.download = ''
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

// ====================================================
// 图生图(Img2Img)
// ====================================================
interface RefImage {
  name: string
  dataUrl: string
  size: number
}
const refImages = ref<RefImage[]>([])
const i2iPrompt = ref('')
const i2iRatio = ref<string>('1:1')
const i2iSize = computed(() =>
  RATIOS.find((r) => r.ratio === i2iRatio.value)?.size ?? '1024x1024',
)
const currentI2iRatio = computed<RatioOpt>(
  () => RATIOS.find((r) => r.ratio === i2iRatio.value) ?? RATIOS[0],
)
const i2iUpscale = ref<UpscaleLevel>('')
watch(i2iRatio, (nv) => {
  i2iPrompt.value = applyRatioPrefix(i2iPrompt.value, nv)
})
const i2iSending = ref(false)
const i2iResult = ref<PlayImageData[]>([])
const i2iError = ref('')
const i2iAbort = ref<AbortController | null>(null)
const MAX_REF_BYTES = 4 * 1024 * 1024 // 4MB

function handleFilePick(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files
  if (!files) return
  for (const file of Array.from(files)) {
    if (file.size > MAX_REF_BYTES) {
      ElMessage.warning(`${file.name} 超过 4MB 限制`)
      continue
    }
    const reader = new FileReader()
    reader.onload = () => {
      refImages.value.push({
        name: file.name,
        dataUrl: String(reader.result || ''),
        size: file.size,
      })
    }
    reader.readAsDataURL(file)
  }
  input.value = ''
}

function removeRefImage(idx: number) {
  refImages.value.splice(idx, 1)
}

async function sendImg2Img() {
  if (refImages.value.length === 0) {
    ElMessage.warning('请先上传至少一张参考图')
    return
  }
  if (!i2iPrompt.value.trim()) {
    ElMessage.warning('请描述希望的改动')
    return
  }
  if (!selectedImageModel.value) {
    ElMessage.warning('请选择一个图片模型')
    return
  }
  i2iSending.value = true
  i2iError.value = ''
  i2iResult.value = []
  i2iAbort.value = new AbortController()
  try {
    const resp = await playGenerateImage(
      {
        model: selectedImageModel.value,
        prompt: i2iPrompt.value.trim(),
        n: 1,
        size: i2iSize.value,
        reference_images: refImages.value.map((r) => r.dataUrl),
        upscale: i2iUpscale.value || undefined,
      },
      i2iAbort.value.signal,
    )
    i2iResult.value = resp.data || []
    if (i2iResult.value.length > 0) {
      ElMessage.success(`生成成功,共 ${i2iResult.value.length} 张`)
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    i2iError.value = msg
    ElMessage.error(msg)
  } finally {
    i2iSending.value = false
    i2iAbort.value = null
  }
}

// 代码块内的 "复制" 按钮(通过事件委托,避免每次重渲染都重新绑定)
function onMsgClick(e: MouseEvent) {
  const t = e.target as HTMLElement | null
  if (!t) return
  const btn = t.closest('.mdk-copy') as HTMLElement | null
  if (!btn) return
  const pre = btn.parentElement?.querySelector('code')
  if (!pre) return
  copyText(pre.textContent || '')
}

// input 自动聚焦(tab 切换后)
watch(activeTab, (v) => {
  if (v === 'chat') nextTick(() => inputRef.value?.focus?.())
})
</script>

<template>
  <div class="page-container play">
    <!-- ============ Top: tab nav + balance ============ -->
    <header class="play-top">
      <nav class="play-nav" role="tablist" aria-label="在线体验切换">
        <button
          v-if="ENABLE_CHAT_MODEL"
          type="button"
          role="tab"
          :aria-selected="activeTab === 'chat'"
          :class="['nav-tab', { active: activeTab === 'chat' }]"
          @click="activeTab = 'chat'"
        >
          <el-icon><ChatDotRound /></el-icon>
          <span>对话</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'text2img'"
          :class="['nav-tab', { active: activeTab === 'text2img' }]"
          @click="activeTab = 'text2img'"
        >
          <el-icon><Picture /></el-icon>
          <span>文生图</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'img2img'"
          :class="['nav-tab', { active: activeTab === 'img2img' }]"
          @click="activeTab = 'img2img'"
        >
          <el-icon><PictureFilled /></el-icon>
          <span>图生图</span>
        </button>
      </nav>

      <div class="play-balance">
        <span class="play-balance__lbl">余额</span>
        <b class="play-balance__val">¥{{ balance }}</b>
      </div>
    </header>

    <!-- =================================================== -->
    <!--                       Chat panel                     -->
    <!-- =================================================== -->
    <section
      v-if="ENABLE_CHAT_MODEL && activeTab === 'chat'"
      role="tabpanel"
      class="panel chat-panel"
    >
      <!-- Slim toolbar -->
      <div class="chat-bar">
        <el-select
          v-model="selectedChatModel"
          placeholder="选择文字模型"
          size="default"
          class="chat-bar__model"
        >
          <el-option v-for="m in chatModels" :key="m.id" :label="m.slug" :value="m.slug">
            <span class="opt-slug">{{ m.slug }}</span>
          </el-option>
        </el-select>

        <div class="chat-bar__sub" v-if="currentChatDesc">{{ currentChatDesc }}</div>
        <div class="chat-bar__spacer" />

        <el-popover trigger="click" placement="bottom-end" :width="340">
          <template #reference>
            <button type="button" class="bar-btn" aria-label="高级设置">
              <el-icon><Setting /></el-icon>
              <span>高级</span>
            </button>
          </template>
          <div class="settings-pop">
            <div class="settings-row">
              <div class="settings-row__head">
                <label>Temperature</label>
                <span class="settings-row__val">{{ temperature.toFixed(1) }}</span>
              </div>
              <el-slider v-model="temperature" :min="0" :max="2" :step="0.1" size="small" />
              <p class="settings-row__hint">越低越保守、越高越发散。默认 0.7</p>
            </div>
            <div class="settings-row">
              <div class="settings-row__head">
                <label>System Prompt</label>
              </div>
              <el-input
                v-model="systemPrompt"
                type="textarea"
                :rows="5"
                resize="none"
                placeholder="为助手设定人格与风格"
              />
            </div>
          </div>
        </el-popover>

        <el-tooltip content="重试上一个问题" placement="top">
          <button
            type="button"
            class="bar-btn"
            :disabled="chatSending || chatMsgs.length === 0"
            aria-label="重试上一个问题"
            @click="regenerate"
          >
            <el-icon><RefreshRight /></el-icon>
          </button>
        </el-tooltip>

        <button
          v-if="chatMsgs.length > 0"
          type="button"
          class="bar-btn ghost"
          aria-label="清空会话"
          @click="resetChat"
        >
          <el-icon><Delete /></el-icon>
          <span>清空</span>
        </button>
      </div>

      <!-- Chat scroll -->
      <div ref="chatScroll" class="chat-scroll" @click="onMsgClick">
        <div v-if="chatMsgs.length === 0" class="welcome">
          <div class="welcome-glyph" aria-hidden="true">✦</div>
          <h2 class="welcome-title">
            你好<template v-if="user?.nickname">,{{ user.nickname }}</template>
          </h2>
          <p class="welcome-sub">告诉我你想做什么,或者从下面挑一个起步。</p>
        </div>

        <article
          v-for="m in chatMsgs"
          :key="m.id"
          :class="['msg', m.role, { err: m.error }]"
        >
          <div :class="['msg-avatar', m.role]" aria-hidden="true">
            <el-icon v-if="m.role === 'user'"><User /></el-icon>
            <el-icon v-else><MagicStick /></el-icon>
          </div>
          <div class="msg-body">
            <div class="msg-head">
              <span class="who">{{ m.role === 'user' ? '我' : '助手' }}</span>
              <button
                v-if="!m.pending && m.content"
                type="button"
                class="msg-copy"
                @click="copyText(m.content)"
              >
                <el-icon><CopyDocument /></el-icon>
                <span>复制</span>
              </button>
            </div>
            <div class="msg-content">
              <div v-if="m.pending && !m.content" class="typing" aria-label="正在回复">
                <span></span><span></span><span></span>
              </div>
              <div v-else class="md" v-html="renderMarkdown(m.content)" />
            </div>
          </div>
        </article>
      </div>

      <!-- Composer + suggestions -->
      <div class="composer-wrap">
        <div v-if="chatMsgs.length === 0" class="suggest-row">
          <button
            v-for="s in suggestions"
            :key="s.title"
            type="button"
            class="suggest-pill"
            @click="useSuggestion(s)"
          >
            <span class="suggest-pill__ic" aria-hidden="true">{{ s.icon }}</span>
            <span class="suggest-pill__t">{{ s.title }}</span>
            <span class="suggest-pill__s">{{ s.sub }}</span>
          </button>
        </div>

        <div class="composer">
          <el-input
            ref="inputRef"
            v-model="chatInput"
            type="textarea"
            :rows="1"
            :autosize="{ minRows: 1, maxRows: 6 }"
            resize="none"
            placeholder="给助手发消息…  Enter 发送, Shift+Enter 换行"
            @keydown.enter.exact.prevent="sendChat"
          />
          <button
            v-if="chatSending"
            type="button"
            class="send-btn stop"
            aria-label="停止生成"
            @click="stopChat"
          >
            <el-icon><VideoPause /></el-icon>
          </button>
          <button
            v-else
            type="button"
            class="send-btn"
            :disabled="!chatInput.trim() || !selectedChatModel"
            aria-label="发送"
            @click="sendChat"
          >
            <el-icon><Promotion /></el-icon>
          </button>
        </div>
      </div>
    </section>

    <!-- =================================================== -->
    <!--                     Text2Img panel                   -->
    <!-- =================================================== -->
    <section
      v-if="activeTab === 'text2img'"
      role="tabpanel"
      class="panel image-panel"
    >
      <!-- Param toolbar -->
      <div class="param-bar">
        <div class="param-group param-group--grow">
          <span class="param-label">模型</span>
          <el-select v-model="selectedImageModel" placeholder="选择图片模型" class="param-select">
            <el-option v-for="m in imageModels" :key="m.id" :label="m.slug" :value="m.slug">
              <span class="opt-slug">{{ m.slug }}</span>
            </el-option>
          </el-select>
        </div>

        <div class="param-group">
          <span class="param-label">比例</span>
          <el-select
            v-model="t2iRatio"
            class="param-select ratio-select"
            popper-class="ratio-popper"
          >
            <template #prefix>
              <span class="ratio-mini" :style="ratioBoxStyleSmall(currentT2iRatio)" />
            </template>
            <el-option
              v-for="r in RATIOS"
              :key="r.ratio"
              :label="`${r.label} · ${r.ratio}`"
              :value="r.ratio"
            >
              <div class="ratio-opt">
                <span class="ratio-mini" :style="ratioBoxStyleSmall(r)" />
                <span class="ratio-opt__name">{{ r.label }}</span>
                <span class="ratio-opt__val">{{ r.ratio }}</span>
              </div>
            </el-option>
          </el-select>
        </div>

        <div class="param-group">
          <span class="param-label">张数</span>
          <div class="seg-control" role="group">
            <button
              v-for="n in 4"
              :key="n"
              type="button"
              :class="{ active: t2iN === n }"
              :aria-pressed="t2iN === n"
              @click="t2iN = n"
            >{{ n }}</button>
          </div>
        </div>

        <div class="param-group">
          <span class="param-label">
            尺寸
            <el-tooltip placement="top" effect="light">
              <template #content>
                <div class="upscale-tip">
                  上游原生出图为 1024 或 1792 px;选择 2K/4K 会在图片加载时用本地
                  <b>Catmull-Rom 插值</b>放大并以 PNG 输出。<br>
                  <span class="upscale-tip__warn">注意:这是传统算法放大,不是 AI 超分,</span>不会补出新的纹理或毛发,只会让画面更大更平滑。4K 首次加载约 +0.5~1.5s,之后命中缓存。
                </div>
              </template>
              <el-icon class="param-info"><InfoFilled /></el-icon>
            </el-tooltip>
          </span>
          <div class="seg-control" role="group">
            <button type="button" :class="{ active: t2iUpscale === '' }" :aria-pressed="t2iUpscale === ''" @click="t2iUpscale = ''">原图</button>
            <button type="button" :class="{ active: t2iUpscale === '2k' }" :aria-pressed="t2iUpscale === '2k'" @click="t2iUpscale = '2k'">2K</button>
            <button type="button" :class="{ active: t2iUpscale === '4k' }" :aria-pressed="t2iUpscale === '4k'" @click="t2iUpscale = '4k'">4K</button>
          </div>
        </div>
      </div>

      <!-- Hero prompt -->
      <div class="prompt-card">
        <el-input
          v-model="t2iPrompt"
          type="textarea"
          :autosize="{ minRows: 4, maxRows: 12 }"
          resize="none"
          placeholder="描述画面的主体、风格、光线、构图…越具体效果越好"
          class="prompt-card__input"
        />
        <div class="prompt-card__foot">
          <div class="prompt-card__chips">
            <button
              v-for="(p, i) in imgExamples"
              :key="i"
              type="button"
              class="example-chip"
              :title="p"
              @click="useT2iExample(p)"
            >
              {{ p }}
            </button>
          </div>
          <button
            v-if="t2iSending"
            type="button"
            class="generate stop"
            @click="stopText2Img"
          >
            <el-icon><VideoPause /></el-icon>
            <span>停止</span>
          </button>
          <button
            v-else
            type="button"
            class="generate"
            :disabled="!t2iPrompt.trim() || !selectedImageModel"
            @click="sendText2Img"
          >
            <el-icon><MagicStick /></el-icon>
            <span>生成图片</span>
          </button>
        </div>
      </div>

      <!-- Result -->
      <div class="result-area">
        <div v-if="t2iSending" class="stage loading">
          <div class="orb"><el-icon class="spin"><Loading /></el-icon></div>
          <div class="stage-title">正在为你绘制…</div>
          <div class="stage-sub">通常需要 1-2 分钟,请保持页面打开</div>
        </div>
        <div v-else-if="t2iError" class="err-block">
          <el-icon><WarningFilled /></el-icon>
          <span>{{ t2iError }}</span>
        </div>
        <div v-else-if="t2iResult.length === 0" class="stage">
          <div class="stage-glyph" aria-hidden="true">✦</div>
          <div class="stage-title">还没有图片</div>
          <div class="stage-sub">填好上方 prompt 与参数,点击「生成图片」</div>
        </div>
        <div v-else class="result-grid">
          <div
            v-for="(img, idx) in t2iResult"
            :key="idx"
            class="img-cell"
            @click="openPreview(t2iResult.map((x) => x.url), idx)"
          >
            <img :src="img.url" :alt="`result-${idx}`" loading="lazy" />
            <div class="img-actions" @click.stop>
              <button type="button" class="iact" aria-label="放大" @click="openPreview(t2iResult.map((x) => x.url), idx)">
                <el-icon><ZoomIn /></el-icon>
              </button>
              <button type="button" class="iact" aria-label="下载" @click="downloadUrl(img.url)">
                <el-icon><Download /></el-icon>
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- =================================================== -->
    <!--                     Img2Img panel                    -->
    <!-- =================================================== -->
    <section
      v-if="activeTab === 'img2img'"
      role="tabpanel"
      class="panel image-panel"
    >
      <!-- Param toolbar -->
      <div class="param-bar">
        <div class="param-group param-group--grow">
          <span class="param-label">模型</span>
          <el-select v-model="selectedImageModel" placeholder="选择图片模型" class="param-select">
            <el-option v-for="m in imageModels" :key="m.id" :label="m.slug" :value="m.slug">
              <span class="opt-slug">{{ m.slug }}</span>
            </el-option>
          </el-select>
        </div>

        <div class="param-group">
          <span class="param-label">比例</span>
          <el-select
            v-model="i2iRatio"
            class="param-select ratio-select"
            popper-class="ratio-popper"
          >
            <template #prefix>
              <span class="ratio-mini" :style="ratioBoxStyleSmall(currentI2iRatio)" />
            </template>
            <el-option
              v-for="r in RATIOS"
              :key="r.ratio"
              :label="`${r.label} · ${r.ratio}`"
              :value="r.ratio"
            >
              <div class="ratio-opt">
                <span class="ratio-mini" :style="ratioBoxStyleSmall(r)" />
                <span class="ratio-opt__name">{{ r.label }}</span>
                <span class="ratio-opt__val">{{ r.ratio }}</span>
              </div>
            </el-option>
          </el-select>
        </div>

        <div class="param-group">
          <span class="param-label">
            尺寸
            <el-tooltip placement="top" effect="light">
              <template #content>
                <div class="upscale-tip">
                  上游原生出图为 1024 或 1792 px;选择 2K/4K 会在图片加载时用本地
                  <b>Catmull-Rom 插值</b>放大并以 PNG 输出。<br>
                  <span class="upscale-tip__warn">注意:这是传统算法放大,不是 AI 超分,</span>不会补出新的纹理或毛发,只会让画面更大更平滑。4K 首次加载约 +0.5~1.5s,之后命中缓存。
                </div>
              </template>
              <el-icon class="param-info"><InfoFilled /></el-icon>
            </el-tooltip>
          </span>
          <div class="seg-control" role="group">
            <button type="button" :class="{ active: i2iUpscale === '' }" :aria-pressed="i2iUpscale === ''" @click="i2iUpscale = ''">原图</button>
            <button type="button" :class="{ active: i2iUpscale === '2k' }" :aria-pressed="i2iUpscale === '2k'" @click="i2iUpscale = '2k'">2K</button>
            <button type="button" :class="{ active: i2iUpscale === '4k' }" :aria-pressed="i2iUpscale === '4k'" @click="i2iUpscale = '4k'">4K</button>
          </div>
        </div>
      </div>

      <!-- Reference + prompt card -->
      <div class="prompt-card">
        <div class="ref-strip">
          <label class="ref-add" :title="`已上传 ${refImages.length} 张`">
            <el-icon><Plus /></el-icon>
            <span>添加参考图</span>
            <input type="file" accept="image/*" multiple @change="handleFilePick" />
          </label>
          <div v-for="(r, idx) in refImages" :key="idx" class="ref-thumb">
            <img :src="r.dataUrl" :alt="r.name" />
            <button type="button" class="ref-x" aria-label="移除" @click="removeRefImage(idx)">
              <el-icon><Close /></el-icon>
            </button>
            <span class="ref-meta">{{ (r.size / 1024).toFixed(0) }}KB</span>
          </div>
        </div>

        <el-input
          v-model="i2iPrompt"
          type="textarea"
          :autosize="{ minRows: 3, maxRows: 10 }"
          resize="none"
          placeholder="例:保持人物姿态,把背景换成赛博朋克夜景"
          class="prompt-card__input"
        />
        <div class="prompt-card__foot">
          <p class="prompt-card__tip">最多多张参考图,每张 ≤ 4MB</p>
          <button
            type="button"
            class="generate"
            :disabled="refImages.length === 0 || !i2iPrompt.trim() || i2iSending"
            @click="sendImg2Img"
          >
            <el-icon><MagicStick /></el-icon>
            <span>{{ i2iSending ? '生成中…' : '生成' }}</span>
          </button>
        </div>
      </div>

      <!-- Result -->
      <div class="result-area">
        <div v-if="i2iError" class="err-block">
          <el-icon><WarningFilled /></el-icon>
          <span>{{ i2iError }}</span>
        </div>
        <div v-else-if="i2iSending" class="stage loading">
          <div class="orb"><el-icon class="spin"><Loading /></el-icon></div>
          <div class="stage-title">正在生成…</div>
        </div>
        <div v-else-if="i2iResult.length === 0" class="stage">
          <div class="stage-glyph" aria-hidden="true">✦</div>
          <div class="stage-title">还没有结果</div>
          <div class="stage-sub">添加参考图、描述改动,然后点击「生成」</div>
        </div>
        <div v-else class="result-grid">
          <div
            v-for="(img, idx) in i2iResult"
            :key="idx"
            class="img-cell"
            @click="openPreview(i2iResult.map((x) => x.url), idx)"
          >
            <img :src="img.url" :alt="`result-${idx}`" />
            <div class="img-actions" @click.stop>
              <button type="button" class="iact" aria-label="放大" @click="openPreview(i2iResult.map((x) => x.url), idx)">
                <el-icon><ZoomIn /></el-icon>
              </button>
              <button type="button" class="iact" aria-label="下载" @click="downloadUrl(img.url)">
                <el-icon><Download /></el-icon>
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ============ 图片预览(全屏 viewer) ============ -->
    <el-image-viewer
      v-if="previewVisible"
      :url-list="previewList"
      :initial-index="previewIndex"
      @close="previewVisible = false"
      teleported
    />
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

/* ============================================================
   Page shell
   ============================================================ */
.play {
  // 让长内容时主区底部不要紧贴 viewport
  padding-bottom: 32px;
  // 占满主区高度,这样 chat-panel 才能撑到底部
  min-height: calc(100vh - 64px);
  display: flex;
  flex-direction: column;
}

/* ============================================================
   Top: tab nav + balance
   ============================================================ */
.play-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.play-nav {
  display: inline-flex;
  align-items: center;
  padding: 4px;
  gap: 2px;
  background: $gray-100;
  border-radius: $r-pill;
}

.nav-tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  cursor: pointer;
  padding: 8px 18px;
  border-radius: $r-pill;
  font-size: 14px;
  font-weight: 600;
  color: $gray-600;
  letter-spacing: -0.005em;
  transition: color .15s, background .2s, box-shadow .2s, transform .15s;
  font-family: $f-sans;

  .el-icon { font-size: 16px; }

  &:hover:not(.active) {
    color: $n-ink;
  }

  &.active {
    background: $n-paper;
    color: $c-pink-text;
    box-shadow:
      0 1px 1px rgba(15, 15, 30, .04),
      0 4px 12px -4px rgba(15, 15, 30, .12);
  }

  &:focus-visible {
    outline: 2px solid $c-pink;
    outline-offset: 2px;
  }
}

.play-balance {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  padding: 8px 16px;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-pill;
  white-space: nowrap;

  &__lbl {
    font-size: 12px;
    color: $gray-500;
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  &__val {
    font-size: 16px;
    font-weight: 800;
    color: $n-ink;
    font-family: $f-mono;
    letter-spacing: -0.01em;
  }
}

/* ============================================================
   Generic panel
   ============================================================ */
.panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* ============================================================
   Chat
   ============================================================ */
.chat-panel {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  overflow: hidden;
  height: calc(100vh - 64px - 28px - 20px - 60px);
  min-height: 600px;
}

.chat-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: $bw solid $gray-200;
  background: $n-paper;

  &__model {
    width: 240px;
    flex-shrink: 0;
  }
  &__sub {
    font-size: 12px;
    color: $gray-500;
    line-height: 1.4;
    max-width: 280px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  &__spacer { flex: 1; }
}

.bar-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 12px;
  border: $bw solid $gray-200;
  background: $n-paper;
  border-radius: $r-md;
  font-size: 13px;
  font-weight: 600;
  color: $gray-700;
  cursor: pointer;
  transition: border-color .15s, color .15s, background .15s;
  font-family: $f-sans;

  .el-icon { font-size: 15px; }

  &:hover:not(:disabled) {
    border-color: $c-pink;
    color: $c-pink-text;
  }
  &:focus-visible {
    outline: 2px solid $c-pink;
    outline-offset: 2px;
  }
  &:disabled {
    opacity: .5;
    cursor: not-allowed;
  }
  &.ghost {
    color: $gray-500;
    border-color: transparent;
    &:hover:not(:disabled) {
      background: $gray-100;
      color: $c-orange-text;
      border-color: transparent;
    }
  }
}

/* settings popover */
.settings-pop {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.settings-row {
  display: flex;
  flex-direction: column;
  gap: 6px;

  &__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    label {
      font-size: 13px;
      font-weight: 600;
      color: $n-ink;
    }
  }
  &__val {
    font-family: $f-mono;
    font-size: 13px;
    font-weight: 700;
    color: $c-pink-text;
  }
  &__hint {
    margin: 0;
    font-size: 12px;
    color: $gray-500;
    line-height: 1.5;
  }
}

/* chat scroll */
.chat-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 28px 32px 20px;
  scroll-behavior: smooth;
}

.welcome {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 40px 24px;
}
.welcome-glyph {
  font-size: 36px;
  color: $c-pink;
  font-weight: 300;
  letter-spacing: 0;
  margin-bottom: 14px;
  opacity: .85;
}
.welcome-title {
  margin: 0;
  font-size: 28px;
  line-height: 1.2;
  font-weight: 800;
  color: $n-ink;
  letter-spacing: -0.02em;
}
.welcome-sub {
  margin: 8px 0 0;
  color: $gray-500;
  font-size: 15px;
  line-height: 1.5;
}

/* messages */
.msg {
  display: flex;
  gap: 14px;
  padding: 18px 0;
  animation: msgIn 0.25s ease;

  & + & { border-top: 1px solid $gray-100; }

  &.err .msg-content { color: $c-orange-text; }
}
@keyframes msgIn {
  from { opacity: 0; transform: translateY(4px); }
  to   { opacity: 1; transform: translateY(0); }
}

.msg-avatar {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;

  .el-icon { font-size: 16px; }

  &.user {
    background: rgba(255, 214, 0, 0.22);
    color: $c-yellow-text;
  }
  &.assistant {
    background: rgba(168, 85, 247, 0.16);
    color: $c-purple-text;
  }
}

.msg-body {
  flex: 1;
  min-width: 0;
}
.msg-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;

  .who {
    font-size: 13px;
    font-weight: 700;
    color: $n-ink;
    letter-spacing: -0.005em;
  }
}
.msg-copy {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 12px;
  color: $gray-400;
  padding: 2px 6px;
  border-radius: $r-sm;
  font-family: $f-sans;
  opacity: 0;
  transition: opacity .2s, color .2s, background .2s;

  &:hover {
    color: $c-pink-text;
    background: rgba(255, 61, 148, .06);
  }
}
.msg:hover .msg-copy { opacity: 1; }
.msg-content {
  font-size: 15px;
  line-height: 1.78;
  color: $n-ink;
  word-break: break-word;
}

/* markdown */
.md :deep(.mdk-pre) {
  background: $n-ink;
  color: $dark-text-1;
  padding: 14px 16px;
  border-radius: $r-md;
  overflow-x: auto;
  font-family: $f-mono;
  font-size: 13px;
  line-height: 1.65;
  margin: 10px 0;
  position: relative;

  &::before {
    content: attr(data-lang);
    position: absolute;
    top: 8px; right: 12px;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: $dark-text-4;
  }
}
.md :deep(.mdk-ic) {
  background: rgba(255, 61, 148, .08);
  color: $c-pink-text;
  padding: 1px 6px;
  border-radius: $r-sm;
  font-family: $f-mono;
  font-size: 13px;
}
.md :deep(a) {
  color: $c-pink-text;
  text-decoration: underline;
  text-decoration-thickness: 1px;
  text-underline-offset: 2px;
  font-weight: 600;
}
.md :deep(strong) { font-weight: 700; }

/* typing dots */
.typing {
  display: inline-flex;
  gap: 5px;
  padding: 6px 0;
  span {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: $c-pink;
    animation: blink 1.4s infinite ease-in-out both;
  }
  span:nth-child(2) { animation-delay: .2s; }
  span:nth-child(3) { animation-delay: .4s; }
}
@keyframes blink {
  0%, 80%, 100% { opacity: .2; transform: scale(.7); }
  40%           { opacity: 1;  transform: scale(1); }
}

/* ----- composer ----- */
.composer-wrap {
  border-top: $bw solid $gray-200;
  background: linear-gradient(180deg, transparent 0, $n-paper 24px);
  padding: 12px 24px 16px;
}

.suggest-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}
.suggest-pill {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  padding: 10px 14px;
  border: $bw solid $gray-200;
  background: $n-paper;
  border-radius: $r-md;
  cursor: pointer;
  text-align: left;
  font-family: $f-sans;
  transition: border-color .15s, background .15s, transform .15s;

  &__ic { font-size: 16px; line-height: 1; margin-bottom: 4px; }
  &__t { font-size: 13px; font-weight: 700; color: $n-ink; }
  &__s {
    font-size: 12px;
    color: $gray-500;
    line-height: 1.4;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 100%;
  }
  &:hover {
    border-color: $c-pink;
    background: rgba(255, 61, 148, .03);
    transform: translateY(-1px);
  }
}

.composer {
  position: relative;
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 8px 8px 8px 16px;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: 18px;
  transition: border-color .2s, box-shadow .2s;

  &:focus-within {
    border-color: $c-pink;
    box-shadow: 0 0 0 4px rgba(255, 61, 148, .1);
  }

  :deep(.el-textarea__inner) {
    border: none !important;
    box-shadow: none !important;
    padding: 8px 0;
    font-size: 15px;
    line-height: 1.55;
    background: transparent;
    resize: none;
    font-family: $f-sans;
    &:focus { box-shadow: none !important; }
  }
}

.send-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  cursor: pointer;
  background: $c-pink;
  color: #fff;
  transition: background .15s, transform .1s, box-shadow .2s;
  box-shadow: 0 4px 14px -4px rgba(255, 61, 148, .55);

  .el-icon { font-size: 16px; }

  &:hover:not(:disabled) {
    background: $c-pink-hover;
    transform: translateY(-1px);
  }
  &:active:not(:disabled) { transform: translateY(0); }
  &:disabled {
    background: $gray-300;
    box-shadow: none;
    cursor: not-allowed;
  }
  &.stop {
    background: $c-orange;
    box-shadow: 0 4px 14px -4px rgba(255, 107, 53, .55);
    &:hover { background: $c-orange-hover; }
  }
}

/* ============================================================
   Image panels (text2img / img2img)
   ============================================================ */
.image-panel {
  gap: 16px;
}

.param-bar {
  display: flex;
  align-items: stretch;
  flex-wrap: wrap;
  gap: 8px 14px;
  padding: 12px 16px;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
}

.param-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;

  &--grow {
    flex: 1 1 220px;
    min-width: 200px;
    max-width: 320px;
  }
}

.param-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  color: $gray-500;
  letter-spacing: 0.04em;
}

.param-info {
  font-size: 13px;
  color: $gray-400;
  cursor: help;
  &:hover { color: $c-pink; }
}

.param-select { width: 100%; min-width: 120px; }

/* ratio prefix preview */
.ratio-mini {
  display: inline-block;
  background: $c-pink;
  border-radius: 2px;
  flex: 0 0 auto;
}

/* ratio dropdown option row */
.ratio-opt {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;

  &__name {
    font-size: 13px;
    color: $n-ink;
    font-weight: 600;
  }
  &__val {
    margin-left: auto;
    font-family: $f-mono;
    font-size: 12px;
    color: $gray-500;
  }
}
:global(.ratio-popper .el-select-dropdown__item) { padding: 0 12px; }
:global(.ratio-popper .el-select-dropdown__item.is-selected .ratio-opt__val) {
  color: $c-pink-text;
  font-weight: 700;
}

/* segmented control (count, upscale) */
.seg-control {
  display: inline-flex;
  background: $gray-100;
  padding: 3px;
  border-radius: $r-md;
  gap: 2px;

  button {
    border: none;
    background: transparent;
    padding: 6px 12px;
    border-radius: $r-sm;
    font-size: 13px;
    font-weight: 600;
    color: $gray-600;
    cursor: pointer;
    min-width: 36px;
    transition: background .15s, color .15s, box-shadow .2s;
    font-family: $f-sans;

    &:hover:not(.active) { color: $n-ink; }
    &.active {
      background: $n-paper;
      color: $c-pink-text;
      box-shadow:
        0 1px 1px rgba(15, 15, 30, .04),
        0 2px 6px -2px rgba(15, 15, 30, .1);
    }
  }
}

.opt-slug {
  font-family: $f-mono;
  font-size: 13px;
  color: $n-ink;
}

/* prompt card */
.prompt-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 20px;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  transition: border-color .2s, box-shadow .2s;

  &:focus-within {
    border-color: $c-pink;
    box-shadow: 0 0 0 4px rgba(255, 61, 148, .08);
  }

  &__input :deep(.el-textarea__inner) {
    border: none !important;
    box-shadow: none !important;
    padding: 0;
    font-size: 16px;
    line-height: 1.6;
    background: transparent;
    font-family: $f-sans;
    color: $n-ink;
    &::placeholder { color: $gray-400; }
    &:focus { box-shadow: none !important; }
  }

  &__foot {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-top: 12px;
    border-top: 1px dashed $gray-200;
  }
  &__chips {
    flex: 1;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    min-width: 0;
  }
  &__tip {
    flex: 1;
    margin: 0;
    font-size: 12px;
    color: $gray-500;
  }
}

.example-chip {
  border: $bw solid $gray-200;
  background: $n-paper;
  border-radius: $r-pill;
  padding: 5px 12px;
  font-size: 12px;
  color: $gray-600;
  cursor: pointer;
  font-family: $f-sans;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: border-color .15s, color .15s, background .15s;

  &:hover {
    background: rgba(255, 61, 148, .06);
    border-color: $c-pink;
    color: $c-pink-text;
  }
}

.generate {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: $c-pink;
  color: #fff;
  font-weight: 700;
  font-size: 14px;
  padding: 11px 22px;
  border-radius: $r-pill;
  cursor: pointer;
  font-family: $f-sans;
  letter-spacing: -0.005em;
  transition: background .15s, transform .1s, box-shadow .2s;
  box-shadow:
    0 6px 20px -8px rgba(255, 61, 148, .55),
    0 2px 4px -2px rgba(255, 61, 148, .3);

  .el-icon { font-size: 15px; }

  &:hover:not(:disabled) {
    background: $c-pink-hover;
    transform: translateY(-1px);
  }
  &:active:not(:disabled) { transform: translateY(0); }
  &:disabled {
    background: $gray-300;
    box-shadow: none;
    cursor: not-allowed;
  }
  &.stop {
    background: $c-orange;
    box-shadow: 0 6px 20px -8px rgba(255, 107, 53, .55);
    &:hover { background: $c-orange-hover; }
  }
}

/* ref strip in img2img */
.ref-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding-bottom: 14px;
  border-bottom: 1px dashed $gray-200;
}
.ref-add {
  position: relative;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  width: 88px;
  height: 88px;
  border: 2px dashed $gray-300;
  border-radius: $r-md;
  background: $gray-50;
  color: $gray-500;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: border-color .15s, background .15s, color .15s;

  .el-icon { font-size: 22px; color: $c-pink; }

  &:hover {
    border-color: $c-pink;
    background: rgba(255, 61, 148, .04);
    color: $c-pink-text;
  }

  input {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
  }
}
.ref-thumb {
  position: relative;
  width: 88px;
  height: 88px;
  border-radius: $r-md;
  overflow: hidden;
  background: $gray-100;
  border: $bw solid $gray-200;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .ref-x {
    position: absolute;
    top: 4px; right: 4px;
    width: 22px; height: 22px;
    border: none;
    border-radius: 50%;
    background: rgba(0, 0, 0, .65);
    color: #fff;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity .15s;

    .el-icon { font-size: 12px; }
  }
  .ref-meta {
    position: absolute;
    bottom: 0; left: 0; right: 0;
    padding: 3px 6px;
    background: rgba(0, 0, 0, .6);
    color: #fff;
    font-size: 10px;
    font-family: $f-mono;
    text-align: center;
    opacity: 0;
    transition: opacity .15s;
  }
  &:hover {
    .ref-x, .ref-meta { opacity: 1; }
  }
}

/* result area */
.result-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 24px;
  min-height: 360px;
}

.stage {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: $gray-500;
  padding: 48px 24px;

  &-glyph {
    font-size: 36px;
    color: $gray-300;
    font-weight: 300;
    margin-bottom: 14px;
  }
  &-title {
    font-size: 18px;
    font-weight: 700;
    color: $n-ink;
    letter-spacing: -0.01em;
  }
  &-sub {
    font-size: 14px;
    margin-top: 6px;
    line-height: 1.5;
    max-width: 360px;
  }
  &.loading { gap: 14px; }

  .orb {
    width: 76px;
    height: 76px;
    border-radius: 50%;
    background: rgba(255, 61, 148, .12);
    display: flex;
    align-items: center;
    justify-content: center;
    animation: orbPulse 1.8s ease-in-out infinite;
  }
}
@keyframes orbPulse {
  0%, 100% { transform: scale(1);    box-shadow: 0 0 0 0   rgba(255, 61, 148, .25); }
  50%      { transform: scale(1.07); box-shadow: 0 0 0 16px rgba(255, 61, 148, 0); }
}
.spin {
  font-size: 32px;
  color: $c-pink;
  animation: spin 1s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.err-block {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 14px 16px;
  background: rgba(255, 107, 53, .08);
  color: $c-orange-text;
  border: $bw solid rgba(255, 107, 53, .25);
  border-radius: $r-md;
  font-size: 14px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

/* result grid */
.result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 14px;
}
.img-cell {
  position: relative;
  aspect-ratio: 1;
  border-radius: $r-md;
  overflow: hidden;
  cursor: zoom-in;
  background: $gray-100;
  box-shadow: $sh-1;
  transition: transform .2s, box-shadow .25s;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    transition: transform .4s;
  }
  &:hover {
    transform: translateY(-2px);
    box-shadow: $sh-2;
    img { transform: scale(1.03); }
    .img-actions { opacity: 1; }
  }
}
.img-actions {
  position: absolute;
  top: 10px; right: 10px;
  display: flex;
  gap: 6px;
  opacity: 0;
  transition: opacity .2s;

  .iact {
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 50%;
    background: rgba(0, 0, 0, .55);
    color: #fff;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    backdrop-filter: blur(6px);
    transition: background .15s;

    .el-icon { font-size: 14px; }

    &:hover { background: $c-pink; }
  }
}

/* ============================================================
   Responsive
   ============================================================ */
@media (max-width: 900px) {
  .play-top {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }
  .play-nav { justify-content: center; }
  .play-balance { align-self: flex-end; }

  .chat-bar {
    flex-wrap: wrap;
    &__model { width: 100%; }
    &__sub { width: 100%; max-width: none; white-space: normal; }
    &__spacer { display: none; }
  }

  .param-bar { gap: 10px; }
  .param-group--grow { flex-basis: 100%; max-width: none; }

  .prompt-card__foot {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }
  .generate {
    width: 100%;
    justify-content: center;
  }

  .chat-panel {
    height: calc(100vh - 64px - 28px - 20px - 100px);
  }
}
</style>
