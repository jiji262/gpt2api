<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as proxyApi from '@/api/proxies'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const rows = ref<proxyApi.Proxy[]>([])
const total = ref(0)
const pager = reactive({ page: 1, page_size: 20 })

async function fetchList() {
  loading.value = true
  try {
    const data = await proxyApi.listProxies(pager)
    rows.value = data.list
    total.value = data.total
  } finally { loading.value = false }
}

const dlg = ref(false)
const isEdit = ref(false)
const form = reactive<proxyApi.ProxyCreate & { id?: number }>({
  id: 0, scheme: 'http', host: '', port: 0, username: '', password: '',
  country: '', isp: '', enabled: true, remark: '',
})

function openCreate() {
  isEdit.value = false
  Object.assign(form, {
    id: 0, scheme: 'http', host: '', port: 0, username: '', password: '',
    country: '', isp: '', enabled: true, remark: '',
  })
  dlg.value = true
}
function openEdit(row: proxyApi.Proxy) {
  isEdit.value = true
  Object.assign(form, {
    id: row.id, scheme: row.scheme, host: row.host, port: row.port,
    username: row.username, password: '',
    country: row.country, isp: row.isp, enabled: row.enabled, remark: row.remark,
  })
  dlg.value = true
}

async function submit() {
  if (!form.host) return ElMessage.warning('host 不能为空')
  if (!form.port) return ElMessage.warning('port 不能为空')
  const payload: proxyApi.ProxyUpdate = {
    scheme: form.scheme!,
    host: form.host,
    port: Number(form.port),
    username: form.username || '',
    password: form.password || '',
    country: form.country || '',
    isp: form.isp || '',
    enabled: !!form.enabled,
    remark: form.remark || '',
  }
  if (isEdit.value && form.id) await proxyApi.updateProxy(form.id, payload)
  else await proxyApi.createProxy(payload)
  ElMessage.success('保存成功')
  dlg.value = false
  fetchList()
}

async function toggleEnabled(row: proxyApi.Proxy) {
  await proxyApi.updateProxy(row.id, {
    scheme: row.scheme, host: row.host, port: row.port,
    username: row.username, password: '',
    country: row.country, isp: row.isp, remark: row.remark,
    enabled: !row.enabled,
  })
  ElMessage.success(row.enabled ? '已禁用' : '已启用')
  fetchList()
}

async function onDelete(row: proxyApi.Proxy) {
  await ElMessageBox.confirm(`确认删除代理 ${row.host}:${row.port}?`, '删除代理', {
    type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
  })
  await proxyApi.deleteProxy(row.id)
  ElMessage.success('已删除')
  fetchList()
}

// ---------- 健康探测 ----------
const probingIds = ref<Set<number>>(new Set())
const probeAllLoading = ref(false)

async function onProbe(row: proxyApi.Proxy) {
  if (probingIds.value.has(row.id)) return
  probingIds.value.add(row.id)
  try {
    const res = await proxyApi.probeProxy(row.id)
    row.health_score = res.health_score
    row.last_probe_at = res.tried_at
    row.last_error = res.ok ? '' : (res.error || 'failed')
    if (res.ok) {
      ElMessage.success(`连通正常 · ${res.latency_ms} ms`)
    } else {
      ElMessage.error(`探测失败:${res.error || 'unknown'}`)
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '探测失败')
  } finally {
    probingIds.value.delete(row.id)
    probingIds.value = new Set(probingIds.value)
  }
}

async function onProbeAll() {
  await ElMessageBox.confirm(
    '将对所有启用的代理发起连通性探测,耗时取决于代理数量。是否继续?',
    '全部探测',
    { type: 'info', confirmButtonText: '开始', cancelButtonText: '取消' },
  )
  probeAllLoading.value = true
  try {
    const res = await proxyApi.probeAllProxies()
    ElMessage.success(`探测完成 · 共 ${res.total} · 通 ${res.ok} · 断 ${res.bad}`)
    fetchList()
  } catch (e: any) {
    ElMessage.error(e?.message || '探测失败')
  } finally {
    probeAllLoading.value = false
  }
}

// ---------- 批量导入 ----------
const importDlg = ref(false)
const importLoading = ref(false)
const importForm = reactive({
  text: '',
  enabled: true,
  country: '',
  isp: '',
  remark: '',
  overwrite: false,
})
const importResult = ref<proxyApi.ProxyImportResp | null>(null)

function openImport() {
  Object.assign(importForm, {
    text: '', enabled: true, country: '', isp: '', remark: '', overwrite: false,
  })
  importResult.value = null
  importDlg.value = true
}

