<script setup>
import { ref } from 'vue';

const props = defineProps({
  code: { type: String, required: true },
  language: { type: String, default: 'bash' }
});

const copied = ref(false);

const copyToClipboard = () => {
  navigator.clipboard.writeText(props.code);
  copied.value = true;
  setTimeout(() => copied.value = false, 2000);
};
</script>

<template>
  <div class="bg-surface-card border border-surface-border rounded-xl overflow-hidden font-mono text-sm">
    <div class="flex items-center justify-between px-4 py-2 bg-zinc-900/50 border-b border-surface-border">
      <div class="flex gap-1.5">
        <div class="w-3 h-3 rounded-full bg-zinc-700"></div>
        <div class="w-3 h-3 rounded-full bg-zinc-700"></div>
        <div class="w-3 h-3 rounded-full bg-zinc-700"></div>
      </div>
      <button 
        @click="copyToClipboard" 
        class="text-xs text-zinc-500 hover:text-zinc-300 cursor-pointer transition-colors px-2 py-1 rounded hover:bg-zinc-800"
      >
        {{ copied ? 'Copied!' : 'Copy' }}
      </button>
    </div>
    <div class="p-4 text-zinc-300 overflow-x-auto">
      <div class="whitespace-pre">
        <span class="text-brand mr-2 select-none">$</span><span>{{ code }}</span>
      </div>
    </div>
  </div>
</template>
