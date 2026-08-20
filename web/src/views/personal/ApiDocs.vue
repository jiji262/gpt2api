<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  listMyModels,
  listMyUsageLogs,
  listMyImageTasks,
  getMyUsageStats,
  type SimpleModel,
  type UsageItem,
  type ImageTask,
  type MyStatsResp,
} from '@/api/me'
import { formatCredit, formatDateTime, formatErrorCode } from '@/utils/format'
import { ENABLE_CHAT_MODEL } from '@/config/feature'

const activeTab = ref<'chat' | 'image'>(ENABLE_CHAT_MODEL ? 'chat' : 'image')

const models = ref<SimpleModel[]>([])
const chatModels = computed(() => models.value.filter((m) => m.type === 'chat'))
const imageModels = computed(() => models.value.filter((m) => m.type === 'image'))

const selectedChatModel = ref<string>('')
const selectedImageModel = ref<string>('')

// 原点:浏览器当前地址,用于 SDK 示例的 base_url
const origin = computed(() => window.location.origin)

// ---------- 当前用户汇总 ----------
const stats = ref<MyStatsResp | null>(null)
const statsLoading = ref(false)

async function loadStats() {
  statsLoading.value = true
  try {
    stats.value = await getMyUsageStats({ days: 14, top_n: 5 })
  } finally {
    statsLoading.value = false
  }
}

// ---------- 文字历史(chat) ----------
const chatLogs = ref<UsageItem[]>([])
const chatPage = ref({ limit: 20, offset: 0, total: 0 })
const chatLoading = ref(false)

async function loadChatLogs() {
  chatLoading.value = true
  try {
    const data = await listMyUsageLogs({
      type: 'chat',
      limit: chatPage.value.limit,
      offset: chatPage.value.offset,
    })
    chatLogs.value = data.items
    chatPage.value.total = data.total
  } finally {
    chatLoading.value = false
  }
}

function chatPageChange(p: number) {
  chatPage.value.offset = (p - 1) * chatPage.value.limit
  loadChatLogs()
}

// ---------- 图片历史 ----------
const imageTasks = ref<ImageTask[]>([])
const imagePage = ref({ limit: 12, offset: 0 })
const imageLoading = ref(false)
const hasMoreImage = ref(false)

async function loadImageTasks(reset = true) {
  imageLoading.value = true
  try {
    if (reset) {
      imagePage.value.offset = 0
      imageTasks.value = []
    }
    const data = await listMyImageTasks({
      limit: imagePage.value.limit,
      offset: imagePage.value.offset,
    })
    if (reset) imageTasks.value = data.items
    else imageTasks.value.push(...data.items)
    hasMoreImage.value = data.items.length >= imagePage.value.limit
  } finally {
    imageLoading.value = false
  }
}

function imageLoadMore() {
  imagePage.value.offset += imagePage.value.limit
  loadImageTasks(false)
}

// ---------- SDK 代码示例 ----------
const chatCurl = computed(() => {
  const model = selectedChatModel.value || 'gpt-5'
  return `curl ${origin.value}/v1/chat/completions \\
  -H "Authorization: Bearer \${YOUR_API_KEY}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${model}",
    "stream": true,
    "messages": [
      {"role": "user", "content": "你好,介绍一下你自己"}
    ]
  }'`
})

const chatPython = computed(() => {
  const model = selectedChatModel.value || 'gpt-5'
  return `from openai import OpenAI

client = OpenAI(
    base_url="${origin.value}/v1",
    api_key="\${YOUR_API_KEY}",
)

resp = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": "你好"}],
    stream=True,
)
for chunk in resp:
    print(chunk.choices[0].delta.content or "", end="")`
})

const imageCurl = computed(() => {
  const model = selectedImageModel.value || 'gpt-image-2'
  return `curl ${origin.value}/v1/images/generations \\
  -H "Authorization: Bearer \${YOUR_API_KEY}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${model}",
    "prompt": "A cute orange cat playing with yarn, studio ghibli style",
    "n": 1,
    "size": "1024x1024"
  }'`
})

