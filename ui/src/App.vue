<script setup>
import { RouterView } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import AppFooter from '@/components/AppFooter.vue'
</script>

<template>
  <div class="min-h-screen bg-surface-base text-zinc-50 font-sans flex flex-col">
    <AppHeader />
    <main class="flex-1">
      <RouterView />
    </main>
    <AppFooter />
  </div>
</template>
