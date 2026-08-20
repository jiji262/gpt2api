<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import * as meApi from '@/api/me'
import { formatCredit, formatDateTime, formatErrorCode } from '@/utils/format'
import { ENABLE_CHAT_MODEL } from '@/config/feature'

// ==================== 概览 + 每日 + 模型 TOP ====================

const statsLoading = ref(false)
const stats = ref<meApi.MyStatsResp | null>(null)

const statsFilter = reactive({
  days: 14,
  type: '' as '' | 'chat' | 'image',
})

async function loadStats() {
  statsLoading.value = true
  try {
    stats.value = await meApi.getMyUsageStats({
      days: statsFilter.days,
      top_n: 8,
      type: statsFilter.type || undefined,
    })
  } finally {
    statsLoading.value = false
  }
}

const overall = computed(() => stats.value?.overall)
const daily = computed(() => stats.value?.daily || [])
const byModel = computed(() => stats.value?.by_model || [])

// ============ 每日请求图表(SVG)============
const chartWrap = ref<HTMLElement | null>(null)
const chartW = ref(720)
const chartH = 220
const padT = 16
const padB = 36
const padL = 40
const padR = 16
const hoverIdx = ref(-1)

let ro: ResizeObserver | null = null
onMounted(() => {
  if (chartWrap.value && typeof ResizeObserver !== 'undefined') {
    ro = new ResizeObserver((entries) => {
      for (const e of entries) {
        const w = e.contentRect.width
        if (w > 0) chartW.value = Math.floor(w)
      }
    })
    ro.observe(chartWrap.value)
  }
})
onBeforeUnmount(() => { ro?.disconnect() })

function niceMax(v: number): number {
  if (v <= 1) return 1
  const exp = Math.pow(10, Math.floor(Math.log10(v)))
  const n = v / exp
  let m = 10
  if (n <= 1) m = 1
  else if (n <= 2) m = 2
  else if (n <= 5) m = 5
  return m * exp
}

const maxRaw = computed(() => daily.value.reduce((x, r) => Math.max(x, r.requests), 0))
const yMax = computed(() => niceMax(maxRaw.value || 1))
const yTicks = computed(() => {
  const m = yMax.value
  const step = m / 4
  return [0, step, step * 2, step * 3, m].map((v) => Math.round(v))
})

const cellW = computed(() => {
  const n = Math.max(daily.value.length, 1)
  return (chartW.value - padL - padR) / n
})
function xCenter(i: number) { return padL + cellW.value * (i + 0.5) }
function barH(v: number) {
  const innerH = chartH - padT - padB
  return (v / yMax.value) * innerH
}
function barY(v: number) { return chartH - padB - barH(v) }

const barW = computed(() => Math.min(22, Math.max(6, cellW.value * 0.55)))

const labelStep = computed(() => (daily.value.length > 18 ? 2 : 1))
function shouldShowLabel(i: number) {
  const n = daily.value.length
  if (i === n - 1) return true
  return i % labelStep.value === 0
}

const tipX = computed(() => (hoverIdx.value >= 0 ? xCenter(hoverIdx.value) : 0))
const tipY = computed(() => (hoverIdx.value >= 0 ? barY(daily.value[hoverIdx.value]?.requests || 0) : 0))
const tipSide = computed<'left' | 'right'>(() => (tipX.value > chartW.value / 2 ? 'left' : 'right'))

function onCellEnter(i: number) { hoverIdx.value = i }
function onChartLeave() { hoverIdx.value = -1 }

function successRate(s?: meApi.UsageOverall) {
  if (!s || s.requests === 0) return '—'
  return `${(((s.requests - s.failures) / s.requests) * 100).toFixed(2)}%`
}

// ==================== 明细 Tab(请求日志 / 积分流水) ====================

const activeTab = ref<'logs' | 'credits'>('logs')

// ---------- 请求日志 ----------
const logLoading = ref(false)
const logItems = ref<meApi.UsageItem[]>([])
const logTotal = ref(0)
const logFilter = reactive({
  type: '' as '' | 'chat' | 'image',
  status: '' as '' | 'success' | 'failed',
  limit: 20,
  offset: 0,
})

async function loadLogs() {
  logLoading.value = true
  try {
    const d = await meApi.listMyUsageLogs({
      type: logFilter.type || undefined,
      status: logFilter.status || undefined,
      limit: logFilter.limit,
      offset: logFilter.offset,
    })
    logItems.value = d.items
    logTotal.value = d.total
  } finally {
    logLoading.value = false
  }
}

