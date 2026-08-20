<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { listKeys } from '@/api/apikey'
import type { ApiKey } from '@/api/apikey'
import * as meApi from '@/api/me'
import { formatCredit, formatDateTime, formatErrorCode } from '@/utils/format'
import KpiWallet from '@/assets/illustrations/kpi-wallet.svg?component'
import KpiChart from '@/assets/illustrations/kpi-chart.svg?component'
import KpiCheck from '@/assets/illustrations/kpi-check.svg?component'
import KpiChain from '@/assets/illustrations/kpi-chain.svg?component'

const store = useUserStore()
const { user } = storeToRefs(store)
const router = useRouter()

// ---------- 基本信息 ----------
const balance = computed(() => formatCredit(user.value?.credit_balance))
const frozen = computed(() => formatCredit(user.value?.credit_frozen))
const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
})

// ---------- 数据:API Keys / 模型 / 统计 / 最近日志 / 账变 ----------
const loading = ref(false)
const keyTotal = ref(0)
const keyActive = ref(0)
const modelCount = ref(0)
const previewKeys = ref<ApiKey[]>([])
const stats14 = ref<meApi.MyStatsResp | null>(null)
const stats1 = ref<meApi.MyStatsResp | null>(null)
const recentLogs = ref<meApi.UsageItem[]>([])
const recentCredits = ref<meApi.MyCreditLog[]>([])

async function loadAll() {
  loading.value = true
  try {
    await store.fetchMe()
    const [keys, models, s14, s1, logs, credits] = await Promise.all([
      listKeys(1, 100),
      meApi.listMyModels(),
      meApi.getMyUsageStats({ days: 14, top_n: 3 }),
      meApi.getMyUsageStats({ days: 1, top_n: 1 }),
      meApi.listMyUsageLogs({ limit: 6, offset: 0 }),
      meApi.listMyCreditLogs({ limit: 3, offset: 0 }),
    ])
    keyTotal.value = keys.total
    keyActive.value = keys.list.filter((k) => k.enabled).length
    modelCount.value = models.total
    previewKeys.value = keys.list.slice(0, 5)
    stats14.value = s14
    stats1.value = s1
    recentLogs.value = logs.items
    recentCredits.value = credits.items
  } finally {
    loading.value = false
  }
}

// ---------- 派生指标 ----------
const todayOverall = computed(() => stats1.value?.overall)
const monthOverall = computed(() => stats14.value?.overall) // 近 14 天,作为"近期"展示
const daily = computed(() => stats14.value?.daily || [])
const topModels = computed(() => (stats14.value?.by_model || []).slice(0, 3))

function successRate(o?: meApi.UsageOverall | null): string {
  if (!o || o.requests === 0) return '—'
  return `${(((o.requests - o.failures) / o.requests) * 100).toFixed(1)}%`
}

// ---------- 趋势图(SVG)—— 复用 Usage 页的思路,改成面积图+折线 ----------
const chartWrap = ref<HTMLElement | null>(null)
const chartW = ref(640)
const chartH = 140
const padT = 10
const padB = 24
const padL = 32
const padR = 10
const hoverIdx = ref(-1)