async function doImport() {
  if (!importForm.text.trim()) return ElMessage.warning('请粘贴至少一行代理 URL')
  importLoading.value = true
  try {
    importResult.value = await proxyApi.importProxies({
      text: importForm.text,
      enabled: importForm.enabled,
      country: importForm.country,
      isp: importForm.isp,
      remark: importForm.remark,
      overwrite: importForm.overwrite,
    })
    const r = importResult.value
    ElMessage.success(
      `完成 · 新增 ${r.created} · 更新 ${r.updated} · 跳过 ${r.skipped} · 无效 ${r.invalid}`,
    )
    fetchList()
  } finally { importLoading.value = false }
}

function importStatusVariant(s: string): 'success' | 'info' | 'warning' | 'danger' | 'active' {
  switch (s) {
    case 'created': return 'success'
    case 'updated': return 'info'
    case 'skipped': return 'info'
    default:        return 'danger'
  }
}
function importStatusText(s: string) {
  return { created: '新增', updated: '更新', skipped: '跳过', invalid: '无效' }[s] || s
}

function healthVariant(score: number): 'active' | 'warning' | 'danger' {
  if (score >= 80) return 'active'
  if (score >= 50) return 'warning'
  return 'danger'
}

onMounted(fetchList)
</script>

<template>
  <div class="page-container">
    <PageHeader crumb="管理员 / 代理" title="代理管理" accent-word="管理" accent="purple">
      <template #extra>
        <NeonButton variant="outline" :loading="probeAllLoading" @click="onProbeAll">全部探测</NeonButton>
        <NeonButton variant="outline" @click="openImport">批量导入</NeonButton>
        <NeonButton variant="purple" @click="openCreate">+ 新建代理</NeonButton>
      </template>
    </PageHeader>

    <!-- 描述 -->
    <div class="page-desc">
      维护 HTTP / SOCKS5 代理池,所有 GPT 账号都应绑定独立代理以分散风控指纹;健康分由定时探测自动维护,探测参数可在「系统设置 → 网关与调度」调整。
    </div>

    <!-- 表格 -->
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="地址" min-width="220">
        <template #default="{ row }">
          <code class="proxy-addr">{{ row.scheme }}://{{ row.host }}:{{ row.port }}</code>
          <div v-if="row.username" class="proxy-auth">auth: {{ row.username }} / ******</div>
        </template>
      </el-table-column>
      <el-table-column label="区域" width="130">
        <template #default="{ row }">
          <div>{{ row.country || '-' }}</div>
          <div class="sub-text">{{ row.isp || '' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="健康" width="160">
        <template #default="{ row }">
          <el-progress :percentage="Math.max(0, Math.min(100, row.health_score))"
                       :status="row.health_score >= 80 ? 'success' : row.health_score >= 50 ? 'warning' : 'exception'" />
          <div v-if="row.last_error" style="margin-top:4px">
            <StatusTag :variant="healthVariant(row.health_score)">
              {{ row.last_error.slice(0, 30) }}
            </StatusTag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="最近探测" width="170">
        <template #default="{ row }">{{ formatDateTime(row.last_probe_at) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" @change="() => toggleEnabled(row)" />
        </template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="180" show-overflow-tooltip />
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <div class="op">
            <a class="op-link" :class="{ loading: probingIds.has(row.id) }"
               @click="!probingIds.has(row.id) && onProbe(row)">探测</a>
            <a class="op-link" @click="openEdit(row)">编辑</a>
            <a class="op-link danger" @click="onDelete(row)">删除</a>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- Empty state -->
    <EmptyState
      v-if="!loading && total === 0"
      title="暂无代理"
      desc="点击「新建代理」或「批量导入」添加代理。"
    />

    <!-- Pagination bar -->
    <div class="pagination-bar">
      <span>共 <b>{{ total }}</b> 个代理</span>
      <el-pagination
        v-model:current-page="pager.page"
        v-model:page-size="pager.page_size"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="sizes, prev, pager, next"
        @current-change="fetchList"
        @size-change="fetchList"
      />
    </div>

    <!-- 新建/编辑弹窗 -->
    <el-dialog v-model="dlg" :title="isEdit ? '编辑代理' : '新建代理'" width="520px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="协议">
          <el-select v-model="form.scheme" style="width:100%">
            <el-option label="http" value="http" />
            <el-option label="https" value="https" />
            <el-option label="socks5" value="socks5" />
          </el-select>
        </el-form-item>
        <el-form-item label="Host" required><el-input v-model="form.host" placeholder="192.0.2.1" /></el-form-item>
        <el-form-item label="Port" required>
          <el-input-number v-model="form.port" :min="1" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item label="用户名"><el-input v-model="form.username" autocomplete="off" /></el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="new-password"
                    :placeholder="isEdit ? '留空表示不改' : ''" />
        </el-form-item>
        <el-form-item label="国家/地区"><el-input v-model="form.country" placeholder="US / JP / HK …" /></el-form-item>
        <el-form-item label="ISP"><el-input v-model="form.isp" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <NeonButton variant="ghost" @click="dlg = false">取消</NeonButton>
        <NeonButton variant="purple" @click="submit">保存</NeonButton>
      </template>
    </el-dialog>

    <!-- 批量导入 -->
    <el-dialog v-model="importDlg" title="批量导入代理" width="720px">
      <el-form label-width="88px" @submit.prevent>
        <el-form-item label="代理列表">
          <el-input
            v-model="importForm.text"
            type="textarea"
            :rows="10"
            resize="vertical"
            placeholder="每行一个,支持以下格式:
http://user:pass@host:port
https://host:port
socks5://user:pass@host:port
user:pass@host:port    (省略 scheme 默认 http)
# 以 # 或 // 开头的行会被跳过"
          />
          <div class="import-hint">
            支持 http / https / socks5。同一 scheme + host + port + username 视为已存在。
          </div>
        </el-form-item>
        <el-form-item label="默认启用">
          <el-switch v-model="importForm.enabled" />
        </el-form-item>
        <el-form-item label="国家/地区">
          <el-input v-model="importForm.country" placeholder="如 US / HK,空则每条自行为空" style="max-width:240px" />
        </el-form-item>
        <el-form-item label="ISP">
          <el-input v-model="importForm.isp" placeholder="如 Arxlabs" style="max-width:240px" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="importForm.remark" placeholder="将填到所有新增行的 remark" />
        </el-form-item>
        <el-form-item label="覆盖已有">
          <el-switch v-model="importForm.overwrite" />
          <span class="import-hint" style="margin-left:8px">
            开启后:同 endpoint 已存在时更新密码/国家/ISP/备注;关闭则跳过。
          </span>
        </el-form-item>
      </el-form>

      <div v-if="importResult" class="import-result">
        <div class="import-summary">
          共 {{ importResult.total }} 行 ·
          <StatusTag variant="success">新增 {{ importResult.created }}</StatusTag>
          <StatusTag variant="info">更新 {{ importResult.updated }}</StatusTag>
          <StatusTag variant="disabled">跳过 {{ importResult.skipped }}</StatusTag>
          <StatusTag variant="danger">无效 {{ importResult.invalid }}</StatusTag>
        </div>
        <el-table :data="importResult.items" size="small" max-height="260" border>
          <el-table-column prop="line" label="行" width="60" />
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <StatusTag :variant="importStatusVariant(row.status)">{{ importStatusText(row.status) }}</StatusTag>
            </template>
          </el-table-column>
          <el-table-column prop="raw" label="内容" show-overflow-tooltip />
          <el-table-column prop="error" label="说明" show-overflow-tooltip />
        </el-table>
      </div>

      <template #footer>
        <NeonButton variant="ghost" @click="importDlg = false">关闭</NeonButton>
        <NeonButton variant="purple" :loading="importLoading" @click="doImport">开始导入</NeonButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.page-desc {
  color: $gray-500;
  font-size: 13px;
  margin-bottom: 16px;
  line-height: 1.6;
}

.proxy-addr {
  background: $gray-200;
  padding: 1px 6px;
  border-radius: $r-sm;
  font-family: $f-mono;
  font-size: 12px;
  color: $n-ink;
}
.proxy-auth { font-size: 12px; color: $gray-500; margin-top: 2px; }
.sub-text   { font-size: 12px; color: $gray-500; }

.op { display: flex; gap: 4px; justify-content: flex-end; }
.op-link {
  color: $c-purple;
  font-size: 13px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: $r-sm;
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
  &:hover { background: rgba(168, 85, 247, 0.08); }
  &.danger { color: $c-orange-text; &:hover { background: rgba(255, 107, 53, 0.08); } }
  &.loading { opacity: 0.5; cursor: wait; }
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

.import-hint {
  font-size: 12px;
  color: $gray-500;
  line-height: 1.5;
  margin-top: 4px;
}
.import-result { margin-top: 8px; }
.import-summary {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  margin-bottom: 8px;
  color: $gray-500;
  font-size: 13px;
}
</style>
