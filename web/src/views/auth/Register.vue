<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import type { FormInstance } from 'element-plus'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'

const router = useRouter()
const store = useUserStore()
const site = useSiteStore()

const siteName = computed(() => site.get('site.name', 'GPT2API'))
const siteDesc = computed(() =>
  site.get('site.description', '基于 chatgpt.com 的 OpenAI 兼容网关 · IMG2 终稿直出 · 批量出图'),
)
const siteLogo = computed(() => site.get('site.logo_url', ''))
const allowRegister = computed(() => site.allowRegister())

const formRef = ref<FormInstance>()
const loading = ref(false)
const form = reactive({ email: '', password: '', confirm: '', nickname: '' })

const rules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 64, message: '6~64 位', trigger: 'blur' },
  ],
  confirm: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_r: unknown, v: string, cb: (e?: Error) => void) => {
        if (v !== form.password) cb(new Error('两次密码不一致'))
        else cb()
      },
      trigger: 'blur',
    },
  ],
}

async function onSubmit() {
  if (!formRef.value) return
  const ok = await formRef.value.validate().catch(() => false)
  if (!ok) return
  loading.value = true
  try {
    await store.register(form.email, form.password, form.nickname)
    ElMessage.success('注册成功,正在登录…')
    await store.login(form.email, form.password)
    router.replace('/personal/dashboard')
  } catch {
    // toast 由拦截器处理
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-brand dark-area">
      <div class="brand-logo">
        <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
        <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
        <span class="logo-name">{{ siteName }}</span>
      </div>
      <h1 class="brand-title">
        加入<br><span class="c-yellow">GPT2API</span>。
      </h1>
      <p class="brand-sub">{{ siteDesc }}</p>
      <ul class="brand-features">
        <li>多账号池 · 多代理池 · 高并发调度</li>
        <li>RBAC 权限 · 全链路审计</li>
        <li>积分钱包 · 预扣结算 · 用量透明</li>
      </ul>
      <svg class="brand-deco" width="180" height="120" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
        <rect x="30" y="40" width="120" height="80" rx="6" stroke="#FFD600" stroke-width="2"/>
        <path d="M30 50 L90 90 L150 50" stroke="#FF3D94" stroke-width="2"/>
        <path d="M155 30 L175 30 M165 20 L165 40" stroke="#00D9FF" stroke-width="2"/>
        <path d="M10 100 l3 0 M11.5 98.5 l0 3" stroke="#A855F7" stroke-width="1.8"/>
      </svg>
    </div>

    <div class="auth-form">
      <div class="auth-form__inner">
        <h2 class="auth-form__title">注册</h2>
        <p class="auth-form__sub">
          已经有账号？<a class="link" @click="router.push('/login')">立即登录 →</a>
        </p>

        <el-alert
          v-if="!allowRegister"
          type="warning"
          :closable="false"
          title="当前站点已关闭自助注册"
          description="请联系管理员开通账号,或改用已有账号登录。"
          style="margin-bottom:16px"
        />

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          size="large"
          label-position="top"
          class="auth-form__form"
          :disabled="!allowRegister"
          @submit.prevent="onSubmit"
        >
          <el-form-item label="邮箱" prop="email">
            <el-input v-model="form.email" placeholder="you@example.com" autocomplete="username" />
          </el-form-item>
          <el-form-item label="昵称" prop="nickname">
            <el-input v-model="form.nickname" placeholder="选填" />
          </el-form-item>
          <el-form-item label="密码" prop="password">
            <el-input v-model="form.password" type="password" show-password autocomplete="new-password" />
          </el-form-item>
          <el-form-item label="确认密码" prop="confirm">
            <el-input v-model="form.confirm" type="password" show-password autocomplete="new-password"
                      @keyup.enter="onSubmit" />
          </el-form-item>
          <NeonButton variant="pink" size="lg" :block="true" :disabled="loading || !allowRegister" @click="onSubmit">
            {{ loading ? '提交中…' : '注册 →' }}
          </NeonButton>
        </el-form>

        <p class="auth-form__foot">受 Cloudflare Turnstile 保护</p>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.auth-page {
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 100vh;
}

.auth-brand {
  background: $n-ink-p;
  color: $dark-text-1;
  padding: 72px 60px;
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;

  .brand-logo {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-weight: 800;
    font-size: 18px;
    margin-bottom: 40px;
    .logo-img { width: 32px; height: 32px; border-radius: $r-md; object-fit: contain; background: white; }
    .logo-mark {
      width: 32px; height: 32px; border-radius: $r-md;
      background: $c-yellow; color: $n-ink;
      display: inline-flex; align-items: center; justify-content: center;
      font-size: 16px; font-weight: 900;
    }
  }

  .brand-title {
    font-size: var(--fs-h1);
    font-weight: 800;
    letter-spacing: -0.03em;
    line-height: 1.02;
    margin: 0 0 18px;
    .c-yellow { color: $c-yellow; }
  }
  .brand-sub {
    color: $dark-text-2;
    line-height: 1.6;
    font-size: 15px;
    margin: 0 0 32px;
    max-width: 400px;
  }
  .brand-features {
    list-style: none;
    padding: 0;
    margin: 0;
    li {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 0;
      font-size: 14px;
      color: $dark-text-3;
      &::before {
        content: '';
        width: 6px; height: 6px; border-radius: 50%;
        background: $c-yellow;
        flex-shrink: 0;
      }
      &:nth-child(2)::before { background: $c-cyan; }
      &:nth-child(3)::before { background: $c-pink; }
    }
  }
  .brand-deco {
    position: absolute;
    right: 32px;
    bottom: 32px;
    opacity: 0.85;
  }
}

.auth-form {
  background: $n-paper;
  padding: 72px 60px;
  display: flex;
  align-items: center;
  justify-content: center;

  &__inner { width: 100%; max-width: 400px; }
  &__title {
    font-size: var(--fs-h2);
    font-weight: 800;
    margin: 0 0 4px;
    letter-spacing: -0.02em;
    color: $n-ink;
  }
  &__sub {
    color: $gray-500;
    margin: 0 0 32px;
    font-size: var(--fs-sm);
    .link { color: $c-pink; font-weight: 700; cursor: pointer; &:hover { color: $c-pink-hover; } }
  }
  &__form { margin-bottom: 20px; }
  &__foot {
    font-size: var(--fs-xs);
    color: $gray-400;
    margin: 28px 0 0;
    text-align: center;
    letter-spacing: 0.04em;
  }

  :deep(.el-form-item__label) {
    font-size: var(--fs-xs);
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: $gray-600;
    padding-bottom: 8px;
  }
}

@media (max-width: 900px) {
  .auth-page { grid-template-columns: 1fr; }
  .auth-brand { padding: 48px 32px; min-height: 360px; }
  .auth-form { padding: 48px 32px; }
}
</style>