const imagePython = computed(() => {
  const model = selectedImageModel.value || 'gpt-image-2'
  return `from openai import OpenAI

client = OpenAI(
    base_url="${origin.value}/v1",
    api_key="\${YOUR_API_KEY}",
)

resp = client.images.generate(
    model="${model}",
    prompt="A cute orange cat playing with yarn",
    n=1,
    size="1024x1024",
)
print(resp.data[0].url)`
})

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败,请手动选择文本')
  }
}

// ---------- 状态标签 ----------
function statusTag(s: string): 'success' | 'warning' | 'danger' | 'info' {
  if (s === 'success') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'running' || s === 'dispatched' || s === 'queued') return 'warning'
  return 'info'
}

// ---------- 初始化 ----------
onMounted(async () => {
  try {
    const m = await listMyModels()
    models.value = ENABLE_CHAT_MODEL
      ? m.items
      : m.items.filter((x) => x.type !== 'chat')
    const firstChat = m.items.find((x) => x.type === 'chat')
    const firstImage = m.items.find((x) => x.type === 'image')
    if (firstChat) selectedChatModel.value = firstChat.slug
    if (firstImage) selectedImageModel.value = firstImage.slug
  } catch {
    // 忽略
  }
  loadStats()
  if (ENABLE_CHAT_MODEL) loadChatLogs()
  loadImageTasks()
})
</script>

