<script setup>
defineProps({
  columns: { type: Array, required: true },
  data: { type: Array, required: true }
});
</script>

<template>
  <div class="overflow-x-auto bg-surface-card border border-surface-border rounded-xl">
    <table class="w-full">
      <thead>
        <tr class="border-b border-surface-border">
          <th 
            v-for="column in columns" 
            :key="column.key"
            class="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase tracking-wider"
          >
            {{ column.label }}
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-surface-border">
        <tr 
          v-for="(row, index) in data" 
          :key="index"
          class="hover:bg-zinc-900/50 transition-colors"
        >
          <td 
            v-for="column in columns" 
            :key="column.key"
            class="px-6 py-4 text-sm text-zinc-300"
          >
            <slot :name="`cell-${column.key}`" :row="row">
              {{ row[column.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
