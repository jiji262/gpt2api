<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import * as adminApi from '@/api/admin'

const loading = ref(false)
const rows = ref<adminApi.Group[]>([])

async function load() {
  loading.value = true
  try {
    const d = await adminApi.listGroups()
    rows.value = d.items
  } finally { loading.value = false }
}

const dlg = ref(false)
const isEdit = ref(false)
const form = reactive<adminApi.Group>({
  id: 0, name: '', ratio: 1, daily_limit_credits: 0,
  rpm_limit: 60, tpm_limit: 60_000, remark: '',
})
function openCreate() {
  isEdit.value = false
  Object.assign(form, { id: 0, name: '', ratio: 1, daily_limit_credits: 0, rpm_limit: 60, tpm_limit: 60_000, remark: '' })
  dlg.value = true
}
function openEdit(row: adminApi.Group) {
  isEdit.value = true
  Object.assign(form, row)
  dlg.value = true
}
async function submit() {
  if (!form.name || form.ratio <= 0) return ElMessage.warning('名称/倍率不合法')
  const payload = {
    name: form.name, ratio: form.ratio,
    daily_limit_credits: form.daily_limit_credits,
    rpm_limit: form.rpm_limit, tpm_limit: form.tpm_limit,
    remark: form.remark,
  }
  if (isEdit.value) await adminApi.updateGroup(form.id, payload)
  else await adminApi.createGroup(payload)
  ElMessage.success('保存成功')
  dlg.value = false
  load()
}
async function onDelete(row: adminApi.Group) {
  await ElMessageBox.confirm(`确认删除分组 "${row.name}"?仅当该分组下无用户时才可删除。`, '删除分组', {
    type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
  })
  await adminApi.deleteGroup(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 分组" title="用户分组" accent-word="分组" accent="cyan">
      <template #extra>
        <NeonButton variant="cyan" size="sm" @click="openCreate">
          <el-icon><Plus /></el-icon> 新增分组
        </NeonButton>
      </template>
    </PageHeader>

    <!-- Table -->
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" width="140" />
      <el-table-column label="倍率" width="110">
        <template #default="{ row }">
          <StatusTag variant="cyan">×{{ row.ratio }}</StatusTag>
        </template>
      </el-table-column>
      <el-table-column label="日限额(厘)" min-width="140">
        <template #default="{ row }">
          {{ row.daily_limit_credits === 0 ? '不限' : row.daily_limit_credits }}
        </template>
      </el-table-column>
      <el-table-column prop="rpm_limit" label="RPM" width="100" />
      <el-table-column prop="tpm_limit" label="TPM" width="130" />
      <el-table-column prop="remark" label="备注" min-width="200" show-overflow-tooltip />
      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <div class="op">
            <a class="op-link" @click="openEdit(row)">编辑</a>
            <a class="op-link danger" :class="{ disabled: row.id === 1 }" @click="row.id !== 1 && onDelete(row)">删除</a>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create / Edit Dialog -->
    <el-dialog v-model="dlg" :title="isEdit ? '编辑分组' : '新建分组'" width="500px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称" required><el-input v-model="form.name" maxlength="64" /></el-form-item>
        <el-form-item label="倍率">
          <el-input-number v-model="form.ratio" :step="0.1" :min="0.1" :precision="2" />
          <span style="margin-left:8px;color:var(--el-text-color-secondary);font-size:12px">
            VIP 通常 0.8,SVIP 0.6
          </span>
        </el-form-item>
        <el-form-item label="日限额">
          <el-input-number v-model="form.daily_limit_credits" :min="0" :step="10000" style="width:100%" />
          <div style="font-size:12px;color:var(--el-text-color-secondary)">0 表示不限,单位厘</div>
        </el-form-item>
        <el-form-item label="RPM">
          <el-input-number v-model="form.rpm_limit" :min="1" :step="10" style="width:100%" />
        </el-form-item>
        <el-form-item label="TPM">
          <el-input-number v-model="form.tpm_limit" :min="100" :step="1000" style="width:100%" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="dlg = false">取消</NeonButton>
        <NeonButton variant="cyan" @click="submit">保存</NeonButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.op { display: flex; gap: 4px; justify-content: flex-end; }
.op-link {
  font-size: 13px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: $r-sm;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
  color: $c-cyan-text;
  &:hover { background: rgba(0, 217, 255, 0.08); }
  &.danger { color: $c-orange-text; &:hover { background: rgba(255, 107, 53, 0.08); } }
  &.disabled { pointer-events: none; opacity: 0.4; }
}
</style>
