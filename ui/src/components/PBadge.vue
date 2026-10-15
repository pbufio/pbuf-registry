<script setup>
defineProps({
  variant: { type: String, default: 'default' }
});
</script>

<template>
  <span :class="[
    'inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium',
    variant === 'brand' 
      ? 'bg-brand/10 text-brand border border-brand/20'
      : variant === 'info'
      ? 'bg-sky-500/10 text-sky-300 border border-sky-500/20'
      : variant === 'warning'
      ? 'bg-amber-500/10 text-amber-300 border border-amber-500/20'
      : variant === 'critical'
      ? 'bg-red-500/10 text-red-300 border border-red-500/20'
      : variant === 'draft'
      ? 'bg-zinc-700/50 text-zinc-400 border border-zinc-700'
      : 'bg-surface-card text-zinc-300 border border-surface-border'
  ]">
    <slot />
  </span>
</template>
