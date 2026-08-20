<script setup lang="ts">
</script>

<template>
  <div class="blank-layout dark-area">
    <router-view />
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.blank-layout {
  min-height: 100vh;
  background: $n-space;
  color: $dark-text-1;
}
</style>
