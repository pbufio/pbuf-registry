<script setup>
defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  type: { type: String, default: 'text' }
});

defineEmits(['update:modelValue']);
</script>

<template>
  <input
    :type="type"
    :value="modelValue"
    @input="$emit('update:modelValue', $event.target.value)"
    :placeholder="placeholder"
    class="w-full px-4 py-2.5 bg-surface-card border border-surface-border rounded-lg text-zinc-50 placeholder-zinc-500 focus:outline-none focus:ring-2 focus:ring-brand focus:border-transparent transition-all"
  />
</template>
