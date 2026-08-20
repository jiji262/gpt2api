<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as backupApi from '@/api/backup'
import { formatBytes, formatDateTime } from '@/utils/format'

const loading = ref(false)
const items = ref<backupApi.BackupFile[]>([])
const total = ref(0)
const allowRestore = ref(false)
const maxUploadMB = ref(512)
const page = reactive({ limit: 50, offset: 0 })
const creating = ref(false)

async function load() {
  loading.value = true
  try {
    const d = await backupApi.listBackups(page.limit, page.offset)
    items.value = d.items
    total.value = d.total
    allowRestore.value = d.allow_restore
    maxUploadMB.value = d.max_upload_mb
  } finally { loading.value = false }
}

async function onCreate() {
  creating.value = true
  try {
    await backupApi.createBackup(true)
    ElMessage.success('备份已创建')
    load()
  } finally { creating.value = false }
}

function download(row: backupApi.BackupFile) {
  return backupApi.downloadBackup(row.backup_id, row.file_name)
}

async function onDelete(row: backupApi.BackupFile) {
  const { value: pwd } = await ElMessageBox.prompt(
    `确认删除 ${row.file_name}?请输入管理员密码:`, '删除备份',
    { inputType: 'password', confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' },
  ).catch(() => ({ value: null }))
  if (!pwd) return
  await backupApi.deleteBackup(row.backup_id, pwd)
  ElMessage.success('已删除')
  load()
}

async function onRestore(row: backupApi.BackupFile) {
  if (!allowRestore.value) return ElMessage.error('后端未启用恢复功能')
  await ElMessageBox.confirm(
    `恢复会覆盖当前数据库!此操作不可撤销。你已理解风险并希望继续?`,
    '恢复数据库', { type: 'error', confirmButtonText: '我确认继续', cancelButtonText: '取消' },
  )
  const { value: pwd } = await ElMessageBox.prompt(
    `最后一次确认:输入你的管理员密码。`, '恢复数据库',
    { inputType: 'password', confirmButtonText: '执行恢复', cancelButtonText: '取消', type: 'error' },
  ).catch(() => ({ value: null }))
  if (!pwd) return
  ElMessage.info('正在恢复,请稍候…')
  await backupApi.restoreBackup(row.backup_id, pwd)
  ElMessage.success('恢复成功,请刷新页面')
}

// ---- 上传 ----
const uploadDlg = ref(false)
const uploadFile = ref<File | null>(null)
const uploadPwd = ref('')
const uploadPct = ref(0)
const uploading = ref(false)

function pickFile(e: Event) {
  const t = e.target as HTMLInputElement
  uploadFile.value = t.files?.[0] || null
}
async function doUpload() {
  if (!uploadFile.value) return ElMessage.warning('请选择 .sql.gz 文件')
  if (!uploadPwd.value) return ElMessage.warning('请输入管理员密码')
  uploading.value = true
  uploadPct.value = 0
  try {
    await backupApi.uploadBackup(uploadFile.value, uploadPwd.value, (p) => (uploadPct.value = p))
    ElMessage.success('上传成功')
    uploadDlg.value = false
    uploadFile.value = null
    uploadPwd.value = ''
    load()
  } finally { uploading.value = false }
}

function backupStatusVariant(status: string) {
  if (status === 'ready') return 'success'
  if (status === 'failed') return 'danger'
  return 'free'
}

onMounted(load)
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 备份" title="数据备份" accent-word="备份" accent="green">
      <template #extra>
        <NeonButton variant="outline" size="sm" @click="uploadDlg = true">上传备份</NeonButton>
      </template>
    </PageHeader>

    <!-- op-cards -->
    <div class="op-cards">
      <div class="op-card op-card--primary">
        <h4>立即备份</h4>
        <p>
          导出当前数据库全量快照，包含用户 / 订单 / 配置。上限
          <b>{{ maxUploadMB }} MB</b>，当前共 {{ total }} 个历史文件。
        </p>
        <NeonButton variant="green" :loading="creating" @click="onCreate">立即备份 →</NeonButton>
      </div>
      <div class="op-card op-card--danger">
        <h4>从备份恢复</h4>
        <p>
          选择下方历史备份文件执行恢复，将覆盖当前数据库。<b>不可回退。</b>
          恢复功能：<b>{{ allowRestore ? '已启用' : '已禁用' }}</b>
        </p>
        <NeonButton variant="outline" @click="uploadDlg = true">上传恢复文件 →</NeonButton>
      </div>
    </div>

    <!-- 历史备份列表 -->
    <el-table v-loading="loading" :data="items" stripe style="width:100%">
      <el-table-column prop="backup_id" label="ID" width="220" show-overflow-tooltip />
      <el-table-column prop="file_name" label="文件" min-width="220" show-overflow-tooltip />

      <el-table-column label="大小" width="100">
        <template #default="{ row }">
          <span class="num-mono">{{ formatBytes(row.size_bytes) }}</span>
        </template>
      </el-table-column>

      <el-table-column label="来源" width="90">
        <template #default="{ row }">
          <StatusTag variant="free">{{ row.trigger }}</StatusTag>
        </template>
      </el-table-column>

      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <StatusTag :variant="backupStatusVariant(row.status)" dot>{{ row.status }}</StatusTag>
        </template>
      </el-table-column>

      <el-table-column label="创建时间" width="165">
        <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
      </el-table-column>

      <el-table-column prop="sha256" label="SHA256" min-width="160" show-overflow-tooltip />

      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <div class="op">
            <span
              class="op-link"
              :class="{ disabled: row.status !== 'ready' }"
              @click="row.status === 'ready' && download(row)"
            >下载</span>
            <span
              class="op-link"
              :class="{ disabled: !allowRestore || row.status !== 'ready' }"
              @click="allowRestore && row.status === 'ready' && onRestore(row)"
            >恢复</span>
            <span class="op-link danger" @click="onDelete(row)">删除</span>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <EmptyState
      v-if="!loading && total === 0"
      title="暂无备份记录"
      desc="点击「立即备份」创建第一个数据库快照"
    />

    <div v-if="total > 0" class="pagination-bar">
      <span>共 <b>{{ total }}</b> 个备份文件</span>
    </div>

    <!-- 上传弹窗 -->
    <el-dialog v-model="uploadDlg" title="上传备份文件" width="460px">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom:12px"
        title="仅接受 .sql.gz 格式；恢复仍需在列表中单独操作。"
      />
      <el-form label-width="110px">
        <el-form-item label="文件">
          <input type="file" accept=".gz,.sql.gz" @change="pickFile" />
          <div
            v-if="uploadFile"
            style="font-size:12px;margin-top:6px;color:var(--el-text-color-secondary)"
          >
            已选择 {{ uploadFile.name }} · {{ formatBytes(uploadFile.size) }}
          </div>
        </el-form-item>
        <el-form-item label="管理员密码">
          <el-input v-model="uploadPwd" type="password" show-password />
        </el-form-item>
        <el-form-item v-if="uploading" label="进度">
          <el-progress :percentage="uploadPct" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="uploadDlg = false">取消</NeonButton>
        <NeonButton variant="green" :loading="uploading" @click="doUpload">上传</NeonButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.op-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 24px;
}

.op-card {
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 24px 28px;

  &--primary { border-left: 5px solid $c-green; }
  &--danger  { border-left: 5px solid $c-orange; }

  h4 {
    font-size: 18px;
    font-weight: 800;
    margin: 0 0 8px;
    letter-spacing: -0.01em;
    color: $n-ink;
  }

  p {
    color: $gray-500;
    margin: 0 0 16px;
    font-size: 14px;
    line-height: 1.6;
    b { color: $c-orange-text; font-weight: 700; }
  }
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
  color: $c-green-text;
  &:hover { background: rgba(0, 230, 118, 0.08); }

  &.danger {
    color: $c-orange-text;
    &:hover { background: rgba(255, 107, 53, 0.08); }
  }

  &.disabled {
    opacity: 0.4;
    cursor: not-allowed;
    pointer-events: none;
  }
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

@media (max-width: 900px) {
  .op-cards { grid-template-columns: 1fr; }
}
</style>
