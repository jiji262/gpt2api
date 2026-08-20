<script setup lang="ts">
type Accent = 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'

withDefaults(defineProps<{
  label: string
  value: string | number
  change?: string
  changeDir?: 'up' | 'down' | 'flat'
  accent?: Accent
}>(), {
  accent: 'pink',
  changeDir: 'flat',
})
</script>

<template>
  <div class="kpi-card" :class="`kpi-card--${accent}`">
    <div class="kpi-card__label">{{ label }}</div>
    <div class="kpi-card__value">{{ value }}</div>
    <div v-if="change" class="kpi-card__change" :class="`is-${changeDir}`">{{ change }}</div>
    <div class="kpi-card__ill"><slot name="illustration" /></div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.kpi-card {
  position: relative;
  background: $n-paper;
  border: $bw solid $gray-200;
  border-radius: $r-lg;
  padding: 18px 22px 20px;
  overflow: hidden;
  transition: transform 0.2s, box-shadow 0.2s;

  &:hover { transform: translateY(-2px); box-shadow: $sh-2; }

  &::before {
    content: '';
    position: absolute;
    left: 0; top: 0; bottom: 0;
    width: 5px;
  }
  &--pink::before   { background: $c-pink; }
  &--cyan::before   { background: $c-cyan; }
  &--yellow::before { background: $c-yellow; }
  &--purple::before { background: $c-purple; }
  &--orange::before { background: $c-orange; }
  &--green::before  { background: $c-green; }

  &__label {
    font-size: var(--fs-xs);
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: $gray-500;
    font-weight: 700;
  }

  &__value {
    font-size: 34px;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: $n-ink;
    margin: 6px 0 2px;
    line-height: 1.1;
    font-family: $f-sans;
  }

  &__change {
    font-size: var(--fs-sm);
    font-weight: 700;
    &.is-up   { color: $c-green-text; &::before { content: '↗ '; } }
    &.is-down { color: $c-orange-text; &::before { content: '↘ '; } }
    &.is-flat { color: $gray-500; }
  }

  &__ill {
    position: absolute;
    right: 14px;
    bottom: 10px;
    opacity: 0.9;
    pointer-events: none;
  }
}
</style>
