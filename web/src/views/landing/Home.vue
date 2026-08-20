<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'
import HeroIll from '@/assets/illustrations/hero-palette.svg?component'
import FeatImg2 from '@/assets/illustrations/feature-img2.svg?component'
import FeatBatch from '@/assets/illustrations/feature-batch.svg?component'
import FeatOpenai from '@/assets/illustrations/feature-openai.svg?component'

const router = useRouter()
const user = useUserStore()
const site = useSiteStore()

const siteName = computed(() => site.get('site.name', 'GPT2API'))
const siteLogo = computed(() => site.get('site.logo_url', ''))
const allowRegister = computed(() => site.allowRegister())
const loggedIn = computed(() => user.isLoggedIn)

function goPlay() {
  if (loggedIn.value) router.push('/personal/play')
  else router.push('/login?redirect=/personal/play')
}
function goDashboard() { router.push('/personal/dashboard') }
function goLogin() { router.push('/login') }
function goRegister() { router.push('/register') }
function scrollTop() { window.scrollTo({ top: 0, behavior: 'smooth' }) }

const scrolled = ref(false)
onMounted(() => {
  const onScroll = () => { scrolled.value = window.scrollY > 24 }
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
})
</script>

<template>
  <div class="landing dark-area">
    <header class="nav" :class="{ scrolled }">
      <div class="nav-inner">
        <a class="logo" @click="scrollTop">
          <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
          <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
          <span class="logo-name">{{ siteName }}</span>
        </a>
        <div class="nav-links">
          <a href="#features">产品</a>
          <a @click="router.push('/personal/docs')">文档</a>
        </div>
        <div class="nav-actions">
          <template v-if="!loggedIn">
            <NeonButton variant="ghost" size="sm" @click="goLogin">登录</NeonButton>
            <NeonButton v-if="allowRegister" variant="pink" size="sm" @click="goRegister">
              免费注册 →
            </NeonButton>
          </template>
          <template v-else>
            <NeonButton variant="pink" size="sm" @click="goDashboard">进入控制台 →</NeonButton>
          </template>
        </div>
      </div>
    </header>

    <section class="hero">
      <div class="hero__text">
        <div class="eyebrow"><span class="dot" />IMG2 · 终稿直出 · OpenAI 兼容</div>
        <h1 class="hero__title">
          给你的 <span class="c-pink">AI</span><br>
          一个<span class="c-yellow">调色盘</span>。
        </h1>
        <p class="hero__lead">
          基于 chatgpt.com 的 OpenAI 兼容 SaaS 网关。多账号池、代理池、IMG2 终稿直出、本地 2K / 4K 高清放大 — 一个 <code>base_url</code> 全部接入。
        </p>
        <div class="hero__ctas">
          <NeonButton variant="pink" size="lg" @click="goPlay">开始使用 →</NeonButton>
          <NeonButton variant="outline" size="lg" tag="a" href="/personal/docs">查看文档</NeonButton>
        </div>
        <div class="hero__stats">
          <div class="stat"><div class="num">2,384+</div><div class="desc">开发者</div></div>
          <div class="stat"><div class="num">12M</div><div class="desc">API 调用</div></div>
          <div class="stat"><div class="num">99.9%</div><div class="desc">成功率</div></div>
          <div class="stat"><div class="num">&lt; 30s</div><div class="desc">P95 延迟</div></div>
        </div>
      </div>
      <div class="hero__ill"><HeroIll /></div>
    </section>

    <section class="features" id="features">
      <div class="sec-head">
        <div class="eyebrow cyan"><span class="dot" />CORE FEATURES</div>
        <h2>三件事，<span class="c-pink">做到极致</span>。</h2>
        <p>IMG2 终稿直出 · 批量多比例 · OpenAI 零改造接入。每一项都是我们反复打磨的核心能力。</p>
      </div>
      <div class="feat-grid">
        <FeatureCard accent="pink" kicker="IMG2 PROTOCOL" title="终稿直出，不悄悄重试">
          全面对齐 <code>picture_v2</code> 正式协议，SSE 够数即返回，60s 短轮询补齐。出错第一时间暴露给调用方。
          <template #illustration><FeatImg2 /></template>
        </FeatureCard>
        <FeatureCard accent="cyan" kicker="BATCH & RATIOS" title="10 种比例，N 张同出">
          21:9 / 16:9 / 4:3 / 1:1 / 9:16 一键切换。一次调用批量返回，同 prompt 可出多变体。
          <template #illustration><FeatBatch /></template>
        </FeatureCard>
        <FeatureCard accent="yellow" kicker="OPENAI COMPAT" title="改一行 base_url，即刻接入">
          <code>/v1/images/generations</code> · <code>/v1/images/edits</code> 原样对齐官方 SDK。切网关只需一行代码。
          <template #illustration><FeatOpenai /></template>
        </FeatureCard>
      </div>
    </section>

    <section class="cta">
      <h2>准备好开始了吗？</h2>
      <p>几分钟内注册账号，生成第一个 API Key，开始调用 IMG2。</p>
      <div class="cta-ctas">
        <NeonButton v-if="!loggedIn && allowRegister" variant="pink" size="lg" @click="goRegister">
          免费注册 →
        </NeonButton>
        <NeonButton v-else-if="loggedIn" variant="pink" size="lg" @click="goDashboard">
          进入控制台 →
        </NeonButton>
        <NeonButton variant="outline" size="lg" tag="a" href="/personal/docs">查看文档</NeonButton>
      </div>
    </section>

    <footer class="footer">
      <div class="footer-inner">
        <div class="footer-brand">
          <img v-if="siteLogo" :src="siteLogo" class="logo-img" alt="logo" />
          <span v-else class="logo-mark">{{ (siteName[0] || 'G').toUpperCase() }}</span>
          <span class="logo-name">{{ siteName }}</span>
        </div>
        <div class="footer-copy">© {{ new Date().getFullYear() }} {{ siteName }} · OpenAI-Compatible Gateway</div>
      </div>
    </footer>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.landing {
  min-height: 100vh;
  background: $n-space;
  color: $dark-text-1;
  font-family: $f-sans;
}

