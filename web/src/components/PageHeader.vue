<script setup lang="ts">
type Accent = 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'

withDefaults(defineProps<{
  crumb?: string
  title: string
  accentWord?: string
  accent?: Accent
}>(), {
  accent: 'pink',
})
</script>

<template>
  <div class="page-header">
    <div class="page-header__text">
      <div v-if="crumb" class="page-header__crumb">{{ crumb }}</div>
      <h1 class="page-header__title" :class="`accent-${accent}`">
        <template v-if="accentWord && title.endsWith(accentWord)">
          {{ title.slice(0, -accentWord.length) }}<span class="page-header__accent">{{ accentWord }}</span>
        </template>
        <template v-else>{{ title }}</template>
      </h1>
    </div>
    <div class="page-header__extra"><slot name="extra" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 20px;
  margin-bottom: 24px;

  &__crumb {
    font-size: var(--fs-xs);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--text-muted);
    font-weight: 700;
    margin-bottom: 4px;
  }

  &__title {
    font-size: var(--fs-h2);
    line-height: 1.1;
    font-weight: 800;
    letter-spacing: -0.02em;
    margin: 0;
    color: var(--text-primary);
  }

  &__extra {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
  }
}

// 亮底上用压深的 text token 保 WCAG AA
.page-header__title.accent-pink   .page-header__accent { color: $c-pink; }
.page-header__title.accent-cyan   .page-header__accent { color: $c-cyan-text; }
.page-header__title.accent-yellow .page-header__accent { color: $c-yellow-text; }
.page-header__title.accent-purple .page-header__accent { color: $c-purple; }
.page-header__title.accent-orange .page-header__accent { color: $c-orange-text; }
.page-header__title.accent-green  .page-header__accent { color: $c-green-text; }

// 暗底上恢复原色
.dark-area {
  .page-header__title.accent-cyan   .page-header__accent { color: $c-cyan; }
  .page-header__title.accent-yellow .page-header__accent { color: $c-yellow; }
  .page-header__title.accent-green  .page-header__accent { color: $c-green; }
}
</style>
