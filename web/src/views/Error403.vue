<script setup lang="ts">
import { useRouter } from 'vue-router'
import Err403Ill from '@/assets/illustrations/error-403.svg?component'
const router = useRouter()
</script>

<template>
  <div class="err-page dark-area">
    <div class="err-ill"><Err403Ill /></div>
    <div class="err-code">403</div>
    <h1 class="err-title">没有权限</h1>
    <p class="err-desc">你暂时无法访问此页面。如有必要请联系管理员分配权限。</p>
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