let ro: ResizeObserver | null = null
onMounted(async () => {
  if (chartWrap.value && typeof ResizeObserver !== 'undefined') {
    ro = new ResizeObserver((entries) => {
      for (const e of entries) {
        const w = e.contentRect.width
        if (w > 0) chartW.value = Math.floor(w)
      }
    })
    ro.observe(chartWrap.value)
  }
  await loadAll()
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
const yMax = computed(() => niceMax(daily.value.reduce((x, r) => Math.max(x, r.requests), 0) || 1))
const yTicks = computed(() => {
  const m = yMax.value
  return [0, m / 2, m].map((v) => Math.round(v))
})
const cellW = computed(() => {
  const n = Math.max(daily.value.length, 1)
  return (chartW.value - padL - padR) / n
})
function xCenter(i: number) { return padL + cellW.value * (i + 0.5) }
function pointY(v: number) {
  const innerH = chartH - padT - padB
  return chartH - padB - (v / yMax.value) * innerH
}

// 折线 path
const linePath = computed(() => {
  if (!daily.value.length) return ''
  return daily.value
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${xCenter(i).toFixed(1)},${pointY(p.requests).toFixed(1)}`)
    .join(' ')
})
// 面积 path(折线 + 底边)
const areaPath = computed(() => {
  if (!daily.value.length) return ''
  const n = daily.value.length
  const baseY = chartH - padB
  let d = `M${xCenter(0).toFixed(1)},${baseY.toFixed(1)}`
  daily.value.forEach((p, i) => {
    d += ` L${xCenter(i).toFixed(1)},${pointY(p.requests).toFixed(1)}`
  })
  d += ` L${xCenter(n - 1).toFixed(1)},${baseY.toFixed(1)} Z`
  return d
})

const labelStep = computed(() => (daily.value.length > 10 ? 3 : 2))
function shouldShowLabel(i: number) {
  const n = daily.value.length
  if (i === 0 || i === n - 1) return true
  return i % labelStep.value === 0
}

const tipX = computed(() => (hoverIdx.value >= 0 ? xCenter(hoverIdx.value) : 0))
const tipY = computed(() => (hoverIdx.value >= 0 ? pointY(daily.value[hoverIdx.value]?.requests || 0) : 0))
const tipSide = computed<'left' | 'right'>(() => (tipX.value > chartW.value / 2 ? 'left' : 'right'))

// ---------- TOP 模型横向条 ----------
const maxTop = computed(() => topModels.value.reduce((x, r) => Math.max(x, r.requests), 0) || 1)

// ---------- 最近请求/账变 辅助 ----------
const statusMap: Record<string, { tag: 'success' | 'danger' | 'warning' | 'info'; label: string }> = {
  success: { tag: 'success', label: '成功' },
  failed:  { tag: 'danger',  label: '失败' },
  partial: { tag: 'warning', label: '部分' },
}
function statusTag(s: string) { return statusMap[s]?.tag || 'info' }
function statusLabel(s: string) { return statusMap[s]?.label || s || '-' }

const creditTypeLabel: Record<string, string> = {
  recharge: '充值', consume: '消费', refund: '退款',
  adjust: '调账', bonus: '赠送', freeze: '冻结', unfreeze: '解冻',
}

// ---------- 导航 CTA ----------
function go(p: string) { router.push(p) }
</script>

<template>
  <div class="page-container" v-loading="loading">

    <PageHeader crumb="个人中心" title="总览" accent-word="览" accent="pink">
      <template #extra>
        <NeonButton variant="outline" @click="loadAll">
          <el-icon><Refresh /></el-icon> 刷新
        </NeonButton>
      </template>
    </PageHeader>

    <!-- 问候语 -->
    <h2 class="greet">{{ greeting }}，<span class="greet-accent">{{ user?.nickname || user?.email }}</span> 👋</h2>
    <p class="greet-sub">你当前余额 <b>{{ balance }}</b>，近 14 天共调用 API <b>{{ monthOverall?.requests ?? 0 }}</b> 次。</p>

    <!-- KPI 四宫格 -->
    <div class="kpi-grid">
      <KpiCard accent="pink" label="余额" :value="balance" :change="`冻结 ${frozen}`">
        <template #illustration><KpiWallet style="width:60px;height:50px" /></template>
      </KpiCard>
      <KpiCard accent="cyan" label="今日调用" :value="todayOverall?.requests ?? 0" :change="`${successRate(todayOverall)} 成功`" change-dir="up">
        <template #illustration><KpiChart style="width:60px;height:50px" /></template>
      </KpiCard>
      <KpiCard accent="yellow" label="成功率" :value="successRate(monthOverall)" :change="`${monthOverall?.failures ?? 0} 次失败`">
        <template #illustration><KpiCheck style="width:60px;height:50px" /></template>
      </KpiCard>
      <KpiCard accent="purple" label="活跃 Key" :value="`${keyActive} / ${keyTotal}`" :change="`${keyTotal - keyActive} 个已禁用`">
        <template #illustration><KpiChain style="width:60px;height:50px" /></template>
      </KpiCard>
    </div>

    <!-- 趋势图 + Keys 快览 -->
    <div class="two-col">
      <!-- 近 14 天调用趋势 -->
      <div class="panel">
        <h4>近 14 天调用趋势</h4>
        <div ref="chartWrap" class="chart-wrap">
          <el-empty
            v-if="!loading && daily.length === 0"
            description="暂无数据，先在「在线体验」发一条请求试试"
            :image-size="80"
            style="padding:24px 0"
          />
          <svg
            v-else
            class="chart-svg"
            :viewBox="`0 0 ${chartW} ${chartH}`"
            :style="{ height: chartH + 'px' }"
            @mouseleave="hoverIdx = -1"
          >
            <!-- 网格 + y 刻度 -->
            <g class="chart-axis">
              <line
                v-for="(t, ti) in yTicks" :key="'ty' + ti"
                :x1="padL" :x2="chartW - padR"
                :y1="pointY(t)" :y2="pointY(t)"
                class="grid-line"
                :class="{ 'grid-zero': ti === 0 }"
              />
              <text
                v-for="(t, ti) in yTicks" :key="'tl' + ti"
                :x="padL - 6" :y="pointY(t) + 4"
                text-anchor="end" class="axis-tick"
              >{{ t }}</text>
            </g>

            <!-- 面积 + 折线 -->
            <path :d="areaPath" fill="rgba(255,61,148,0.08)" stroke="none" />
            <path
              :d="linePath"
              fill="none"
              stroke="#FF3D94"
              stroke-width="2"
              stroke-linejoin="round"
              stroke-linecap="round"
            />

            <!-- 交互矩形 + 圆点 + 日期 -->
            <g v-for="(p, i) in daily" :key="p.day">
              <rect
                :x="padL + cellW * i" :y="padT"
                :width="cellW" :height="chartH - padT - padB"
                fill="transparent"
                @mouseenter="hoverIdx = i"
              />
              <circle
                :cx="xCenter(i)" :cy="pointY(p.requests)"
                :r="hoverIdx === i ? 4.5 : 2.5"
                fill="#FF3D94"
              />
              <text
                v-if="shouldShowLabel(i)"
                :x="xCenter(i)" :y="chartH - padB + 14"
                text-anchor="middle" class="axis-date"
              >{{ p.day.slice(5) }}</text>
            </g>

            <!-- hover 指示 + tooltip -->
            <line
              v-if="hoverIdx >= 0"
              :x1="tipX" :x2="tipX"
              :y1="padT" :y2="chartH - padB"
              class="hover-guide"
            />
            <foreignObject
              v-if="hoverIdx >= 0"
              :x="tipSide === 'right' ? tipX + 10 : tipX - 170"
              :y="Math.max(padT, tipY - 60)"
              width="160" height="72"
            >
              <div class="chart-tip">
                <div class="tip-day">{{ daily[hoverIdx]?.day }}</div>
                <div class="tip-row">
                  <span class="tip-dot pink"></span>请求
                  <b>{{ daily[hoverIdx]?.requests || 0 }}</b>
                </div>
                <div class="tip-row">
                  <span class="tip-dot orange"></span>失败
                  <b>{{ daily[hoverIdx]?.failures || 0 }}</b>
                </div>
              </div>
            </foreignObject>
          </svg>
        </div>
      </div>

      <!-- 我的 API Keys 快览 -->
      <div class="panel">
        <h4>我的 API Keys</h4>
        <div class="klist" style="margin-bottom:14px">
          <div v-for="k in previewKeys" :key="k.id" class="kitem">
            <code>{{ k.key_prefix }}...</code>
            <StatusTag v-if="k.enabled" variant="active" dot>启用</StatusTag>
            <StatusTag v-else variant="disabled" dot>禁用</StatusTag>
          </div>
          <div v-if="previewKeys.length === 0 && !loading" class="muted" style="font-size:13px;text-align:center;padding:8px 0">
            暂无 Key
          </div>
        </div>
        <NeonButton variant="pink" :block="true" @click="router.push('/personal/keys')" size="md">
          生成新 Key +
        </NeonButton>
      </div>
    </div>

    <!-- 最近请求 + 最近账变 -->
    <div class="two-col" style="margin-top:16px">
      <!-- 最近请求 -->
      <div class="panel">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:14px">
          <h4 style="margin:0">最近请求</h4>
          <NeonButton variant="ghost" size="sm" @click="go('/personal/usage')">查看全部</NeonButton>
        </div>
        <el-table
          :data="recentLogs"
          stripe
          size="small"
          empty-text="暂无请求记录"
          :show-header="recentLogs.length > 0"
        >
          <el-table-column prop="created_at" label="时间" width="150">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="模型" min-width="140">
            <template #default="{ row }">
              <code>{{ row.model_slug || `#${row.model_id}` }}</code>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <StatusTag :variant="statusTag(row.status)">{{ statusLabel(row.status) }}</StatusTag>
            </template>
          </el-table-column>
          <el-table-column label="扣费" width="100">
            <template #default="{ row }">
              <span class="cost">{{ formatCredit(row.credit_cost) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="错误" min-width="120">
            <template #default="{ row }">
              <el-tooltip v-if="row.error_code" :content="row.error_code" placement="top">
                <span class="err">{{ formatErrorCode(row.error_code) }}</span>
              </el-tooltip>
              <span v-else class="muted">-</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 最近账变 -->
      <div class="panel">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:14px">
          <h4 style="margin:0">最近账变</h4>
          <NeonButton variant="ghost" size="sm" @click="go('/personal/billing')">查看全部</NeonButton>
        </div>
        <div v-if="recentCredits.length === 0" class="muted" style="padding:16px 0;text-align:center">
          暂无账变记录
        </div>
        <div v-else class="credit-list">
          <div v-for="c in recentCredits" :key="c.id" class="credit-row">
            <div class="cr-left">
              <el-tag size="small" effect="plain">
                {{ creditTypeLabel[c.type] || c.type }}
              </el-tag>
              <div class="cr-remark">{{ c.remark || '-' }}</div>
              <div class="cr-time muted">{{ formatDateTime(c.created_at) }}</div>
            </div>
            <div class="cr-right">
              <div :class="['cr-amt', c.amount >= 0 ? 'in' : 'out']">
                {{ c.amount >= 0 ? '+' : '' }}{{ formatCredit(c.amount) }}
              </div>
              <div class="cr-bal muted">余额 {{ formatCredit(c.balance_after) }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.greet {
  font-size: var(--fs-h2);
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 0 0 6px;
  color: $n-ink;
  .greet-accent { color: $c-pink; }
}
.greet-sub { color: $gray-500; margin: 0 0 24px; font-size: 15px; b { color: $n-ink; font-weight: 700; } }

.kpi-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 28px; }

.two-col { display: grid; grid-template-columns: 2fr 1fr; gap: 16px; }

.panel {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 22px 24px;
  h4 { margin: 0 0 14px; font-size: 16px; font-weight: 800; letter-spacing: -0.01em; color: $n-ink; }
}

.chart-wrap {
  width: 100%;
  overflow: hidden;
}

.chart-svg {
  width: 100%;
  display: block;
  user-select: none;
}
.chart-svg .grid-line {
  stroke: $gray-200;
  stroke-width: 1;
  stroke-dasharray: 3 4;
}
.chart-svg .grid-line.grid-zero { stroke: $gray-300; stroke-dasharray: none; }
.chart-svg .axis-tick { fill: $gray-400; font-size: 10.5px; }
.chart-svg .axis-date { fill: $gray-500; font-size: 10.5px; }
.chart-svg .hover-guide {
  stroke: #FF3D94;
  stroke-width: 1;
  stroke-dasharray: 3 3;
  opacity: 0.45;
  pointer-events: none;
}
.chart-tip {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-md;
  box-shadow: $sh-1;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.6;
  color: $n-ink;
  pointer-events: none;
}
.chart-tip .tip-day { font-weight: 600; margin-bottom: 2px; }
.chart-tip .tip-row { display: flex; align-items: center; }
.chart-tip .tip-row b { margin-left: auto; font-weight: 600; }
.chart-tip .tip-dot {
  width: 8px; height: 8px; border-radius: 50%;
  display: inline-block; margin-right: 6px;
}
.chart-tip .tip-dot.pink   { background: $c-pink; }
.chart-tip .tip-dot.orange { background: $c-orange; }

.klist { display: flex; flex-direction: column; gap: 10px; }
.kitem {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 14px;
  background: $n-cloud;
  border-radius: $r-md;
  border: $bw solid $gray-200;
  font-size: 13px;
  code {
    font-family: $f-mono;
    color: $c-pink;
    background: none;
    padding: 0;
  }
}

.muted { color: $gray-500; font-size: 13px; }
.cost { font-weight: 600; color: $c-orange-text; }
.err  { color: $c-pink; font-size: 12px; }

.credit-list { display: flex; flex-direction: column; }
.credit-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 8px 2px;
  border-bottom: $bw solid $gray-200;
}
.credit-row:last-child { border-bottom: none; }
.cr-left { min-width: 0; flex: 1; }
.cr-remark {
  margin-top: 4px;
  font-size: 13px;
  color: $n-ink;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 260px;
}
.cr-time { font-size: 11.5px; margin-top: 2px; }
.cr-right { text-align: right; flex-shrink: 0; }
.cr-amt { font-weight: 700; font-size: 15px; font-variant-numeric: tabular-nums; }
.cr-amt.in  { color: $c-green-text; }
.cr-amt.out { color: $c-orange-text; }
.cr-bal { font-size: 11.5px; margin-top: 2px; }

@media (max-width: 1100px) {
  .kpi-grid { grid-template-columns: repeat(2, 1fr); }
  .two-col { grid-template-columns: 1fr; }
}
</style>
