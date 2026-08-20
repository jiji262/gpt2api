<script setup lang="ts">
type Accent = 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'

withDefaults(defineProps<{
  accent?: Accent
  kicker?: string
  title: string
}>(), {
  accent: 'pink',
})
</script>

<template>
  <div class="feature-card" :class="`feature-card--${accent}`">
    <div v-if="kicker" class="feature-card__kicker">{{ kicker }}</div>
    <h3 class="feature-card__title"><slot name="title">{{ title }}</slot></h3>
    <p class="feature-card__desc"><slot /></p>
    <div class="feature-card__ill"><slot name="illustration" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.feature-card {
  background: $n-ink-p;
  border: $bw solid $dark-border-strong;
  border-radius: $r-xl;
  padding: 28px 26px 30px;
  position: relative;
  overflow: hidden;
  min-height: 320px;
  display: flex;
  flex-direction: column;
  color: $dark-text-1;
  transition: transform 0.2s, border-color 0.2s;

  &:hover { transform: translateY(-4px); }

  &--pink   { border-color: $c-pink;   .feature-card__kicker { color: $c-pink; } }
  &--cyan   { border-color: $c-cyan;   .feature-card__kicker { color: $c-cyan; } }
  &--yellow { border-color: $c-yellow; .feature-card__kicker { color: $c-yellow; } }
  &--purple { border-color: $c-purple; .feature-card__kicker { color: $c-purple; } }
  &--orange { border-color: $c-orange; .feature-card__kicker { color: $c-orange; } }
  &--green  { border-color: $c-green;  .feature-card__kicker { color: $c-green; } }

  &__kicker {
    font-size: var(--fs-xs);
    letter-spacing: 0.18em;
    text-transform: uppercase;
    font-weight: 800;
  }
  &__title {
    font-size: var(--fs-h3);
    font-weight: 800;
    margin: 12px 0 10px;
    letter-spacing: -0.01em;
    line-height: 1.15;
  }
  &__desc {
    font-size: var(--fs-ui);
    color: $dark-text-3;
    line-height: 1.6;
    margin: 0;
  }
  &__ill {
    margin-top: auto;
    padding-top: 18px;
    align-self: flex-end;
    :deep(svg) { width: 130px; height: auto; }
  }
}
</style>
