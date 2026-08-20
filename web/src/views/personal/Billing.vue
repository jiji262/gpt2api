<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as rechargeApi from '@/api/recharge'
import { formatCredit } from '@/utils/format'
import { useUserStore } from '@/stores/user'
import EmptyRecharges from '@/assets/illustrations/empty-recharges.svg?component'

const userStore = useUserStore()
const packages = ref<rechargeApi.Package[]>([])
const channelEnabled = ref(false)
const orders = ref<rechargeApi.Order[]>([])
const total = ref(0)
const paging = reactive({ limit: 10, offset: 0, status: '' as '' | 'pending' | 'paid' | 'cancelled' | 'expired' })
const loadingPkg = ref(false)
const loadingOrder = ref(false)

async function loadPackages() {
  loadingPkg.value = true
  try {
    const d = await rechargeApi.listMyPackages()
    packages.value = d.items
    channelEnabled.value = d.enabled
  } finally { loadingPkg.value = false }
}

async function loadOrders() {
  loadingOrder.value = true
  try {
    const d = await rechargeApi.listMyOrders({
      limit: paging.limit,
      offset: paging.offset,
      status: paging.status || undefined,
    })
    orders.value = d.items
    total.value = d.total
  } finally { loadingOrder.value = false }
}

/**
 * 下单 -> 打开 pay_url(上游收银台)
 * 返回后用户点"刷新"即可拉到最新订单状态(支付回调已异步把 pending -> paid)。
 */
async function buy(pkg: rechargeApi.Package, payType?: string) {
  if (!channelEnabled.value) {
    ElMessage.warning('支付通道未配置,请联系管理员')
    return
  }
  try {
    const order = await rechargeApi.createOrder(pkg.id, payType)
    if (!order.pay_url) {
      ElMessage.error('支付链接生成失败')
      return
    }
    window.open(order.pay_url, '_blank', 'noopener,noreferrer')
    ElMessageBox.alert(
      `订单号:${order.out_trade_no}\n\n支付完成后请返回本页并点击"刷新"按钮查看到账状态。`,
      '已跳转支付',
      { confirmButtonText: '去刷新订单', callback: () => { paging.offset = 0; loadOrders() } },
    )
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  }
}

async function cancel(o: rechargeApi.Order) {
  await ElMessageBox.confirm(`确认取消订单 ${o.out_trade_no}?`, '取消订单', { type: 'warning' })
  await rechargeApi.cancelMyOrder(o.id)
  ElMessage.success('已取消')
  loadOrders()
}

const statusColor: Record<string, 'success' | 'info' | 'warning' | 'danger'> = {
  paid: 'success', pending: 'warning', cancelled: 'info', expired: 'info', failed: 'danger',
}
const statusLabel: Record<string, string> = {
  paid: '已到账', pending: '待支付', cancelled: '已取消', expired: '已超时', failed: '失败',
}

// StatusTag variant mapping for orders
function orderStatusVariant(s: string): 'success' | 'warning' | 'info' | 'danger' | 'free' {
  const map: Record<string, 'success' | 'warning' | 'info' | 'danger' | 'free'> = {
    paid: 'success',
    pending: 'warning',
    cancelled: 'free',
    expired: 'info',
    failed: 'danger',
  }
  return map[s] || 'free'
}

const currentPage = computed<number>({
  get() { return Math.floor(paging.offset / paging.limit) + 1 },
  set(v) { paging.offset = (v - 1) * paging.limit; loadOrders() },
})

function priceYuan(fen: number) { return (fen / 100).toFixed(2) }
function openPayUrl(url: string) { window.open(url, '_blank', 'noopener,noreferrer') }