<template>
  <div class="page-container docs-page">
    <!-- ============ Header ============ -->
    <PageHeader crumb="个人中心" title="接口文档" accent-word="文档" accent="pink">
      <template #extra>
        <div class="stats-chips" v-loading="statsLoading">
          <span class="stat-chip">
            <span class="stat-lbl">14 天请求</span>
            <b>{{ stats?.overall.requests ?? 0 }}</b>
          </span>
          <span v-if="ENABLE_CHAT_MODEL" class="stat-chip">
            <span class="stat-lbl">Token in/out</span>
            <b>{{ stats?.overall.input_tokens ?? 0 }} / {{ stats?.overall.output_tokens ?? 0 }}</b>
          </span>
          <span class="stat-chip">
            <span class="stat-lbl">图片张数</span>
            <b>{{ stats?.overall.image_images ?? 0 }}</b>
          </span>
          <span class="stat-chip accent">
            <span class="stat-lbl">消耗积分</span>
            <b>{{ formatCredit(stats?.overall.credit_cost) }}</b>
          </span>
        </div>
      </template>
    </PageHeader>

    <!-- ============ Tabs + Content ============ -->
    <el-tabs v-model="activeTab" class="docs-tabs">
      <!-- ================== 文字对话 ================== -->
      <el-tab-pane v-if="ENABLE_CHAT_MODEL" label="对话生成(文字模型)" name="chat">
        <div class="docs-body">
          <div class="row">
            <div class="label">文字模型</div>
            <el-select v-model="selectedChatModel" placeholder="选择模型" style="width: 320px">
              <el-option
                v-for="m in chatModels"
                :key="m.id"
                :label="`${m.slug}${m.description ? ' · ' + m.description : ''}`"
                :value="m.slug"
              />
            </el-select>
            <router-link to="/personal/keys">
              <el-button text type="primary">没有 Key?去「API Keys」创建</el-button>
            </router-link>
          </div>

          <el-tabs type="border-card" class="code-tabs">
            <el-tab-pane label="curl">
              <pre class="code"><code>{{ chatCurl }}</code></pre>
              <el-button size="small" @click="copy(chatCurl)">复制 curl</el-button>
            </el-tab-pane>
            <el-tab-pane label="Python (OpenAI SDK)">
              <pre class="code"><code>{{ chatPython }}</code></pre>
              <el-button size="small" @click="copy(chatPython)">复制 Python</el-button>
            </el-tab-pane>
          </el-tabs>
        </div>

        <div class="docs-body">
          <div class="flex-between" style="margin-bottom: 10px">
            <h3 class="section-title">文字调用历史</h3>
            <el-button size="small" @click="loadChatLogs">刷新</el-button>
          </div>
          <el-table v-loading="chatLoading" :data="chatLogs" stripe size="small">
            <el-table-column prop="created_at" label="时间" min-width="160">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column prop="model_slug" label="模型" min-width="140" />
            <el-table-column label="Token (in / out / cache)" min-width="170">
              <template #default="{ row }">
                {{ row.input_tokens }} / {{ row.output_tokens }}
                <span v-if="row.cache_read_tokens" class="mute">/ {{ row.cache_read_tokens }}</span>
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="90">
              <template #default="{ row }">{{ row.duration_ms }} ms</template>
            </el-table-column>
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="statusTag(row.status)" size="small">{{ row.status }}</el-tag>
                <el-tooltip v-if="row.error_code" :content="formatErrorCode(row.error_code) + '(' + row.error_code + ')'">
                  <el-icon style="margin-left:4px"><InfoFilled /></el-icon>
                </el-tooltip>
              </template>
            </el-table-column>
            <el-table-column label="扣费(积分)" width="110">
              <template #default="{ row }">{{ formatCredit(row.credit_cost) }}</template>
            </el-table-column>
          </el-table>
          <div class="pager">
            <el-pagination
              layout="prev, pager, next, total"
              :total="chatPage.total"
              :page-size="chatPage.limit"
              :current-page="Math.floor(chatPage.offset / chatPage.limit) + 1"
              @current-change="chatPageChange"
            />
          </div>
        </div>
      </el-tab-pane>

      <!-- ================== 图片生成 ================== -->
      <el-tab-pane label="图片生成(图片模型)" name="image">
        <div class="docs-body">
          <div class="row">
            <div class="label">图片模型</div>
            <el-select v-model="selectedImageModel" placeholder="选择模型" style="width: 320px">
              <el-option
                v-for="m in imageModels"
                :key="m.id"
                :label="`${m.slug}${m.description ? ' · ' + m.description : ''}`"
                :value="m.slug"
              />
            </el-select>
          </div>

          <el-tabs type="border-card" class="code-tabs">
            <el-tab-pane label="curl">
              <pre class="code"><code>{{ imageCurl }}</code></pre>
              <el-button size="small" @click="copy(imageCurl)">复制 curl</el-button>
            </el-tab-pane>
            <el-tab-pane label="Python (OpenAI SDK)">
              <pre class="code"><code>{{ imagePython }}</code></pre>
              <el-button size="small" @click="copy(imagePython)">复制 Python</el-button>
            </el-tab-pane>
          </el-tabs>
        </div>

        <div class="docs-body">
          <div class="flex-between" style="margin-bottom: 10px">
            <h3 class="section-title">图片任务历史</h3>
            <el-button size="small" @click="loadImageTasks(true)">刷新</el-button>
          </div>
          <div v-loading="imageLoading">
            <div v-if="imageTasks.length === 0 && !imageLoading" class="empty">
              暂无图片任务,复制上方代码调用一次即可生成记录。
            </div>
            <div class="grid">
              <el-card
                v-for="t in imageTasks"
                :key="t.id"
                shadow="hover"
                class="img-card"
              >
                <div class="thumb">
                  <img v-if="t.image_urls?.[0]" :src="t.image_urls[0]" :alt="t.prompt" />
                  <div v-else class="thumb-ph">
                    <el-icon :size="32"><PictureRounded /></el-icon>
                    <div class="s">{{ t.status }}</div>
                  </div>
                </div>
                <div class="meta">
                  <div class="title" :title="t.prompt">{{ t.prompt || '(无 prompt)' }}</div>
                  <div class="sub">
                    <el-tag size="small" :type="statusTag(t.status)">{{ t.status }}</el-tag>
                    <span>{{ t.size }}</span>
                    <span class="mute">n={{ t.n }}</span>
                  </div>
                  <div class="foot">
                    <span class="mute">{{ formatDateTime(t.created_at) }}</span>
                    <span class="credit">{{ formatCredit(t.credit_cost) }} 积分</span>
                  </div>
                  <div v-if="t.error" class="err">{{ t.error }}</div>
                </div>
              </el-card>
            </div>
            <div v-if="hasMoreImage" class="pager">
              <el-button @click="imageLoadMore">加载更多</el-button>
            </div>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.docs-page { padding: 28px 32px 60px; }

