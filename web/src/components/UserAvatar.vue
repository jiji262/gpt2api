<script setup lang="ts">
import { computed } from 'vue'

type Size = 'sm' | 'md' | 'lg' | 'xl'

const props = withDefaults(defineProps<{
  name?: string
  size?: Size
  color?: 'pink' | 'cyan' | 'yellow' | 'purple' | 'orange' | 'green'
}>(), {
  name: '?',
  size: 'md',
})

function hashColor(name: string): string {
  const palette = ['pink', 'cyan', 'yellow', 'purple', 'orange', 'green']
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h + name.charCodeAt(i)) >>> 0
  return palette[h % palette.length]!
}

const initial = computed(() => (props.name || '?').trim().charAt(0).toUpperCase() || '?')
const flavor = computed(() => props.color || hashColor(props.name || 'anon'))
</script>

<template>
  <span class="avatar" :class="[`avatar--${size}`, `avatar--${flavor}`]">
    {{ initial }}
  </span>
</template>

<style scoped lang="scss">
@use '@/styles/tokens' as *;

.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: white;
  font-family: $f-sans;
  font-weight: 800;
  flex-shrink: 0;

  &--sm { width: 28px; height: 28px; font-size: 12px; }
  &--md { width: 38px; height: 38px; font-size: 14px; }
  &--lg { width: 52px; height: 52px; font-size: 18px; }
  &--xl { width: 72px; height: 72px; font-size: 26px; }

  &--pink   { background: $c-pink; }
  &--purple { background: $c-purple; }
  &--orange { background: $c-orange; }
  &--cyan   { background: $c-cyan;   color: $n-space; }
  &--yellow { background: $c-yellow; color: $n-ink; }
  &--green  { background: $c-green;  color: $n-space; }
}
</style>
