import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import api from '@/api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
      meta: { guest: true },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/Register.vue'),
      meta: { guest: true },
    },
    {
      path: '/',
      name: 'home',
      component: () => import('@/views/Home.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/profile',
      name: 'profile',
      component: () => import('@/views/Profile.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin',
      name: 'admin',
      component: () => import('@/views/Admin.vue'),
      meta: { requiresAuth: true, admin: true },
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // Check system init status
  if (to.name !== 'register') {
    try {
      const res = await api.get('/system/init-status')
      if (!res.data.initialized) {
        return { name: 'register' }
      }
    } catch {
      // ignore
    }
  }

  if (to.meta.guest) {
    return true
  }

  // For pages that require auth, validate token with server
  if (to.meta.requiresAuth) {
    if (!auth.isLoggedIn) {
      return { name: 'login' }
    }

    // Verify token is still valid with the server
    try {
      await api.get('/user/profile')
    } catch {
      auth.logout()
      return { name: 'login' }
    }
  }

  if (!auth.isLoggedIn) {
    return { name: 'login' }
  }

  if (to.meta.admin && !auth.isAdmin) {
    return { name: 'home' }
  }

  return true
})

export default router
