<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import * as adminApi from '@/api/admin'
import { formatCredit, formatDateTime } from '@/utils/format'
import { useUserStore } from '@/stores/user'
import EmptyUsers from '@/assets/illustrations/empty-users.svg?component'

const store = useUserStore()

const loading = ref(false)
const filter = reactive<adminApi.UserFilter>({
  q: '', role: '', status: '', group_id: undefined,
  limit: 20, offset: 0,
})
const total = ref(0)
const rows = ref<adminApi.AdminUser[]>([])
const groups = ref<adminApi.Group[]>([])

async function fetchList() {
  loading.value = true
  try {
    const data = await adminApi.listUsers({
      ...filter,
      group_id: filter.group_id || undefined,
    })
    rows.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function fetchGroups() {
  try {
    const g = await adminApi.listGroups()
    groups.value = g.items
  } catch { /* noop */ }
}

function groupName(id: number) {
  return groups.value.find((g) => g.id === id)?.name || `#${id}`
}

function resetFilter() {
  filter.q = ''
  filter.role = ''
  filter.status = ''
  filter.group_id = undefined
  filter.offset = 0
  fetchList()
}

// ---- 编辑 ----
const editDlg = ref(false)
const editingRow = ref<adminApi.AdminUser | null>(null)
const editForm = reactive({ nickname: '', role: '', status: '', group_id: 1 })

function openEdit(row: adminApi.AdminUser) {
  editingRow.value = row
  editForm.nickname = row.nickname
  editForm.role = row.role
  editForm.status = row.status
  editForm.group_id = row.group_id
  editDlg.value = true
}
async function onSaveEdit() {
  if (!editingRow.value) return
  await adminApi.patchUser(editingRow.value.id, { ...editForm })
  ElMessage.success('保存成功')
  editDlg.value = false
  fetchList()
}

// ---- 重置密码 ----
const pwdDlg = ref(false)
const pwdForm = reactive({ uid: 0, newPwd: '', adminPwd: '' })
function openReset(row: adminApi.AdminUser) {
  pwdForm.uid = row.id
  pwdForm.newPwd = ''
  pwdForm.adminPwd = ''
  pwdDlg.value = true
}
async function onResetSubmit() {
  if (pwdForm.newPwd.length < 6) return ElMessage.warning('新密码至少 6 位')
  if (!pwdForm.adminPwd) return ElMessage.warning('请输入管理员密码')
  await adminApi.resetUserPassword(pwdForm.uid, pwdForm.newPwd, pwdForm.adminPwd)
  ElMessage.success('已重置')
  pwdDlg.value = false
}

// ---- 调账 ----
const adjustDlg = ref(false)
const adjustForm = reactive({ uid: 0, delta: 0, remark: '', ref_id: '', adminPwd: '' })
function openAdjust(row: adminApi.AdminUser) {
  adjustForm.uid = row.id
  adjustForm.delta = 0
  adjustForm.remark = ''
  adjustForm.ref_id = ''
  adjustForm.adminPwd = ''
  adjustDlg.value = true
}
async function onAdjustSubmit() {
  if (!adjustForm.delta) return ElMessage.warning('金额不能为 0')
  if (!adjustForm.remark) return ElMessage.warning('请填备注')
  if (!adjustForm.adminPwd) return ElMessage.warning('请输入管理员密码')
  await adminApi.adjustCredit(adjustForm.uid, {
    delta: adjustForm.delta,
    remark: adjustForm.remark,
    ref_id: adjustForm.ref_id,
  }, adjustForm.adminPwd)
  ElMessage.success('调账成功')
  adjustDlg.value = false
  fetchList()
}

// ---- 删除 ----
async function onDelete(row: adminApi.AdminUser) {
  if (row.id === store.user?.id) return ElMessage.warning('不能删除自己')
  const { value: pwd } = await ElMessageBox.prompt(
    `确认删除用户 ${row.email}?此操作会将账号标记为已删除并封禁。请输入你的管理员密码:`,
    '删除用户',
    {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      inputType: 'password',
      type: 'warning',
      inputPlaceholder: '管理员密码',
    },
  ).catch(() => ({ value: null }))
  if (!pwd) return
  await adminApi.deleteUser(row.id, pwd)
  ElMessage.success('已删除')
  fetchList()
}

// ---- 流水 ----
const logsDlg = ref(false)
const logs = ref<adminApi.CreditLog[]>([])
const logLoading = ref(false)
async function openLogs(row: adminApi.AdminUser) {
  logsDlg.value = true
  logLoading.value = true
  try {
    const data = await adminApi.listCreditLogs(row.id, 100, 0)
    logs.value = data.items
  } finally {
    logLoading.value = false
  }
}

const canEdit = computed(() => store.hasPerm('user:write'))
const canCredit = computed(() => store.hasPerm('user:credit'))

// ---- 分页 ----
const currentPage = computed({
  get: () => Math.floor((filter.offset || 0) / (filter.limit || 20)) + 1,
  set: (p: number) => {
    filter.offset = (p - 1) * (filter.limit || 20)
    fetchList()
  },
})

onMounted(() => { fetchGroups(); fetchList() })
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 用户" title="用户管理" accent-word="管理" accent="pink">
      <template #extra>
        <NeonButton variant="outline" @click="fetchList">
          <el-icon><Refresh /></el-icon> 刷新
        </NeonButton>
      </template>
    </PageHeader>

    <!-- Filter bar -->
    <div class="filter-bar">
      <el-input
        v-model="filter.q"
        placeholder="搜索邮箱 / 昵称 / ID"
        style="width: 220px"
        clearable
        @keyup.enter="fetchList"
      />
      <el-select v-model="filter.role" placeholder="全部角色" style="width: 140px" clearable>
        <el-option label="管理员" value="admin" />
        <el-option label="普通用户" value="user" />
      </el-select>
      <el-select v-model="filter.status" placeholder="全部状态" style="width: 140px" clearable>
        <el-option label="启用" value="active" />
        <el-option label="禁用" value="disabled" />
      </el-select>
      <el-select v-model="filter.group_id" placeholder="全部分组" style="width: 160px" clearable>
        <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
      </el-select>
      <NeonButton variant="pink" size="sm" @click="fetchList">搜索</NeonButton>
      <NeonButton variant="ghost" size="sm" @click="resetFilter">重置</NeonButton>
    </div>

    <!-- Table -->
    <el-table :data="rows" v-loading="loading" class="user-table">
      <el-table-column label="用户" min-width="260">
        <template #default="{ row }">
          <div class="u-cell">
            <UserAvatar :name="row.nickname || row.email" size="md" />
            <div class="u-meta">
              <div class="u-email">{{ row.email }}</div>
              <div class="u-sub">ID #{{ row.id }}<template v-if="row.nickname"> · {{ row.nickname }}</template></div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="角色" width="110">
        <template #default="{ row }">
          <StatusTag :variant="row.role === 'admin' ? 'admin' : (row.role === 'pro' ? 'pro' : 'free')">
            {{ row.role }}
          </StatusTag>
        </template>
      </el-table-column>
      <el-table-column label="余额 / 冻结" min-width="160">
        <template #default="{ row }">
          <div class="num-mono">{{ formatCredit(row.credit_balance) }}</div>
          <div class="u-sub">冻结 {{ formatCredit(row.credit_frozen) }}</div>
        </template>
      </el-table-column>
      <el-table-column label="分组" width="120">
        <template #default="{ row }">{{ groupName(row.group_id) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <StatusTag :variant="row.status === 'active' ? 'active' : 'disabled'" dot>
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </StatusTag>
        </template>
      </el-table-column>
      <el-table-column label="最近登录" min-width="160">
        <template #default="{ row }">
          <div class="u-sub">
            <div>{{ row.last_login_at ? formatDateTime(row.last_login_at) : '—' }}</div>
            <div v-if="row.last_login_ip">{{ row.last_login_ip }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="操作" align="right" width="260" fixed="right">
        <template #default="{ row }">
          <div class="op">
            <a class="op-link" :class="{ disabled: !canEdit }" @click="canEdit && openEdit(row)">编辑</a>
            <a class="op-link" :class="{ disabled: !canCredit }" @click="canCredit && openAdjust(row)">调账</a>
            <a class="op-link" :class="{ disabled: !canEdit }" @click="canEdit && openReset(row)">重置</a>
            <a class="op-link" @click="openLogs(row)">流水</a>
            <a class="op-link danger" :class="{ disabled: !canEdit }" @click="canEdit && onDelete(row)">删除</a>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- Empty state -->
    <EmptyState
      v-if="!loading && total === 0"
      title="没有匹配的用户"
      desc="换个筛选条件试试。"
    >
      <template #illustration><EmptyUsers /></template>
    </EmptyState>

    <!-- Pagination bar -->
    <div class="pagination-bar">
      <span class="pg-info">共 <b>{{ total }}</b> 位用户</span>
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="filter.limit"
        :page-sizes="[20, 50, 100]"
        :total="total"
        layout="sizes, prev, pager, next"
        @size-change="(s: number) => { filter.limit = s; filter.offset = 0; fetchList() }"
      />
    </div>

    <!-- ============ DIALOGS ============ -->

    <!-- 编辑用户 -->
    <el-dialog v-model="editDlg" title="编辑用户" width="480px" :close-on-click-modal="false">
      <el-form :model="editForm" label-width="90px">
        <el-form-item label="昵称">
          <el-input v-model="editForm.nickname" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="editForm.role" style="width:100%">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="editForm.status" style="width:100%">
            <el-option label="正常" value="active" />
            <el-option label="封禁" value="banned" />
          </el-select>
        </el-form-item>
        <el-form-item label="分组">
          <el-select v-model="editForm.group_id" style="width:100%">
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="editDlg = false">取消</NeonButton>
        <NeonButton variant="pink" @click="onSaveEdit">保存</NeonButton>
      </template>
    </el-dialog>

    <!-- 重置密码 -->
    <el-dialog v-model="pwdDlg" title="重置密码" width="420px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px"
                title="该操作会强制写入新密码,旧登录 token 仍短时可用,请提醒用户改密并退出重登。" />
      <el-form label-width="110px">
        <el-form-item label="新密码">
          <el-input v-model="pwdForm.newPwd" type="password" show-password placeholder="≥ 6 位" />
        </el-form-item>
        <el-form-item label="管理员密码">
          <el-input v-model="pwdForm.adminPwd" type="password" show-password placeholder="二次确认" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="pwdDlg = false">取消</NeonButton>
        <NeonButton variant="pink" @click="onResetSubmit">确认重置</NeonButton>
      </template>
    </el-dialog>

    <!-- 调账 -->
    <el-dialog v-model="adjustDlg" title="积分调账" width="440px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px"
                title="正数为加款,负数为扣款;扣款不会把余额扣成负数。单位:厘(1 积分 = 10000 厘)" />
      <el-form label-width="110px">
        <el-form-item label="金额(厘)">
          <el-input-number v-model="adjustForm.delta" :step="10000" style="width:100%" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="adjustForm.remark" maxlength="200" show-word-limit />
        </el-form-item>
        <el-form-item label="关联单号">
          <el-input v-model="adjustForm.ref_id" placeholder="选填,订单号/工单号" />
        </el-form-item>
        <el-form-item label="管理员密码">
          <el-input v-model="adjustForm.adminPwd" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="adjustDlg = false">取消</NeonButton>
        <NeonButton variant="pink" @click="onAdjustSubmit">确认</NeonButton>
      </template>
    </el-dialog>

    <!-- 积分流水 -->
    <el-dialog v-model="logsDlg" title="积分流水" width="820px">
      <el-table v-loading="logLoading" :data="logs" max-height="480" size="small">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="type" label="类型" width="120" />
        <el-table-column label="金额" width="140">
          <template #default="{ row }">
            <span :style="{ color: row.amount >= 0 ? 'var(--c-green-text)' : 'var(--c-orange-text)' }">
              {{ row.amount >= 0 ? '+' : '' }}{{ formatCredit(row.amount) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="余额" width="110">
          <template #default="{ row }">{{ formatCredit(row.balance_after) }}</template>
        </el-table-column>
        <el-table-column prop="ref_id" label="关联" min-width="120" />
        <el-table-column prop="remark" label="备注" min-width="180" show-overflow-tooltip />
        <el-table-column prop="created_at" label="时间" width="170" />
      </el-table>
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

.user-table { /* Element override already handles base table styling */ }

.u-cell { display: flex; align-items: center; gap: 12px; }
.u-meta { display: flex; flex-direction: column; gap: 2px; }
.u-email { font-weight: 700; color: $n-ink; font-size: 14px; }
.u-sub { font-size: 12px; color: $gray-500; }

.num-mono { font-family: $f-mono; font-size: 14px; color: $n-ink; }

.op { display: flex; gap: 4px; justify-content: flex-end; flex-wrap: wrap; }
.op-link {
  color: $c-pink;
  font-size: 13px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: $r-sm;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
  &:hover { background: rgba(255, 61, 148, 0.08); }
  &.danger { color: $c-orange-text; &:hover { background: rgba(255, 107, 53, 0.08); } }
  &.disabled { opacity: 0.4; cursor: not-allowed; &:hover { background: none; } }
}

.pagination-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
  font-size: var(--fs-sm);
  color: $gray-500;
  .pg-info b { color: $n-ink; font-weight: 700; }
}
</style>
