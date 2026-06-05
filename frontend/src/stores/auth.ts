import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/utils/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('token'))
  const isAuthenticated = ref(!!token.value)

  function init() {
    if (token.value) {
      isAuthenticated.value = true
    }
  }

  async function login(password: string) {
    const response = await api.post('/login', { password })
    token.value = String(response.data.token || '')
    isAuthenticated.value = true
    localStorage.setItem('token', token.value)
  }

  function logout() {
    token.value = null
    isAuthenticated.value = false
    localStorage.removeItem('token')
  }

  return {
    token,
    isAuthenticated,
    init,
    login,
    logout
  }
})
