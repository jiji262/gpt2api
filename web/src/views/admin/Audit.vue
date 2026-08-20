<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import * as adminApi from '@/api/admin'
import EmptyAudit from '@/assets/illustrations/empty-audit.svg?component'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const filter = reactive<adminApi.AuditFilter>({ action: '', actor_id: undefined, limit: 50, offset: 0 })
const items = ref<adminApi.AuditLog[]>([])
const total = ref(0)

async function load() {
  loading.value = true
  try {
    const d = await adminApi.listAudit({ ...filter, actor_id: filter.actor_id || undefined })
    items.value = d.items
    total.value = d.total
  } finally { loading.value = false }
}

const detailDlg = ref(false)
const detailRow = ref<adminApi.AuditLog | null>(null)
function openDetail(row: adminApi.AuditLog) {
  detailRow.value = row
  detailDlg.value = true
}

type StatusVariant = 'free' | 'danger' | 'warning' | 'success' | 'info'
function statusVariant(code: number | undefined): StatusVariant {
  if (!code) return 'free'
  if (code >= 500) return 'danger'
  if (code >= 400) return 'warning'
  if (code >= 200 && code < 300) return 'success'
  return 'free'
}

onMounted(load)
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 审计" title="审计日志" accent-word="日志" accent="orange" />

    <div class="filter-bar">
      <el-input
        v-model="filter.action"
        placeholder="action（如 users.update）"
        clearable
        style="width:220px"
      />
      <el-input-number
        v-model="filter.actor_id"
        placeholder="操作者 ID"
        :min="0"
        style="width:170px"
      />
      <NeonButton variant="orange" size="sm" @click="load">查询</NeonButton>
      <NeonButton
        variant="ghost"
        size="sm"
        @click="() => { filter.action = ''; filter.actor_id = undefined; filter.offset = 0; load() }"
      >重置</NeonButton>
    </div>

    <el-table v-loading="loading" :data="items" stripe style="width:100%">
      <el-table-column prop="id" label="ID" width="72" />

      <el-table-column label="操作者" min-width="180">
        <template #default="{ row }">
          <div class="actor-cell">
            <UserAvatar :name="row.actor_email || String(row.actor_id)" size="sm" />
            <div class="actor-info">
              <div class="actor-email">{{ row.actor_email || '-' }}</div>
              <div class="actor-id">ID: {{ row.actor_id }}</div>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="Action" min-width="180">
        <template #default="{ row }">
          <span class="num-mono">{{ row.action }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="method" label="Method" width="88" />

      <el-table-column label="Status" width="90">
        <template #default="{ row }">
          <StatusTag
            :variant="statusVariant(row.status_code)"
            :dot="row.status_code >= 400"
          >{{ row.status_code }}</StatusTag>
        </template>
      </el-table-column>

      <el-table-column prop="path" label="Path" min-width="200" show-overflow-tooltip />
      <el-table-column prop="target" label="Target" min-width="100" show-overflow-tooltip />
      <el-table-column prop="ip" label="IP" width="120" />

      <el-table-column label="时间" width="165">
        <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
      </el-table-column>

      <el-table-column label="" width="70" fixed="right">
        <template #default="{ row }">
          <div class="op">
            <span class="op-link" @click="openDetail(row)">详情</span>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <EmptyState
      v-if="!loading && total === 0"
      title="没有审计记录"
      desc="调整筛选条件，或检查日期范围"
    >
      <template #illustration><EmptyAudit /></template>
    </EmptyState>

    <div v-if="total > 0" class="pagination-bar">
      <span>共 <b>{{ total }}</b> 条记录</span>
      <el-pagination
        :current-page="Math.floor((filter.offset || 0) / (filter.limit || 50)) + 1"
        @current-change="(p: number) => { filter.offset = (p - 1) * (filter.limit || 50); load() }"
        :page-size="filter.limit"
        :total="total"
        :page-sizes="[50, 100, 200]"
        @size-change="(s: number) => { filter.limit = s; filter.offset = 0; load() }"
        layout="sizes, prev, pager, next"
      />
    </div>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailDlg" title="审计详情" width="620px">
      <el-descriptions v-if="detailRow" :column="2" border size="small">
        <el-descriptions-item label="ID">{{ detailRow.id }}</el-descriptions-item>
        <el-descriptions-item label="时间">{{ formatDateTime(detailRow.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="Action">{{ detailRow.action }}</el-descriptions-item>
        <el-descriptions-item label="Status">{{ detailRow.status_code }}</el-descriptions-item>
        <el-descriptions-item label="Method">{{ detailRow.method }}</el-descriptions-item>
        <el-descriptions-item label="Path">{{ detailRow.path }}</el-descriptions-item>
        <el-descriptions-item label="Actor">{{ detailRow.actor_email }} (#{{ detailRow.actor_id }})</el-descriptions-item>
        <el-descriptions-item label="IP">{{ detailRow.ip }}</el-descriptions-item>
        <el-descriptions-item label="UA" :span="2">{{ detailRow.ua }}</el-descriptions-item>
        <el-descriptions-item label="Target" :span="2">{{ detailRow.target || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Meta" :span="2">
          <pre class="meta">{{ typeof detailRow.meta === 'string' ? detailRow.meta : JSON.stringify(detailRow.meta, null, 2) }}</pre>
        </el-descriptions-item>
      </el-descriptions>
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

.pagination-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
  font-size: var(--fs-sm);
  color: $gray-500;
  b { color: $n-ink; font-weight: 700; }
}

.num-mono {
  font-family: $f-mono;
  font-size: 13px;
  color: $n-ink;
}

.op {
  display: flex;
  gap: 4px;
  justify-content: flex-end;
}

.op-link {
  font-size: 13px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: $r-sm;
  cursor: pointer;
  user-select: none;
  transition: background .15s;
  color: $c-orange-text;
  &:hover { background: rgba(255, 107, 53, 0.08); }
}

.actor-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.actor-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.actor-email {
  font-size: 13px;
  font-weight: 600;
  color: $n-ink;
}

.actor-id {
  font-size: 11px;
  color: $gray-500;
  font-family: $f-mono;
}

.meta {
  font-family: $f-mono;
  font-size: 12px;
  background: #f7f8fa;
  padding: 8px;
  border-radius: 4px;
  max-height: 280px;
  overflow: auto;
  margin: 0;
}
</style>
