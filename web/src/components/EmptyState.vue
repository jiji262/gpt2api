<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  desc?: string
}>(), {})
</script>

<template>
  <div class="empty-state">
    <div class="empty-state__ill"><slot name="illustration" /></div>
    <h3 class="empty-state__title">{{ title }}</h3>
    <p v-if="desc" class="empty-state__desc">{{ desc }}</p>
    <div class="empty-state__action"><slot name="action" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 48px 24px;
  min-height: 280px;

  &__ill {
    margin-bottom: 20px;
    max-width: 180px;
    :deep(svg) { width: 100%; height: auto; }
  }
  &__title {
    font-size: var(--fs-h4);
    font-weight: 800;
    color: var(--text-primary);
    margin: 0 0 8px;
    letter-spacing: -0.01em;
  }
  &__desc {
    font-size: var(--fs-sm);
    color: var(--text-muted);
    line-height: 1.6;
    margin: 0 0 20px;
    max-width: 380px;
  }
  &__action {
    display: flex;
    gap: 10px;
  }
}
</style>
