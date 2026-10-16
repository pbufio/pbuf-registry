<script setup>
defineProps({
  hoverable: { type: Boolean, default: false }
});
</script>

<template>
  <div :class="[
    'p-6 bg-surface-card border border-surface-border rounded-xl',
    hoverable && 'hover:border-brand/50 transition-colors cursor-pointer'
  ]">
    <slot />
  </div>
</template>
