<script setup>
</script>

<template>
  <div class="p-6 bg-surface-card border border-surface-border rounded-2xl hover:border-brand/50 transition-colors group">
    <div class="w-12 h-12 rounded-lg bg-brand/10 flex items-center justify-center mb-4 text-brand group-hover:bg-brand group-hover:text-white transition-all">
      <slot name="icon" />
    </div>
    <h3 class="text-lg font-semibold text-zinc-50 mb-2">
      <slot name="title" />
    </h3>
    <p class="text-zinc-400 leading-relaxed text-sm">
      <slot />
    </p>
  </div>
</template>
