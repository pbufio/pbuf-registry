<script setup>
import { ref, onBeforeUnmount } from 'vue';
import { copyTextToClipboard } from '@/utils/protoDiff';

const props = defineProps({
  code: { type: String, required: true },
  language: { type: String, default: 'bash' }
});

const copied = ref(false);
let copyTimer = null;

const copyToClipboard = async () => {
  if (!(await copyTextToClipboard(props.code))) return;
  if (copyTimer) clearTimeout(copyTimer);
  copied.value = true;
  copyTimer = setTimeout(() => {
    copied.value = false;
    copyTimer = null;
  }, 2000);
};

onBeforeUnmount(() => {
  if (copyTimer) clearTimeout(copyTimer);
});
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
