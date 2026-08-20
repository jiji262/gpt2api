<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import * as statsApi from '@/api/stats'
import { nullVal } from '@/utils/format'

const loading = ref(false)
const rows = ref<statsApi.AdminKeyRow[]>([])
const total = ref(0)

const filter = reactive({
  q: '',
  user_id: undefined as number | undefined,
  enabled: '' as '' | '1' | '0',
  limit: 20,
  offset: 0,
})

async function load() {
  loading.value = true
  try {
    const d = await statsApi.listAdminKeys({
      q: filter.q || undefined,
      user_id: filter.user_id || undefined,
      enabled: filter.enabled || undefined,
      limit: filter.limit,
      offset: filter.offset,
    })
    rows.value = d.items
    total.value = d.total
  } finally { loading.value = false }
}

function reset() {
  filter.q = ''
  filter.user_id = undefined
  filter.enabled = ''
  filter.offset = 0
  load()
}

async function toggle(row: statsApi.AdminKeyRow) {
  const next = !row.enabled
  await ElMessageBox.confirm(
    `确认${next ? '启用' : '禁用'} #${row.id}${row.name}(用户 ${row.user_email})?`,
    '操作确认',
    { type: 'warning' },
  )
  await statsApi.setAdminKeyEnabled(row.id, next)
  ElMessage.success('已更新')
  load()
}

function usagePercent(r: statsApi.AdminKeyRow) {
  if (!r.quota_limit) return 0
  return Math.min(100, Math.round((r.quota_used / r.quota_limit) * 100))
}

const currentPage = computed<number>({
  get() { return Math.floor(filter.offset / filter.limit) + 1 },
  set(v) { filter.offset = (v - 1) * filter.limit },
})

onMounted(load)
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / Keys" title="全局 Keys" accent-word="Keys" accent="yellow">
      <template #extra>
        <!-- no create action for admin keys — header extra left empty intentionally -->
      </template>
    </PageHeader>

    <!-- Filter bar -->
    <div class="filter-bar">
      <el-input v-model="filter.q" placeholder="按名称 / prefix / 邮箱" style="width:240px" clearable
                @keyup.enter="load" />
      <el-input-number v-model="filter.user_id" :min="1" placeholder="用户 ID" style="width:140px" />
      <el-select v-model="filter.enabled" placeholder="状态" style="width:120px" clearable>
        <el-option label="全部" value="" />
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="0" />
      </el-select>
      <NeonButton variant="yellow" size="sm" :disabled="loading" @click="load">
        <el-icon><Search /></el-icon> 查询
      </NeonButton>
      <NeonButton variant="ghost" size="sm" @click="reset">重置</NeonButton>
    </div>

    <!-- Table -->
    <el-table :data="rows" stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="归属用户" min-width="220">
        <template #default="{ row }">
          <div class="u-cell">
            <UserAvatar :name="row.user_email || row.owner_email || String(row.user_id)" size="md" />
            <div class="u-meta">
              <div class="u-email">{{ row.user_email || '—' }}</div>
              <div class="u-sub">#{{ row.user_id }}</div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
      <el-table-column label="Prefix" width="150">
        <template #default="{ row }">
          <span class="num-mono">{{ row.key_prefix }}...</span>
        </template>
      </el-table-column>
      <el-table-column label="额度" width="220">
        <template #default="{ row }">
          <div v-if="row.quota_limit > 0">
            <el-progress :percentage="usagePercent(row)"
                         :status="usagePercent(row) >= 90 ? 'exception' : undefined"
                         :stroke-width="8" />
            <div style="font-size:12px;color:var(--el-text-color-secondary);margin-top:2px">
              {{ row.quota_used }} / {{ row.quota_limit }}
            </div>
          </div>
          <span v-else style="color:var(--el-text-color-secondary)">不限</span>
        </template>
      </el-table-column>
      <el-table-column label="限速" width="120">
        <template #default="{ row }">
          <div style="font-size:12px">rpm: {{ row.rpm || '∞' }}</div>
          <div style="font-size:12px">tpm: {{ row.tpm || '∞' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="最近使用" width="170">
        <template #default="{ row }">
          <div style="font-size:12px">{{ nullVal(row.last_used_at) || '—' }}</div>
          <div style="font-size:11px;color:var(--el-text-color-secondary)">
            {{ row.last_used_ip || '' }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <StatusTag :variant="row.enabled ? 'active' : 'disabled'" dot>
            {{ row.enabled ? '启用' : '禁用' }}
          </StatusTag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <div class="op">
            <a class="op-link" :class="{ danger: row.enabled }" @click="toggle(row)">
              {{ row.enabled ? '禁用' : '启用' }}
            </a>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- Pagination -->
    <div class="pagination-bar">
      <span>共 <b>{{ total }}</b> 条记录</span>
      <el-pagination
        background
        layout="sizes, prev, pager, next"
        :total="total"
        v-model:current-page="currentPage"
        :page-sizes="[20, 50, 100]"
        v-model:page-size="filter.limit"
        @size-change="() => { filter.offset = 0; load() }"
        @current-change="(p: number) => { filter.offset = (p - 1) * filter.limit; load() }"
      />
    </div>
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

.u-cell { display: flex; align-items: center; gap: 10px; }
.u-meta { display: flex; flex-direction: column; gap: 2px; }
.u-email { font-weight: 700; color: $n-ink; font-size: 14px; }
.u-sub { font-size: 12px; color: $gray-500; }

.num-mono { font-family: $f-mono; font-size: 13px; color: $n-ink; }

.op { display: flex; gap: 4px; justify-content: flex-end; }
.op-link {
  font-size: 13px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: $r-sm;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
  color: $c-yellow-text;
  &:hover { background: rgba(255, 214, 0, 0.12); }
  &.danger { color: $c-orange-text; &:hover { background: rgba(255, 107, 53, 0.08); } }
  &.disabled { pointer-events: none; opacity: 0.4; }
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
