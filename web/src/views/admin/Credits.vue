<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as adminApi from '@/api/admin'

// -------- 统计摘要 --------
const summary = ref<adminApi.CreditsSummary | null>(null)

async function fetchSummary() {
  try {
    summary.value = await adminApi.creditsSummary()
  } catch (e: any) {
    ElMessage.error(e?.message || '加载摘要失败')
  }
}

/** credit 单位是"厘",10000 厘 = 1 积分。展示时转成积分,保留 2 位小数。 */
function fmtCredits(milli: number | undefined | null) {
  if (!milli) return '0'
  const v = milli / 10000
  return v.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

// -------- 流水表格 --------
const loading = ref(false)
const rows = ref<adminApi.CreditLogGlobal[]>([])
const total = ref(0)
const pager = reactive({ limit: 20, offset: 0 })
const page = computed({
  get: () => Math.floor(pager.offset / pager.limit) + 1,
  set: (v: number) => { pager.offset = (v - 1) * pager.limit },
})

const filter = reactive<adminApi.CreditLogFilter>({
  user_id: undefined,
  keyword: '',
  type: '',
  sign: '',
  start_at: '',
  end_at: '',
})

const timeRange = ref<[string, string] | null>(null)

async function fetchLogs() {
  loading.value = true
  try {
    const q: adminApi.CreditLogFilter = { ...filter, ...pager }
    if (timeRange.value && timeRange.value.length === 2) {
      q.start_at = timeRange.value[0]
      q.end_at = timeRange.value[1]
    } else {
      delete q.start_at
      delete q.end_at
    }
    if (!q.keyword) delete q.keyword
    if (!q.user_id) delete q.user_id
    if (!q.type) delete q.type
    if (!q.sign) delete q.sign

    const data = await adminApi.listCreditLogsGlobal(q)
    rows.value = data.items || []
    total.value = data.total || 0
  } catch (e: any) {
    ElMessage.error(e?.message || '加载流水失败')
  } finally { loading.value = false }
}

function onSearch() {
  pager.offset = 0
  fetchLogs()
}
function onReset() {
  filter.user_id = undefined
  filter.keyword = ''
  filter.type = ''
  filter.sign = ''
  timeRange.value = null
  pager.offset = 0
  fetchLogs()
}

// -------- 类型徽标 --------
const TYPE_MAP: Record<string, { label: string; variant: 'success' | 'warning' | 'info' | 'danger' | 'purple' }> = {
  recharge:     { label: '充值',       variant: 'success' },
  redeem:       { label: '兑换码',     variant: 'success' },
  admin_adjust: { label: '管理员调账', variant: 'warning' },
  refund:       { label: '退款',       variant: 'purple'  },
  consume:      { label: '消费',       variant: 'danger'  },
  freeze:       { label: '冻结',       variant: 'info'    },
  unfreeze:     { label: '解冻',       variant: 'info'    },
}
function typeInfo(t: string) { return TYPE_MAP[t] || { label: t, variant: 'info' as const } }

// -------- 调账对话框 --------
const adjDlg = ref(false)
const adjLoading = ref(false)
const adjForm = reactive({
  user_id: null as number | null,
  delta_credits: null as number | null, // 注意:前端填"积分",提交时 ×10000
  remark: '',
  ref_id: '',
  admin_password: '',
})

function openAdjust(row?: adminApi.CreditLogGlobal) {
  adjForm.user_id = row?.user_id ?? null
  adjForm.delta_credits = null
  adjForm.remark = ''
  adjForm.ref_id = ''
  adjForm.admin_password = ''
  adjDlg.value = true
}

async function submitAdjust() {
  if (!adjForm.user_id || adjForm.user_id <= 0) return ElMessage.warning('请填写有效的用户 ID')
  if (!adjForm.delta_credits || adjForm.delta_credits === 0) return ElMessage.warning('调账积分不能为 0')
  if (!adjForm.remark.trim()) return ElMessage.warning('请填写备注(便于稽核)')
  if (!adjForm.admin_password) return ElMessage.warning('需要二次输入您当前账号的登录密码')

  const delta = Math.round(adjForm.delta_credits * 10000) // 积分 → 厘
  await ElMessageBox.confirm(
    `确认对用户 #${adjForm.user_id} ${delta > 0 ? '增加' : '扣减'} ${Math.abs(adjForm.delta_credits)} 积分?此操作会写入审计日志。`,
    '调账确认',
    { type: 'warning', confirmButtonText: '确认调账', cancelButtonText: '取消' },
  )

  adjLoading.value = true
  try {
    const r = await adminApi.adjustCreditByUser({
      user_id: adjForm.user_id,
      delta,
      remark: adjForm.remark,
      ref_id: adjForm.ref_id || undefined,
    }, adjForm.admin_password)
    ElMessage.success(`调账成功 · 用户当前余额 ${fmtCredits(r.balance_after)} 积分`)
    adjDlg.value = false
    fetchSummary()
    fetchLogs()
  } catch (e: any) {
    ElMessage.error(e?.message || '调账失败')
  } finally { adjLoading.value = false }
}

onMounted(() => {
  fetchSummary()
  fetchLogs()
})
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 积分" title="积分管理" accent-word="管理" accent="green">
      <template #extra>
        <NeonButton variant="green" @click="openAdjust()">批量调账</NeonButton>
      </template>
    </PageHeader>

    <!-- 摘要卡片 -->
    <div class="summary-row">
      <div class="sum-card income">
        <div class="sum-title">今日新增</div>
        <div class="sum-value">+ {{ fmtCredits(summary?.in_today) }}</div>
        <div class="sum-sub">今日消费 - {{ fmtCredits(summary?.out_today) }}</div>
      </div>
      <div class="sum-card week">
        <div class="sum-title">近 7 天新增</div>
        <div class="sum-value">+ {{ fmtCredits(summary?.in_7days) }}</div>
        <div class="sum-sub">近 7 天消费 - {{ fmtCredits(summary?.out_7days) }}</div>
      </div>
      <div class="sum-card total">
        <div class="sum-title">累计入账</div>
        <div class="sum-value">{{ fmtCredits(summary?.in_total) }}</div>
        <div class="sum-sub">累计消费 {{ fmtCredits(summary?.out_total) }}</div>
      </div>
      <div class="sum-card balance">
        <div class="sum-title">全站当前余额</div>
        <div class="sum-value">{{ fmtCredits(summary?.total_balance) }}</div>
        <div class="sum-sub">所有用户未消费积分总和</div>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-input-number v-model="filter.user_id" :min="1" controls-position="right"
        placeholder="用户 ID" style="width:160px" />
      <el-input v-model="filter.keyword" placeholder="邮箱/昵称" clearable style="width:200px" />
      <el-select v-model="filter.type" clearable placeholder="全部类型" style="width:150px">
        <el-option v-for="(v, k) in TYPE_MAP" :key="k" :label="v.label" :value="k" />
      </el-select>
      <el-select v-model="filter.sign" clearable placeholder="全部方向" style="width:120px">
        <el-option label="入账" value="in" />
        <el-option label="出账" value="out" />
      </el-select>
      <el-date-picker v-model="timeRange" type="datetimerange"
        value-format="YYYY-MM-DD HH:mm:ss"
        start-placeholder="开始" end-placeholder="结束" clearable />
      <NeonButton variant="green" size="sm" @click="onSearch">查询</NeonButton>
      <NeonButton variant="ghost" size="sm" @click="onReset">重置</NeonButton>
    </div>

    <!-- 表格 -->
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="created_at" label="时间" width="170" />
      <el-table-column label="用户" min-width="200">
        <template #default="{ row }">
          <div class="cell-user">
            <UserAvatar :name="row.user_email || String(row.user_id)" size="sm" />
            <div>
              <div class="u-email">{{ row.user_email || '-' }}</div>
              <div class="u-sub">#{{ row.user_id }}<template v-if="row.user_nickname"> · {{ row.user_nickname }}</template></div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="120">
        <template #default="{ row }">
          <StatusTag :variant="typeInfo(row.type).variant" dot>{{ typeInfo(row.type).label }}</StatusTag>
        </template>
      </el-table-column>
      <el-table-column label="变动" width="150" align="right">
        <template #default="{ row }">
          <span :class="row.amount >= 0 ? 'delta-up' : 'delta-down'">
            {{ row.amount >= 0 ? '+' : '' }}{{ fmtCredits(row.amount) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="余额" width="150" align="right">
        <template #default="{ row }">
          <span class="num-mono">{{ fmtCredits(row.balance_after) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="关联单号" width="180" show-overflow-tooltip>
        <template #default="{ row }">
          <code v-if="row.ref_id" class="ref">{{ row.ref_id }}</code>
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="200" show-overflow-tooltip />
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <div class="op">
            <a class="op-link" @click="openAdjust(row)">调账</a>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- Empty state -->
    <EmptyState
      v-if="!loading && total === 0"
      title="暂无积分流水"
      desc="换个筛选条件试试。"
    />

    <!-- Pagination bar -->
    <div class="pagination-bar">
      <span>共 <b>{{ total }}</b> 条流水</span>
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pager.limit"
        :total="total"
        :page-sizes="[20, 50, 100, 200]"
        layout="sizes, prev, pager, next"
        @current-change="fetchLogs"
        @size-change="() => { pager.offset = 0; fetchLogs() }"
      />
    </div>

    <!-- 调账对话框 -->
    <el-dialog v-model="adjDlg" title="手动调账" width="540px" @closed="() => { adjForm.admin_password = '' }">
      <el-form :model="adjForm" label-width="100px">
        <el-form-item label="目标用户 ID" required>
          <el-input-number v-model="adjForm.user_id" :min="1" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item label="调账积分" required>
          <el-input-number v-model="adjForm.delta_credits" :precision="2"
            :step="100" controls-position="right" style="width:100%"
            placeholder="正数=增加,负数=扣减" />
          <div class="form-hint">单位:积分(1 积分 = 10000 厘,后端精度单位)。</div>
        </el-form-item>
        <el-form-item label="备注" required>
          <el-input v-model="adjForm.remark" maxlength="200" show-word-limit
            placeholder="请简要说明调账原因(写入审计日志)" />
        </el-form-item>
        <el-form-item label="关联单号">
          <el-input v-model="adjForm.ref_id" placeholder="可选,如工单号、退款单号" />
        </el-form-item>
        <el-form-item label="管理员密码" required>
          <el-input v-model="adjForm.admin_password" type="password" show-password
            placeholder="请再次输入您当前账号的登录密码以二次确认" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="adjDlg = false">取消</NeonButton>
        <NeonButton variant="green" :loading="adjLoading" @click="submitAdjust">确认调账</NeonButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

/* ---- 摘要卡 ---- */
.summary-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}
.sum-card {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 16px 18px;
  position: relative;
  overflow: hidden;
  transition: transform 0.15s, box-shadow 0.15s;
  &:hover { transform: translateY(-1px); box-shadow: 0 2px 10px rgba(0,0,0,0.05); }
  &::before {
    content: '';
    position: absolute;
    top: 0; left: 0; right: 0;
    height: 3px;
  }
  &.income::before  { background: $c-green; }
  &.week::before    { background: $c-yellow-text; }
  &.total::before   { background: $c-orange; }
  &.balance::before { background: $c-purple; }
}
.sum-title { font-size: 13px; color: $gray-500; }
.sum-value { font-size: 22px; font-weight: 700; margin-top: 6px; color: $n-ink; font-family: $f-mono; line-height: 1.2; }
.sum-sub   { font-size: 12px; color: $gray-500; margin-top: 4px; }
@media (max-width: 1100px) {
  .summary-row { grid-template-columns: repeat(2, 1fr); }
}

/* ---- 共用布局 ---- */
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

.cell-user { display: flex; align-items: center; gap: 10px; }
.u-email { font-weight: 700; color: $n-ink; font-size: 14px; }
.u-sub   { font-size: 12px; color: $gray-500; }

.num-mono { font-family: $f-mono; font-size: 14px; color: $n-ink; }

.delta-up   { color: $c-green-text;  font-weight: 700; font-family: $f-mono; }
.delta-down { color: $c-orange-text; font-weight: 700; font-family: $f-mono; }

.muted { color: $gray-500; }
.ref {
  background: $gray-200;
  padding: 1px 6px;
  border-radius: $r-sm;
  font-family: $f-mono;
  font-size: 12px;
}

.op { display: flex; gap: 4px; justify-content: flex-end; }
.op-link {
  color: $c-green-text;
  font-size: 13px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: $r-sm;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
  &:hover { background: rgba(0, 230, 118, 0.08); }
  &.danger { color: $c-orange-text; &:hover { background: rgba(255, 107, 53, 0.08); } }
}

.pagination-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
  font-size: var(--fs-sm);
  color: $gray-500;
  b { color: $n-ink; font-weight: 700; }
}

.form-hint { font-size: 12px; color: $gray-500; line-height: 1.5; margin-top: 4px; }
</style>