.nav {
  position: sticky;
  top: 0;
  z-index: 20;
  backdrop-filter: blur(12px);
  background: transparent;
  transition: background 0.15s, border-color 0.15s;
  border-bottom: 1px solid transparent;

  &.scrolled {
    background: rgba(10, 7, 24, 0.85);
    border-bottom-color: $dark-border;
  }

  .nav-inner {
    max-width: 1200px;
    margin: 0 auto;
    padding: 18px 32px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 32px;
  }

  .logo {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    font-weight: 800;
    font-size: 18px;
    letter-spacing: -0.01em;
    color: $dark-text-1;
    .logo-img { width: 28px; height: 28px; border-radius: $r-md; object-fit: contain; background: white; }
    .logo-mark {
      width: 28px; height: 28px; border-radius: $r-md;
      background: $c-pink; color: white;
      display: inline-flex; align-items: center; justify-content: center;
      font-weight: 900;
    }
  }

  .nav-links {
    display: flex;
    gap: 28px;
    font-size: 14px;
    font-weight: 500;
    color: $dark-text-3;
    a { cursor: pointer; color: inherit; &:hover { color: $dark-text-1; } }
  }

  .nav-actions { display: inline-flex; gap: 10px; align-items: center; }
}

.hero {
  max-width: 1200px;
  margin: 0 auto;
  padding: 80px 32px 80px;
  display: grid;
  grid-template-columns: 1.3fr 1fr;
  gap: 60px;
  align-items: center;

  &__text { max-width: 640px; }
  &__title {
    font-size: var(--fs-hero);
    font-weight: 800;
    letter-spacing: -0.035em;
    line-height: 0.98;
    margin: 20px 0 28px;
    .c-pink   { color: $c-pink; }
    .c-yellow { color: $c-yellow; }
  }
  &__lead {
    font-size: var(--fs-lead);
    line-height: 1.55;
    color: $dark-text-2;
    margin: 0 0 36px;
    code { font-family: $f-mono; background: rgba(255,255,255,0.06); padding: 1px 6px; border-radius: $r-sm; font-size: 0.9em; }
  }
  &__ctas { display: flex; gap: 12px; flex-wrap: wrap; }

  &__stats {
    display: flex;
    gap: 40px;
    margin-top: 60px;
    flex-wrap: wrap;
    .stat {
      border-left: 2px solid rgba(255, 61, 148, 0.5);
      padding-left: 14px;
      .num {
        font-size: 26px; font-weight: 800; letter-spacing: -0.02em; color: $dark-text-1;
      }
      .desc { font-size: var(--fs-sm); color: $dark-text-3; margin-top: 2px; }
    }
  }

  &__ill :deep(svg) { width: 100%; height: auto; }
}

.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: var(--fs-xs);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  font-weight: 800;
  color: $c-pink;
  &.cyan { color: $c-cyan; }
  &.yellow { color: $c-yellow; }
  .dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: currentColor;
  }
}

.features {
  max-width: 1200px;
  margin: 0 auto;
  padding: 80px 32px;
}
.sec-head {
  text-align: center;
  margin-bottom: 48px;
  h2 {
    font-size: var(--fs-h1);
    font-weight: 800;
    letter-spacing: -0.03em;
    line-height: 1.05;
    margin: 10px 0 14px;
    .c-pink { color: $c-pink; }
    .c-cyan { color: $c-cyan; }
  }
  p {
    color: $dark-text-3;
    font-size: 17px;
    max-width: 600px;
    margin: 0 auto;
    line-height: 1.6;
  }
  .eyebrow { justify-content: center; }
}

.feat-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 22px; }

.cta {
  max-width: 900px;
  margin: 40px auto 80px;
  padding: 60px 32px;
  background: $n-ink-p;
  border: $bw solid $dark-border-strong;
  border-radius: $r-xl;
  text-align: center;

  h2 {
    font-size: var(--fs-h1);
    font-weight: 800;
    letter-spacing: -0.03em;
    margin: 0 0 14px;
    color: $dark-text-1;
  }
  p {
    color: $dark-text-3;
    font-size: 17px;
    margin: 0 0 32px;
  }
  .cta-ctas { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
}

.footer {
  border-top: $bw solid $dark-border;
  padding: 32px;
  .footer-inner {
    max-width: 1200px;
    margin: 0 auto;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 20px;
    flex-wrap: wrap;
  }
  .footer-brand {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-weight: 800;
    color: $dark-text-1;
    .logo-mark {
      width: 24px; height: 24px; border-radius: $r-sm;
      background: $c-pink; color: white;
      display: inline-flex; align-items: center; justify-content: center;
      font-size: 12px; font-weight: 900;
    }
  }
  .footer-copy { color: $dark-text-3; font-size: var(--fs-sm); }
}

@media (max-width: 900px) {
  .hero { grid-template-columns: 1fr; gap: 40px; padding: 60px 24px; }
  .feat-grid { grid-template-columns: 1fr; }
  .sec-head h2 { font-size: 36px; }
  .hero__title { font-size: 56px; }
}
</style>