const logPage = computed<number>({
  get: () => Math.floor(logFilter.offset / logFilter.limit) + 1,
  set: (v) => {
    logFilter.offset = (v - 1) * logFilter.limit
    loadLogs()
  },
})

function refreshLogs() {
  logFilter.offset = 0
  loadLogs()
}

const statusMap: Record<string, { tag: 'success' | 'danger' | 'warning' | 'info'; label: string }> = {
  success: { tag: 'success', label: '成功' },
  failed: { tag: 'danger', label: '失败' },
  partial: { tag: 'warning', label: '部分' },
}
function statusTag(s: string) {
  return statusMap[s]?.tag || 'info'
}
function statusLabel(s: string) {
  return statusMap[s]?.label || s || '-'
}

// ---------- 积分流水 ----------
const creditLoading = ref(false)
const creditItems = ref<meApi.MyCreditLog[]>([])
const creditTotal = ref(0)
const creditFilter = reactive({ limit: 20, offset: 0 })

async function loadCredits() {
  creditLoading.value = true
  try {
    const d = await meApi.listMyCreditLogs({
      limit: creditFilter.limit,
      offset: creditFilter.offset,
    })
    creditItems.value = d.items
    creditTotal.value = d.total
  } finally {
    creditLoading.value = false
  }
}

const creditPage = computed<number>({
  get: () => Math.floor(creditFilter.offset / creditFilter.limit) + 1,
  set: (v) => {
    creditFilter.offset = (v - 1) * creditFilter.limit
    loadCredits()
  },
})

const typeLabel: Record<string, string> = {
  recharge: '充值',
  consume: '消费',
  refund: '退款',
  adjust: '调账',
  bonus: '赠送',
  freeze: '冻结',
  unfreeze: '解冻',
}

function onTabChange(v: string | number) {
  if (v === 'credits' && creditItems.value.length === 0) loadCredits()
}

