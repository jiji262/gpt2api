<script setup lang="ts">
import { computed, type Component } from 'vue'

type Variant = 'pink' | 'cyan' | 'yellow' | 'purple' | 'green' | 'orange' | 'ink' | 'outline' | 'ghost'
type Size = 'sm' | 'md' | 'lg'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: Size
  block?: boolean
  disabled?: boolean
  loading?: boolean
  icon?: Component | string
  tag?: 'button' | 'a'
  type?: 'button' | 'submit' | 'reset'
  href?: string
}>(), {
  variant: 'pink',
  size: 'md',
  block: false,
  disabled: false,
  loading: false,
  tag: 'button',
  type: 'button',
})

const emit = defineEmits<{ (e: 'click', ev: MouseEvent): void }>()

const classes = computed(() => [
  'neon-btn',
  `neon-btn--${props.variant}`,
  `neon-btn--${props.size}`,
  {
    'is-block': props.block,
    'is-disabled': props.disabled || props.loading,
    'is-loading': props.loading,
  },
])

function onClick(ev: MouseEvent) {
  if (props.disabled || props.loading) {
    ev.preventDefault()
    ev.stopPropagation()
    return
  }
  emit('click', ev)
}
</script>

<template>
  <component
    :is="tag"
    :class="classes"
    :href="tag === 'a' ? href : undefined"
    :type="tag === 'button' ? type : undefined"
    :disabled="tag === 'button' ? (disabled || loading) : undefined"
    :aria-busy="loading ? 'true' : undefined"
    @click="onClick"
  >
    <span v-if="loading" class="neon-btn__spinner" aria-hidden="true" />
    <el-icon v-else-if="icon" class="neon-btn__icon"><component :is="icon" /></el-icon>
    <slot />
  </component>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.neon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 0;
  border-radius: $r-md;
  font-family: $f-sans;
  font-weight: 700;
  letter-spacing: 0.01em;
  cursor: pointer;
  text-decoration: none;
  transition: transform 0.15s, background 0.15s, color 0.15s, border-color 0.15s;
  user-select: none;

  &:hover:not(.is-disabled) { transform: translateY(-1px); }
  &:active:not(.is-disabled) { transform: translateY(0); }
  &.is-block { display: flex; width: 100%; }
  &.is-disabled { opacity: 0.5; cursor: not-allowed; }
  &.is-loading { cursor: wait; }

  // WCAG 2.1 SC 2.4.7 — 键盘聚焦环
  &:focus-visible {
    outline: 3px solid $c-purple;
    outline-offset: 2px;
  }

  // --- sizes ---
  &--sm { height: 32px; padding: 0 14px; font-size: 13px; }
  &--md { height: 40px; padding: 0 20px; font-size: 14px; }
  &--lg { height: 48px; padding: 0 28px; font-size: 16px; }

  // --- variants ---
  &--pink   { background: $c-pink;   color: white;      &:hover:not(.is-disabled) { background: $c-pink-hover; } }
  &--cyan   { background: $c-cyan;   color: $n-space;   &:hover:not(.is-disabled) { background: $c-cyan-hover; } }
  &--yellow { background: $c-yellow; color: $n-ink;     &:hover:not(.is-disabled) { background: $c-yellow-hover; } }
  &--purple { background: $c-purple; color: white;      &:hover:not(.is-disabled) { background: $c-purple-hover; } }
  &--green  { background: $c-green;  color: $n-space;   &:hover:not(.is-disabled) { background: $c-green-hover; } }
  &--orange { background: $c-orange; color: white;      &:hover:not(.is-disabled) { background: $c-orange-hover; } }
  &--ink    { background: $n-ink;    color: white;      &:hover:not(.is-disabled) { background: $n-ink-p; } }

  &--outline {
    background: transparent;
    color: currentColor;
    border: $bw solid currentColor;
    &:hover:not(.is-disabled) { background: currentColor; color: var(--panel-bg); }
  }

  &--ghost {
    background: transparent;
    color: currentColor;
    &:hover:not(.is-disabled) { background: rgba(0, 0, 0, 0.04); }
  }

  // Loading spinner（14px，描边跟当前文字色）
  &__spinner {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    border: 2px solid currentColor;
    border-right-color: transparent;
    animation: neon-spin 0.7s linear infinite;
    flex-shrink: 0;
  }

  // 前置图标
  &__icon {
    font-size: 1.1em;
    line-height: 1;
    flex-shrink: 0;
  }
}

@keyframes neon-spin {
  from { transform: rotate(0); }
  to { transform: rotate(360deg); }
}

.dark-area .neon-btn--ghost:hover { background: rgba(255, 255, 255, 0.06); }
</style>
