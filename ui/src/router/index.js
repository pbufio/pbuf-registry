import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import ModulesView from '@/views/ModulesView.vue'
import ModuleDetailView from '@/views/ModuleDetailView.vue'
import ModuleVersionView from '@/views/ModuleVersionView.vue'
import LoginView from '@/views/LoginView.vue'

import { appConfig } from '@/config/appConfig'
import { authToken } from '@/auth/authToken'
import { getAuthRedirect } from './authGuard.js'
import { buildRootRoute } from './routeUtils.js'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView
    },
    buildRootRoute({ publicEnabled: appConfig.publicEnabled, HomeView }),
    {
      path: '/modules',
      name: 'modules',
      component: ModulesView
    },
    {
      path: '/modules/:name+/tags/:tag',
      name: 'module-version',
      component: ModuleVersionView
    },
    {
      path: '/modules/:name+',
      name: 'module-detail',
      component: ModuleDetailView
    }
  ]
})

router.beforeEach((to) => {
  return (
    getAuthRedirect({
      publicEnabled: appConfig.publicEnabled,
      to,
      isLoggedIn: authToken.isLoggedIn.value
    }) || true
  )
})

export default router
