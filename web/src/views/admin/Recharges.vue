<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as rechargeApi from '@/api/recharge'
import { formatCredit } from '@/utils/format'

const tab = ref<'packages' | 'orders'>('packages')

// ---------- packages ----------
const packages = ref<rechargeApi.Package[]>([])
const loadingPkg = ref(false)

async function loadPackages() {
  loadingPkg.value = true
  try {
    const d = await rechargeApi.adminListPackages()
    packages.value = d.items
  } finally { loadingPkg.value = false }
}

const pkgDialog = reactive({
  visible: false,
  mode: 'create' as 'create' | 'edit',
  form: {
    id: 0, name: '', price_cny: 100, credits: 1000000,
    bonus: 0, description: '', sort: 0, enabled: true,
  } as Partial<rechargeApi.Package>,
})
function openCreatePkg() {
  pkgDialog.mode = 'create'
  Object.assign(pkgDialog.form, {
    id: 0, name: '', price_cny: 100, credits: 1000000, bonus: 0,
    description: '', sort: 0, enabled: true,
  })
  pkgDialog.visible = true
}
function openEditPkg(p: rechargeApi.Package) {
  pkgDialog.mode = 'edit'
  Object.assign(pkgDialog.form, p)
  pkgDialog.visible = true
}
async function savePkg() {
  const f = pkgDialog.form
  if (!f.name || (f.price_cny ?? 0) <= 0) {
    ElMessage.warning('名称和金额不能为空')
    return
  }
  if (pkgDialog.mode === 'create') {
    await rechargeApi.adminCreatePackage(f)
    ElMessage.success('已创建')
  } else {
    await rechargeApi.adminUpdatePackage(f.id!, f)
    ElMessage.success('已保存')
  }
  pkgDialog.visible = false
  loadPackages()
}
async function deletePkg(p: rechargeApi.Package) {
  await ElMessageBox.confirm(`确认删除套餐【${p.name}】?该操作不可撤销`, '删除套餐', { type: 'warning' })
  await rechargeApi.adminDeletePackage(p.id)
  ElMessage.success('已删除')
  loadPackages()
}

// ---------- orders ----------
const orders = ref<rechargeApi.Order[]>([])
const total = ref(0)
const loadingOrd = ref(false)
const filter = reactive({
  user_id: undefined as number | undefined,
  status: '' as '' | 'pending' | 'paid' | 'cancelled' | 'expired' | 'failed',
  limit: 20,
  offset: 0,
})

async function loadOrders() {
  loadingOrd.value = true
  try {
    const d = await rechargeApi.adminListOrders({
      user_id: filter.user_id || undefined,
      status: filter.status || undefined,
      limit: filter.limit,
      offset: filter.offset,
    })
    orders.value = d.items
    total.value = d.total
  } finally { loadingOrd.value = false }
}

async function forcePaid(o: rechargeApi.Order) {
  if (o.status !== 'pending') {
    ElMessage.warning('只有 pending 状态可以手工入账')
    return
  }
  const { value: pwd } = await ElMessageBox.prompt(
    `请输入管理员密码以确认为订单 ${o.out_trade_no} 强制入账(不会调用上游收银台)。`,
    '手工入账',
    { type: 'warning', inputType: 'password', confirmButtonText: '确认入账', cancelButtonText: '取消' },
  )
  if (!pwd) return
  await rechargeApi.adminForcePaid(o.id, pwd)
  ElMessage.success('已入账')
  loadOrders()
}

function orderStatusVariant(s: string): 'success' | 'warning' | 'info' | 'danger' | 'free' {
  if (s === 'paid')      return 'success'
  if (s === 'pending')   return 'warning'
  if (s === 'refunded')  return 'info'
  if (s === 'cancelled' || s === 'expired') return 'info'
  if (s === 'failed')    return 'danger'
  return 'free'
}
const statusLabel: Record<string, string> = {
  paid: '已到账', pending: '待支付', cancelled: '已取消',
  expired: '已超时', failed: '失败', refunded: '已退款',
}

const currentPage = computed<number>({
  get() { return Math.floor(filter.offset / filter.limit) + 1 },
  set(v) { filter.offset = (v - 1) * filter.limit; loadOrders() },
})

function priceYuan(fen: number) { return (fen / 100).toFixed(2) }

