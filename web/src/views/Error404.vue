<script setup lang="ts">
import { useRouter } from 'vue-router'
import Err404Ill from '@/assets/illustrations/error-404.svg?component'
const router = useRouter()
</script>

<template>
  <div class="err-page dark-area">
    <div class="err-ill"><Err404Ill /></div>
    <div class="err-code">404</div>
    <h1 class="err-title">找不到页面</h1>
    <p class="err-desc">你要去的地方可能被搬家了，或者链接已失效。</p>
    <NeonButton variant="pink" size="lg" @click="router.replace('/')">回到首页 →</NeonButton>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.err-page {
  min-height: 100vh;
  background: $n-space;
  color: $dark-text-1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  text-align: center;
}
.err-ill { margin-bottom: 28px; :deep(svg) { width: 180px; height: 180px; } }
.err-code {
  font-size: 120px;
  font-weight: 900;
  color: $c-pink;
  letter-spacing: -0.05em;
  line-height: 1;
  margin-bottom: 14px;
  font-family: $f-sans;
}
.err-title {
  font-size: var(--fs-h1);
  font-weight: 800;
  letter-spacing: -0.03em;
  margin: 0 0 12px;
}
.err-desc {
  color: $dark-text-3;
  font-size: 17px;
  margin: 0 0 32px;
  max-width: 400px;
  line-height: 1.6;
}
</style>