onMounted(() => {
  loadPackages()
  loadOrders()
})
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="个人中心" title="账单与充值" accent-word="充值" accent="green">
      <template #extra>
        <NeonButton variant="ghost" @click="userStore.fetchMe()">
          <el-icon><Refresh /></el-icon> 刷新余额
        </NeonButton>
      </template>
    </PageHeader>

    <!-- 大余额卡 -->
    <div class="balance-card">
      <div class="balance-card__label">当前可用积分</div>
      <div class="balance-card__value">{{ formatCredit(userStore.user?.credit_balance) }}</div>
      <div class="balance-card__sub">
        冻结 {{ formatCredit(userStore.user?.credit_frozen) }} 积分
      </div>
    </div>

    <!-- 套餐选择 -->
    <div class="panel" style="margin-bottom:20px">
      <div class="panel__head">
        <h4>选择充值套餐</h4>
        <StatusTag v-if="!channelEnabled" variant="warning">支付通道未配置</StatusTag>
      </div>

      <el-empty v-if="!loadingPkg && packages.length === 0" description="暂无可用套餐" />
      <div class="pkg-grid" v-loading="loadingPkg">
        <div v-for="p in packages" :key="p.id" class="pkg-card">
          <div class="pkg-card__name">{{ p.name }}</div>
          <div class="pkg-card__price">
            ¥ <span class="pkg-card__price-num">{{ priceYuan(p.price_cny) }}</span>
          </div>
          <div class="pkg-card__credit">
            到账 <b>{{ formatCredit(p.credits) }}</b> 积分
            <span v-if="p.bonus > 0" class="pkg-card__bonus">+赠送 {{ formatCredit(p.bonus) }}</span>
          </div>
          <div class="pkg-card__desc">{{ p.description || '—' }}</div>
          <div class="pkg-card__actions">
            <NeonButton variant="cyan" size="sm" :disabled="!channelEnabled" @click="buy(p, 'alipay')">
              支付宝
            </NeonButton>
            <NeonButton variant="green" size="sm" :disabled="!channelEnabled" @click="buy(p, 'wxpay')">
              微信
            </NeonButton>
          </div>
        </div>
      </div>
    </div>

    <!-- 订单列表 -->
    <div class="panel">
      <div class="panel__head">
        <h4>我的订单</h4>
        <div class="panel__head-tools">
          <el-select v-model="paging.status" placeholder="状态" clearable style="width:130px"
                     @change="() => { paging.offset = 0; loadOrders() }">
            <el-option label="全部" value="" />
            <el-option label="待支付" value="pending" />
            <el-option label="已到账" value="paid" />
            <el-option label="已取消" value="cancelled" />
            <el-option label="已超时" value="expired" />
          </el-select>
          <NeonButton variant="ghost" @click="loadOrders">
            <el-icon><Refresh /></el-icon> 刷新
          </NeonButton>
        </div>
      </div>

      <el-table
        v-if="orders.length > 0 || loadingOrder"
        :data="orders"
        stripe
        v-loading="loadingOrder"
      >
        <el-table-column label="订单号" min-width="170">
          <template #default="{ row }">
            <code class="order-no">{{ row.out_trade_no }}</code>
          </template>
        </el-table-column>
        <el-table-column label="套餐" min-width="120">
          <template #default="{ row }">{{ row.remark || `#${row.package_id}` }}</template>
        </el-table-column>
        <el-table-column label="金额" width="100">
          <template #default="{ row }">¥ {{ priceYuan(row.price_cny) }}</template>
        </el-table-column>
        <el-table-column label="积分" width="130">
          <template #default="{ row }">
            {{ formatCredit(row.credits + row.bonus) }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <StatusTag
              :variant="orderStatusVariant(row.status)"
              :dot="row.status === 'pending' || row.status === 'paid' || row.status === 'failed'"
            >
              {{ statusLabel[row.status] || row.status }}
            </StatusTag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ row.created_at }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending' && row.pay_url" type="primary" link
                       @click="() => openPayUrl(row.pay_url!)">继续支付</el-button>
            <el-button v-if="row.status === 'pending'" type="danger" link @click="cancel(row)">
              取消
            </el-button>
            <span v-if="row.status !== 'pending'" class="muted">—</span>
          </template>
        </el-table-column>
      </el-table>

      <EmptyState
        v-else
        title="暂无充值记录"
        desc="充值后会在这里看到充值订单和账变流水。"
      >
        <template #illustration>
          <EmptyRecharges style="width:180px;height:140px" />
        </template>
        <template #action>
          <NeonButton variant="green" @click="loadPackages">立即充值 →</NeonButton>
        </template>
      </EmptyState>

      <div class="pagination-bar" v-if="total > 0">
        <span>共 <b>{{ total }}</b> 条</span>
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          v-model:current-page="currentPage"
          :page-size="paging.limit"
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

/* 大余额卡 */
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
    position: absolute;
    left: 0; top: 0; bottom: 0;
    width: 6px;
    background: $c-green;
  }

  &__label {
    font-size: var(--fs-xs);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: $gray-500;
    font-weight: 700;
  }

  &__value {
    font-size: 56px;
    font-weight: 800;
    letter-spacing: -0.03em;
    color: $n-ink;
    margin: 8px 0 4px;
    line-height: 1;
    font-family: $f-sans;
  }

  &__sub {
    font-size: var(--fs-sm);
    color: $gray-500;
  }
}

/* Panel */
.panel {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 22px 24px;

  &__head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;

    h4 {
      margin: 0;
      font-size: 16px;
      font-weight: 800;
      letter-spacing: -0.01em;
      color: $n-ink;
    }
  }

  &__head-tools {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

/* 套餐网格 */
.pkg-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.pkg-card {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 20px;
  transition: transform 0.15s, box-shadow 0.15s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: $sh-2;
    border-color: $c-green;
  }

  &__name {
    font-size: 15px;
    font-weight: 700;
    color: $n-ink;
    margin-bottom: 10px;
  }

  &__price {
    font-size: 14px;
    color: $c-orange-text;
    margin-bottom: 6px;
  }

  &__price-num {
    font-size: 28px;
    font-weight: 800;
    letter-spacing: -0.02em;
  }

  &__credit {
    font-size: 14px;
    color: $n-ink;
    margin-bottom: 4px;

    b { font-weight: 700; }
  }

  &__bonus {
    color: $c-green-text;
    font-weight: 700;
    margin-left: 6px;
  }

  &__desc {
    font-size: 12px;
    color: $gray-500;
    margin: 10px 0 14px;
    min-height: 36px;
  }

  &__actions {
    display: flex;
    gap: 8px;
  }
}

/* 订单表 */
.order-no {
  font-family: $f-mono;
  font-size: 12px;
  background: $gray-100;
  padding: 2px 6px;
  border-radius: $r-sm;
  color: $gray-700;
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

.muted { color: $gray-500; font-size: 13px; }

@media (max-width: 900px) {
  .pkg-grid { grid-template-columns: 1fr; }
  .balance-card__value { font-size: 38px; }
}
</style>
