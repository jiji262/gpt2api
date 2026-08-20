<script setup lang="ts">
type Variant =
  | 'pro' | 'free' | 'admin'
  | 'active' | 'disabled'
  | 'success' | 'warning' | 'danger' | 'info'
  | 'pink' | 'cyan' | 'yellow' | 'purple' | 'green' | 'orange'

withDefaults(defineProps<{
  variant?: Variant
  dot?: boolean
}>(), {
  variant: 'free',
  dot: false,
})
</script>

<template>
  <span class="status-tag" :class="`status-tag--${variant}`">
    <span v-if="dot" class="dot" />
    <slot />
  </span>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.status-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: $r-pill;
  border: $bw solid currentColor;
  background: transparent;
  line-height: 1.4;
  letter-spacing: 0.01em;
  font-family: $f-sans;

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
  }

  &--pro, &--purple    { color: $c-purple-text; background: rgba(168, 85, 247, 0.08); }
  &--admin, &--orange  { color: $c-orange-text; background: rgba(255, 107, 53, 0.08); }
  &--active, &--success, &--green { color: $c-green-text; background: rgba(0, 230, 118, 0.08); }
  &--pink              { color: $c-pink-text;   background: rgba(255, 61, 148, 0.08); }
  &--cyan, &--info     { color: $c-cyan-text;   background: rgba(0, 217, 255, 0.1); }
  &--yellow, &--warning{ color: $c-yellow-text; background: rgba(255, 214, 0, 0.12); }
  &--danger            { color: $c-orange-text; background: rgba(255, 107, 53, 0.08); }
  &--free, &--disabled { color: $gray-500;      border-color: $gray-300; background: $gray-100; }
}

.dark-area .status-tag {
  &--pro, &--purple    { color: #C5A1FF; background: rgba(168, 85, 247, 0.12); }
  &--cyan, &--info     { color: $c-cyan;  background: rgba(0, 217, 255, 0.1); }
  &--yellow, &--warning{ color: $c-yellow; background: rgba(255, 214, 0, 0.1); }
  &--free, &--disabled { color: $dark-text-3; background: rgba(255, 255, 255, 0.04); border-color: $dark-border-strong; }
}
</style>