// ---- stats chips in PageHeader extra ----
.stats-chips {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.stat-chip {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 6px 14px;
  background: $gray-100;
  border-radius: $r-pill;
  font-size: var(--fs-xs);

  .stat-lbl { color: $gray-500; font-weight: 500; }
  b { color: $n-ink; font-weight: 800; font-size: var(--fs-sm); }

  &.accent {
    background: rgba(255, 61, 148, 0.08);
    b { color: $c-pink; }
  }
}

// ---- docs tabs ----
:deep(.docs-tabs) {
  .el-tabs__item {
    font-weight: 700;
    font-size: 14px;
    color: $gray-600;
    &.is-active { color: $c-pink; }
  }
  .el-tabs__active-bar { background: $c-pink; height: 3px; }
  .el-tabs__nav-wrap::after { background-color: $gray-200; }
  .el-tabs__header { margin-bottom: 16px; }
}

// ---- docs body card ----
.docs-body {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 24px 28px;
  margin-bottom: 16px;
  line-height: 1.7;
}

.section-title { margin: 0; font-size: 16px; font-weight: 800; color: $n-ink; }
.flex-between { display: flex; justify-content: space-between; align-items: center; }

.row {
  display: flex; gap: 12px; align-items: center; flex-wrap: wrap; margin-bottom: 16px;
  .label { font-weight: 700; font-size: var(--fs-sm); color: $gray-600; min-width: 68px; }
}

// ---- code blocks ----
.code-tabs {
  :deep(.el-tabs__content) { padding: 12px; }
}
.code {
  background: $n-ink;
  color: $dark-text-1;
  border-radius: $r-lg;
  padding: 16px 18px;
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.7;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: $f-mono;
  border: $bw solid $dark-border-strong;
}

// ---- misc ----
.mute { color: $gray-500; }
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
.empty { padding: 24px 0; color: $gray-500; text-align: center; }

// ---- image task grid ----
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}
.img-card {
  border: $bw solid $gray-200 !important;
  border-radius: $r-lg !important;
  overflow: hidden;
  :deep(.el-card__body) { padding: 0; }
  .thumb {
    height: 180px;
    display: flex; align-items: center; justify-content: center;
    background: $gray-100;
    img { max-width: 100%; max-height: 100%; object-fit: contain; }
  }
  .thumb-ph { text-align: center; color: $gray-500; .s { font-size: 12px; } }
  .meta { padding: 10px 12px; }
  .title {
    font-size: 13px; font-weight: 700; color: $n-ink; margin-bottom: 6px;
    overflow: hidden; white-space: nowrap; text-overflow: ellipsis;
  }
  .sub { display: flex; gap: 6px; font-size: 12px; align-items: center; color: $gray-600; }
  .foot {
    display: flex; justify-content: space-between; margin-top: 6px; font-size: 12px;
    .credit { color: $c-orange-text; font-weight: 700; }
  }
  .err {
    color: $c-orange-text; font-size: 12px; margin-top: 6px;
    background: rgba(255, 107, 53, 0.08); padding: 4px 6px; border-radius: $r-sm;
    white-space: pre-wrap; word-break: break-word;
  }
}

@media (max-width: 640px) {
  .docs-page { padding: 16px 16px 40px; }
  .stats-chips { gap: 8px; }
}
</style>
