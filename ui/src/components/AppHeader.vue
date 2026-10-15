<script setup>
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import PButton from './PButton.vue'
import { authToken } from '@/auth/authToken'
import { appConfig } from '@/config/appConfig'

const isLoggedIn = authToken.isLoggedIn

const mobileMenuOpen = ref(false)

const router = useRouter()
const route = useRoute()

const toggleMobileMenu = () => {
  mobileMenuOpen.value = !mobileMenuOpen.value
}

const logout = async () => {
  authToken.clearToken()

  // In non-public mode, force the login screen after logging out.
  if (!appConfig.publicEnabled && route.path !== '/login') {
    await router.push({ path: '/login', query: { next: route.fullPath } })
  }

  mobileMenuOpen.value = false
}
</script>

<template>
  <header class="border-b border-surface-border bg-surface-card/50 backdrop-blur-sm sticky top-0 z-50">
    <nav class="max-w-7xl mx-auto px-4 py-4">
      <div class="flex items-center justify-between">
        <!-- Logo -->
        <RouterLink to="/" class="flex items-center gap-2 group">
          <div class="w-8 h-8 bg-brand rounded-lg flex items-center justify-center group-hover:bg-brand-dark transition-colors">
            <span class="text-white font-bold text-lg">P</span>
          </div>
          <span class="text-xl font-bold text-zinc-50">pbuf<span class="text-brand">.io</span></span>
        </RouterLink>

        <!-- Desktop Navigation -->
        <div class="hidden md:flex items-center gap-6">
          <RouterLink 
            to="/modules" 
            class="text-zinc-300 hover:text-zinc-50 transition-colors font-medium"
            active-class="text-brand"
          >
            Modules
          </RouterLink>
          <a 
            href="https://github.com/pbufio/pbuf-cli" 
            target="_blank"
            rel="noopener noreferrer"
            class="text-zinc-300 hover:text-zinc-50 transition-colors font-medium"
          >
            Docs
          </a>
          <a 
            href="https://github.com/pbufio" 
            target="_blank"
            rel="noopener noreferrer"
            class="text-zinc-300 hover:text-zinc-50 transition-colors font-medium"
          >
            GitHub
          </a>
          <PButton v-if="!isLoggedIn" variant="secondary" to="/login">
            Login
          </PButton>
          <PButton v-if="isLoggedIn" variant="secondary" @click="logout">
            Logout
          </PButton>
          <PButton variant="primary" to="/modules">Get Started</PButton>
        </div>

        <!-- Mobile Menu Button -->
        <button 
          @click="toggleMobileMenu"
          class="md:hidden text-zinc-300 hover:text-zinc-50 p-2 rounded-lg cursor-pointer hover:bg-zinc-800/50 transition-colors"
        >
          <svg v-if="!mobileMenuOpen" class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
          <svg v-else class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Mobile Menu -->
      <div v-if="mobileMenuOpen" class="md:hidden mt-4 pb-4 border-t border-surface-border pt-4">
        <div class="flex flex-col gap-4">
          <RouterLink 
            to="/modules" 
            @click="toggleMobileMenu"
            class="text-zinc-300 hover:text-zinc-50 transition-colors font-medium"
            active-class="text-brand"
          >
            Modules
          </RouterLink>
          <a 
            href="https://github.com/pbufio/pbuf-cli" 
            target="_blank"
            rel="noopener noreferrer"
            class="text-zinc-300 hover:text-zinc-50 transition-colors font-medium"
          >
            Docs
          </a>
          <a 
            href="https://github.com/pbufio" 
            target="_blank"
            rel="noopener noreferrer"
            class="text-zinc-300 hover:text-zinc-50 transition-colors font-medium"
          >
            GitHub
          </a>
          <PButton variant="primary" to="/modules" @click="toggleMobileMenu">
            Get Started
          </PButton>
          <PButton v-if="!isLoggedIn" variant="secondary" to="/login" @click="toggleMobileMenu">
            Login
          </PButton>
          <PButton v-if="isLoggedIn" variant="secondary" @click="logout">
            Logout
          </PButton>
        </div>
      </div>
    </nav>
  </header>
</template>
