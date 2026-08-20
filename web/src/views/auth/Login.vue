<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import type { FormInstance } from 'element-plus'
import { ElMessage } from 'element-plus'
import { useRouter, useRoute } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'

const router = useRouter()
const route = useRoute()
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

const form = reactive({
  email: '',
  password: '',
})

const rules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email' as const, message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '至少 6 位', trigger: 'blur' },
  ],
}

async function onSubmit() {
  if (!formRef.value) return
  const ok = await formRef.value.validate().catch(() => false)
  if (!ok) return
  loading.value = true
  try {
    await store.login(form.email, form.password)
    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || '/personal/dashboard'
    router.replace(redirect)
  } catch {
    // 错误已由 axios 拦截器 toast
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
        欢迎<br><span class="c-pink">回来</span>。
      </h1>
      <p class="brand-sub">{{ siteDesc }}</p>
      <ul class="brand-features">
        <li>多账号池 · 多代理池 · 高并发调度</li>
        <li>RBAC 权限 · 全链路审计</li>
        <li>积分钱包 · 预扣结算 · 用量透明</li>
      </ul>
      <svg class="brand-deco" width="180" height="120" viewBox="0 0 180 140" fill="none" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="55" cy="70" r="22" stroke="#FF3D94" stroke-width="2"/>
        <circle cx="55" cy="70" r="8" stroke="#FF3D94" stroke-width="2"/>
        <path d="M77 70 L160 70 M140 70 L140 85 M120 70 L120 80" stroke="#00D9FF" stroke-width="2"/>
        <path d="M10 35 l4 0 M12 33 l0 4" stroke="#FFD600" stroke-width="1.8"/>
        <path d="M145 30 l3 0 M146.5 28.5 l0 3" stroke="#A855F7" stroke-width="1.8"/>
      </svg>
    </div>

    <div class="auth-form">
      <div class="auth-form__inner">
        <h2 class="auth-form__title">登录</h2>
        <p v-if="allowRegister" class="auth-form__sub">
          还没账号？<a class="link" @click="router.push('/register')">立即注册 →</a>
        </p>
        <p v-else class="auth-form__sub">请使用管理员分配的账号登录</p>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          size="large"
          label-position="top"
          class="auth-form__form"
          @submit.prevent="onSubmit"
        >
          <el-form-item label="邮箱" prop="email">
            <el-input
              v-model="form.email"
              placeholder="you@example.com"
              autocomplete="email"
              @keyup.enter="onSubmit"
            />
          </el-form-item>
          <el-form-item label="密码" prop="password">
            <el-input
              v-model="form.password"
              type="password"
              show-password
              placeholder="至少 6 位"
              autocomplete="current-password"
              @keyup.enter="onSubmit"
            />
          </el-form-item>
          <NeonButton variant="pink" size="lg" :block="true" :disabled="loading" @click="onSubmit">
            {{ loading ? '登录中…' : '登录 →' }}
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
      background: $c-pink; color: white;
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
    .c-pink { color: $c-pink; }
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
        background: $c-pink;
        flex-shrink: 0;
      }
      &:nth-child(2)::before { background: $c-cyan; }
      &:nth-child(3)::before { background: $c-yellow; }
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