onMounted(() => {
  loadPackages()
  loadOrders()
})
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 充值" title="充值订单" accent-word="订单" accent="yellow">
      <template #extra>
        <NeonButton variant="outline" @click="() => { tab = 'packages'; loadPackages() }">套餐管理</NeonButton>
      </template>
    </PageHeader>

    <el-tabs v-model="tab">
      <!-- ===== 套餐管理 ===== -->
      <el-tab-pane label="套餐管理" name="packages">
        <div class="filter-bar" style="justify-content:space-between">
          <span style="font-size:13px;color:var(--el-text-color-secondary)">
            普通用户在 <b>个人中心 → 账单</b> 看到的是启用中的套餐。价格单位:分,积分单位:厘。
          </span>
          <NeonButton variant="yellow" @click="openCreatePkg">+ 新增套餐</NeonButton>
        </div>

        <el-table :data="packages" stripe v-loading="loadingPkg">
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column prop="name" label="名称" min-width="160" />
          <el-table-column label="价格" width="110">
            <template #default="{ row }">¥ {{ priceYuan(row.price_cny) }}</template>
          </el-table-column>
          <el-table-column label="基础积分" width="120">
            <template #default="{ row }">
              <span class="num-mono">{{ formatCredit(row.credits) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="赠送" width="110">
            <template #default="{ row }">
              <span class="num-mono">{{ formatCredit(row.bonus) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="sort" label="排序" width="80" />
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <StatusTag :variant="row.enabled ? 'active' : 'disabled'" dot>
                {{ row.enabled ? '启用' : '停用' }}
              </StatusTag>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="描述" show-overflow-tooltip />
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <div class="op">
                <a class="op-link" @click="openEditPkg(row)">编辑</a>
                <a class="op-link danger" @click="deletePkg(row)">删除</a>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ===== 订单流水 ===== -->
      <el-tab-pane label="订单流水" name="orders">
        <div class="filter-bar">
          <el-input-number v-model="filter.user_id" :min="1" placeholder="用户 ID" style="width:140px" />
          <el-select v-model="filter.status" placeholder="全部状态" clearable style="width:140px">
            <el-option label="全部" value="" />
            <el-option label="待支付" value="pending" />
            <el-option label="已到账" value="paid" />
            <el-option label="已取消" value="cancelled" />
            <el-option label="已超时" value="expired" />
            <el-option label="失败"   value="failed" />
          </el-select>
          <NeonButton variant="yellow" size="sm" :loading="loadingOrd"
            @click="() => { filter.offset = 0; loadOrders() }">查询</NeonButton>
        </div>

        <el-table :data="orders" stripe v-loading="loadingOrd">
          <el-table-column label="订单号" min-width="180">
            <template #default="{ row }">
              <code class="order-code">{{ row.out_trade_no }}</code>
            </template>
          </el-table-column>
          <el-table-column label="用户" width="120">
            <template #default="{ row }">
              <span class="num-mono">#{{ row.user_id }}</span>
            </template>
          </el-table-column>
          <el-table-column label="金额" width="100">
            <template #default="{ row }">
              <span class="num-mono">¥ {{ priceYuan(row.price_cny) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="积分" width="160">
            <template #default="{ row }">
              <span class="num-mono">{{ formatCredit(row.credits) }} + {{ formatCredit(row.bonus) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="pay_method" label="方式" width="90" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <StatusTag :variant="orderStatusVariant(row.status)" dot>
                {{ statusLabel[row.status] || row.status }}
              </StatusTag>
            </template>
          </el-table-column>
          <el-table-column label="上游单号" min-width="160">
            <template #default="{ row }">
              <span style="font-size:12px">{{ row.trade_no || '—' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="支付时间" width="170">
            <template #default="{ row }">{{ row.paid_at || '—' }}</template>
          </el-table-column>
          <el-table-column label="创建时间" width="170">
            <template #default="{ row }">{{ row.created_at }}</template>
          </el-table-column>
          <el-table-column label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <div class="op">
                <a v-if="row.status === 'pending'" class="op-link" @click="forcePaid(row)">手工入账</a>
                <span v-else style="color:var(--el-text-color-placeholder)">—</span>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <!-- Pagination bar -->
        <div class="pagination-bar">
          <span>共 <b>{{ total }}</b> 条订单</span>
          <el-pagination
            background
            layout="sizes, prev, pager, next"
            :total="total"
            v-model:current-page="currentPage"
            :page-sizes="[20, 50, 100]"
            v-model:page-size="filter.limit"
            @size-change="() => { filter.offset = 0; loadOrders() }"
          />
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 套餐编辑弹窗 -->
    <el-dialog v-model="pkgDialog.visible"
               :title="pkgDialog.mode === 'create' ? '新增套餐' : '编辑套餐'"
               width="520px">
      <el-form label-width="110px">
        <el-form-item label="名称">
          <el-input v-model="pkgDialog.form.name" />
        </el-form-item>
        <el-form-item label="售价(分)">
          <el-input-number v-model="pkgDialog.form.price_cny" :min="1" style="width:220px" />
          <span style="margin-left:8px;color:var(--el-text-color-secondary);font-size:13px">
            = ¥ {{ ((pkgDialog.form.price_cny || 0) / 100).toFixed(2) }}
          </span>
        </el-form-item>
        <el-form-item label="基础积分(厘)">
          <el-input-number v-model="pkgDialog.form.credits" :min="0" style="width:220px" />
        </el-form-item>
        <el-form-item label="赠送积分(厘)">
          <el-input-number v-model="pkgDialog.form.bonus" :min="0" style="width:220px" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="pkgDialog.form.sort" :min="0" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="pkgDialog.form.enabled" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="pkgDialog.form.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="pkgDialog.visible = false">取消</NeonButton>
        <NeonButton variant="yellow" @click="savePkg">保存</NeonButton>
      </template>
    </el-dialog>
  </div>
</template>

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

.num-mono { font-family: $f-mono; font-size: 14px; color: $n-ink; }

.order-code {
  background: $gray-200;
  padding: 1px 6px;
  border-radius: $r-sm;
  font-family: $f-mono;
  font-size: 12px;
  color: $n-ink;
}

.op { display: flex; gap: 4px; justify-content: flex-end; }
.op-link {
  color: $c-yellow-text;
  font-size: 13px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: $r-sm;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
  &:hover { background: rgba(255, 214, 0, 0.12); }
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
</style>