onMounted(() => {
  loadStats()
  loadLogs()
})
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="个人中心" title="使用记录" accent-word="记录" accent="purple">
      <template #extra>
        <el-select v-model="statsFilter.days" style="width:110px" @change="loadStats">
          <el-option :value="7"  label="近 7 天" />
          <el-option :value="14" label="近 14 天" />
          <el-option :value="30" label="近 30 天" />
          <el-option :value="60" label="近 60 天" />
        </el-select>
        <el-select v-model="statsFilter.type" style="width:120px" clearable placeholder="类型"
                   @change="loadStats">
          <el-option label="全部" value="" />
          <el-option v-if="ENABLE_CHAT_MODEL" label="对话" value="chat" />
          <el-option label="生图" value="image" />
        </el-select>
        <NeonButton variant="outline" @click="loadStats">
          <el-icon><Refresh /></el-icon> 刷新
        </NeonButton>
      </template>
    </PageHeader>

    <!-- KPI 卡 -->
    <div class="kpi-grid" v-loading="statsLoading">
      <KpiCard accent="purple" label="请求数" :value="overall?.requests ?? 0"
               :change="`失败 ${overall?.failures ?? 0}`" change-dir="flat" />
      <KpiCard accent="pink" label="累计扣费" :value="formatCredit(overall?.credit_cost)" />
      <KpiCard v-if="ENABLE_CHAT_MODEL" accent="orange" label="对话请求"
               :value="overall?.chat_requests ?? 0"
               :change="`成功率 ${successRate(overall)}`" change-dir="flat" />
      <KpiCard v-else accent="orange" label="生图张数"
               :value="overall?.image_images ?? 0"
               :change="`成功率 ${successRate(overall)}`" change-dir="flat" />
    </div>

    <!-- 每日柱状图 -->
    <div class="panel">
      <div class="panel__head">
        <div>
          <h4>每日请求趋势</h4>
          <div class="panel__sub">近 {{ statsFilter.days }} 天 · 悬停柱条查看详情</div>
        </div>
        <div class="legend">
          <span class="legend-dot legend-dot--purple"></span>
          <span class="legend-label">请求总数</span>
          <span class="legend-dot legend-dot--danger"></span>
          <span class="legend-label">失败</span>
        </div>
      </div>

      <div ref="chartWrap" class="chart-wrap" v-loading="statsLoading">
        <el-empty
          v-if="!statsLoading && daily.length === 0"
          description="暂无数据"
          :image-size="80"
          style="padding:24px 0"
        />
        <svg
          v-else
          class="chart-svg"
          :viewBox="`0 0 ${chartW} ${chartH}`"
          :style="{ height: chartH + 'px' }"
          @mouseleave="onChartLeave"
        >
          <!-- y 轴网格 + 刻度 -->
          <g class="chart-axis">
            <line
              v-for="(t, ti) in yTicks" :key="'ty' + ti"
              :x1="padL" :x2="chartW - padR"
              :y1="padT + (chartH - padT - padB) * (1 - t / yMax)"
              :y2="padT + (chartH - padT - padB) * (1 - t / yMax)"
              class="grid-line"
              :class="{ 'grid-zero': ti === 0 }"
            />
            <text
              v-for="(t, ti) in yTicks" :key="'tl' + ti"
              :x="padL - 8"
              :y="padT + (chartH - padT - padB) * (1 - t / yMax) + 4"
              text-anchor="end"
              class="axis-tick"
            >{{ t }}</text>
          </g>

          <!-- 柱条 + 交互区 -->
          <g
            v-for="(p, i) in daily" :key="p.day"
            class="bar-group"
            :class="{ hover: hoverIdx === i }"
            @mouseenter="onCellEnter(i)"
          >
            <rect
              :x="padL + cellW * i" :y="padT"
              :width="cellW"
              :height="chartH - padT - padB"
              class="bar-hit"
            />
            <!-- 请求柱 (紫色) -->
            <rect
              :x="xCenter(i) - barW / 2"
              :y="barY(p.requests)"
              :width="barW"
              :height="barH(p.requests)"
              rx="3"
              fill="#A855F7"
              class="bar-rect"
            />
            <!-- 失败覆盖 (橙色) -->
            <rect
              v-if="p.failures"
              :x="xCenter(i) - barW / 2"
              :y="barY(p.failures)"
              :width="barW"
              :height="barH(p.failures)"
              rx="3"
              fill="#FF6B35"
              class="bar-rect bar-rect-fail"
            />
            <!-- x 轴日期 -->
            <text
              v-if="shouldShowLabel(i)"
              :x="xCenter(i)"
              :y="chartH - padB + 16"
              text-anchor="middle"
              class="axis-date"
            >{{ p.day.slice(5) }}</text>
          </g>

          <!-- 悬停指示线 -->
          <line
            v-if="hoverIdx >= 0"
            :x1="tipX" :x2="tipX"
            :y1="padT" :y2="chartH - padB"
            class="hover-guide"
          />

          <!-- 悬停 tooltip -->
          <g v-if="hoverIdx >= 0" class="tip-group">
            <foreignObject
              :x="tipSide === 'right' ? tipX + 10 : tipX - 170"
              :y="Math.max(padT, tipY - 58)"
              width="160" height="70"
            >
              <div class="chart-tip">
                <div class="tip-day">{{ daily[hoverIdx]?.day }}</div>
                <div class="tip-row">
                  <span class="tip-dot tip-dot--purple"></span>请求
                  <b>{{ daily[hoverIdx]?.requests || 0 }}</b>
                </div>
                <div class="tip-row">
                  <span class="tip-dot tip-dot--danger"></span>失败
                  <b>{{ daily[hoverIdx]?.failures || 0 }}</b>
                </div>
              </div>
            </foreignObject>
          </g>
        </svg>
      </div>
    </div>

    <!-- 模型 TOP -->
    <div class="panel" style="margin-top:16px">
      <h4>模型 TOP</h4>
      <el-table :data="byModel" stripe size="small" v-loading="statsLoading" empty-text="暂无数据">
        <el-table-column label="模型" min-width="180">
          <template #default="{ row }">
            <StatusTag variant="cyan">{{ row.model_slug || `#${row.model_id}` }}</StatusTag>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="80">
          <template #default="{ row }">
            <StatusTag :variant="row.type === 'image' ? 'yellow' : 'purple'">
              {{ row.type || '-' }}
            </StatusTag>
          </template>
        </el-table-column>
        <el-table-column prop="requests" label="请求数" width="100" />
        <el-table-column prop="failures" label="失败" width="80" />
        <el-table-column prop="input_tokens" label="输入 tok" width="110" />
        <el-table-column prop="output_tokens" label="输出 tok" width="110" />
        <el-table-column prop="image_count" label="图数" width="80" />
        <el-table-column label="扣费" width="120">
          <template #default="{ row }">{{ formatCredit(row.credit_cost) }}</template>
        </el-table-column>
        <el-table-column prop="avg_dur_ms" label="平均耗时(ms)" width="130" />
      </el-table>
    </div>

    <!-- 明细 Tabs -->
    <div class="panel" style="margin-top:16px">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <!-- ----- 请求日志 ----- -->
        <el-tab-pane name="logs" label="请求日志">
          <div class="tab-toolbar">
            <div class="flex-wrap-gap">
              <el-select v-model="logFilter.type" style="width:120px" clearable placeholder="类型"
                         @change="refreshLogs">
                <el-option label="全部" value="" />
                <el-option v-if="ENABLE_CHAT_MODEL" label="对话" value="chat" />
                <el-option label="生图" value="image" />
              </el-select>
              <el-select v-model="logFilter.status" style="width:120px" clearable placeholder="状态"
                         @change="refreshLogs">
                <el-option label="全部" value="" />
                <el-option label="成功" value="success" />
                <el-option label="失败" value="failed" />
              </el-select>
            </div>
            <NeonButton variant="ghost" @click="refreshLogs">
              <el-icon><Refresh /></el-icon> 刷新
            </NeonButton>
          </div>

          <el-table :data="logItems" stripe size="small" v-loading="logLoading" empty-text="暂无记录">
            <el-table-column prop="created_at" label="时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="模型" min-width="150">
              <template #default="{ row }">
                <StatusTag variant="cyan">{{ row.model_slug || `#${row.model_id}` }}</StatusTag>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="80">
              <template #default="{ row }">
                <StatusTag :variant="row.type === 'image' ? 'yellow' : 'purple'">
                  {{ row.type || '-' }}
                </StatusTag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <StatusTag :variant="statusTag(row.status)" dot>
                  {{ statusLabel(row.status) }}
                </StatusTag>
              </template>
            </el-table-column>
            <el-table-column prop="input_tokens" label="输入 tok" width="95" />
            <el-table-column prop="output_tokens" label="输出 tok" width="95" />
            <el-table-column label="图数" width="70">
              <template #default="{ row }">{{ row.image_count || 0 }}</template>
            </el-table-column>
            <el-table-column label="扣费" width="110">
              <template #default="{ row }">
                <span class="cost">{{ formatCredit(row.credit_cost) }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="duration_ms" label="耗时(ms)" width="100" />
            <el-table-column label="Request ID" min-width="180">
              <template #default="{ row }">
                <span class="req-id" :title="row.request_id">{{ row.request_id || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="错误" min-width="160">
              <template #default="{ row }">
                <el-tooltip v-if="row.error_code" :content="row.error_code" placement="top">
                  <StatusTag variant="danger">{{ formatErrorCode(row.error_code) }}</StatusTag>
                </el-tooltip>
                <span v-else class="muted">-</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-bar">
            <span>共 <b>{{ logTotal }}</b> 条</span>
            <el-pagination background layout="prev, pager, next, sizes"
              :total="logTotal"
              v-model:current-page="logPage"
              v-model:page-size="logFilter.limit"
              :page-sizes="[10, 20, 50, 100]"
              @size-change="refreshLogs" />
          </div>
        </el-tab-pane>

        <!-- ----- 积分流水 ----- -->
        <el-tab-pane name="credits" label="积分流水">
          <div class="tab-toolbar">
            <span class="muted">展示账户所有账变：充值、消费、退款、管理员调账等。金额为正表示收入，负表示支出。</span>
            <NeonButton variant="ghost" @click="loadCredits">
              <el-icon><Refresh /></el-icon> 刷新
            </NeonButton>
          </div>

          <el-table :data="creditItems" stripe size="small" v-loading="creditLoading" empty-text="暂无账变">
            <el-table-column prop="created_at" label="时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="类型" width="100">
              <template #default="{ row }">
                <StatusTag variant="info">{{ typeLabel[row.type] || row.type }}</StatusTag>
              </template>
            </el-table-column>
            <el-table-column label="金额" width="140">
              <template #default="{ row }">
                <span :class="row.amount >= 0 ? 'amount-in' : 'amount-out'">
                  {{ row.amount >= 0 ? '+' : '' }}{{ formatCredit(row.amount) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="余额" width="140">
              <template #default="{ row }">{{ formatCredit(row.balance_after) }}</template>
            </el-table-column>
            <el-table-column label="Key" width="90">
              <template #default="{ row }">
                <span v-if="row.key_id">#{{ row.key_id }}</span>
                <span v-else class="muted">-</span>
              </template>
            </el-table-column>
            <el-table-column label="关联" min-width="160">
              <template #default="{ row }">
                <span v-if="row.ref_id" class="ref">{{ row.ref_id }}</span>
                <span v-else class="muted">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="remark" label="备注" min-width="200" show-overflow-tooltip />
          </el-table>

          <div class="pagination-bar">
            <span>共 <b>{{ creditTotal }}</b> 条</span>
            <el-pagination background layout="prev, pager, next, sizes"
              :total="creditTotal"
              v-model:current-page="creditPage"
              v-model:page-size="creditFilter.limit"
              :page-sizes="[10, 20, 50, 100]"
              @size-change="loadCredits" />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.kpi-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.panel {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 22px 24px;

  &__head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 14px;
  }

  &__sub {
    font-size: var(--fs-sm);
    color: $gray-500;
    margin-top: 3px;
  }

  h4 {
    margin: 0 0 14px;
    font-size: 16px;
    font-weight: 800;
    letter-spacing: -0.01em;
    color: $n-ink;
  }
}

/* 图例 */
.legend {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: $gray-500;
  flex-wrap: wrap;
}
.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
  margin-left: 8px;
  &--purple { background: #A855F7; }
  &--danger  { background: #FF6B35; }
}
.legend-label { margin-right: 4px; }

/* 图表 */
.chart-wrap {
  width: 100%;
  min-height: 220px;
  position: relative;
}
.chart-svg {
  width: 100%;
  display: block;
  font-family: $f-sans;
  user-select: none;
}

.chart-svg .grid-line {
  stroke: $gray-200;
  stroke-width: 1;
  stroke-dasharray: 3 4;
}
.chart-svg .grid-line.grid-zero {
  stroke: $gray-300;
  stroke-dasharray: none;
}
.chart-svg .axis-tick {
  fill: $gray-400;
  font-size: 11px;
}
.chart-svg .axis-date {
  fill: $gray-500;
  font-size: 11px;
}
.chart-svg .bar-group { cursor: pointer; }
.chart-svg .bar-hit { fill: transparent; }
.chart-svg .bar-rect { transition: opacity .15s; }
.chart-svg .bar-group:hover .bar-hit {
  fill: rgba(168, 85, 247, 0.04);
}
.chart-svg .bar-group:not(.hover) .bar-rect { opacity: 0.88; }
.chart-svg .bar-group .bar-rect {
  filter: drop-shadow(0 1px 2px rgba(168, 85, 247, 0.2));
}
.chart-svg .hover-guide {
  stroke: #A855F7;
  stroke-width: 1;
  stroke-dasharray: 3 3;
  opacity: 0.45;
  pointer-events: none;
}

/* tooltip */
.chart-tip {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-md;
  box-shadow: $sh-2;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.6;
  color: $n-ink;
  pointer-events: none;
}
.chart-tip .tip-day {
  font-weight: 700;
  margin-bottom: 2px;
  color: $gray-600;
}
.chart-tip .tip-row {
  display: flex;
  align-items: center;
  b { margin-left: auto; font-weight: 700; }
}
.chart-tip .tip-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 6px;
  &--purple { background: #A855F7; }
  &--danger  { background: #FF6B35; }
}

/* tab 工具栏 */
.tab-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  gap: 8px;
}

/* 分页栏 */
.pagination-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
  font-size: var(--fs-sm);
  color: $gray-500;
  b { color: $n-ink; font-weight: 700; }
}

/* 表格内细节 */
.muted { color: $gray-500; font-size: 13px; }
.req-id {
  font-family: $f-mono;
  font-size: 12px;
  color: $gray-600;
  display: inline-block;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}
.cost { font-weight: 700; color: $c-orange-text; }
.amount-in  { color: $c-green-text;  font-weight: 700; }
.amount-out { color: $c-orange-text; font-weight: 700; }
.ref {
  background: $gray-100;
  padding: 1px 6px;
  border-radius: $r-sm;
  font-family: $f-mono;
  font-size: 12px;
}

@media (max-width: 1000px) {
  .kpi-grid { grid-template-columns: 1fr; }
}
</style>
