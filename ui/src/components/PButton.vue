<script setup>
defineProps({
  variant: { type: String, default: 'primary' },
  to: { type: String, default: null },
  href: { type: String, default: null },
  external: { type: Boolean, default: false }
});

const getComponentType = (to, href) => {
  if (href) return 'a'
  if (to) return 'router-link'
  return 'button'
}
</script>

<template>
  <component
    :is="getComponentType(to, href)"
    :to="to"
    :href="href"
    :target="external ? '_blank' : undefined"
    :rel="external ? 'noopener noreferrer' : undefined"
    :class="[
      'px-5 py-2.5 rounded-lg font-medium cursor-pointer transition-all duration-200 active:scale-95 inline-flex items-center justify-center gap-2',
      variant === 'primary' 
        ? 'bg-brand text-white hover:bg-brand-dark shadow-lg shadow-brand/20' 
        : 'bg-surface-card border border-surface-border text-zinc-300 hover:text-white hover:bg-zinc-800'
    ]"
  >
    <slot />
  </component>
</template>
