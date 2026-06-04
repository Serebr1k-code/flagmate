<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <h1>Flagmate</h1>
        <p class="text-muted">CTF Attack-Defence Suricata UI</p>
      </div>

      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-group">
          <label class="label">Password</label>
          <input v-model="password" type="password" class="input" placeholder="Enter password" required />
        </div>

        <button type="submit" class="btn btn-primary btn-lg" :disabled="loading">
          {{ loading ? 'Logging in...' : 'Login' }}
        </button>

        <p v-if="error" class="error-text text-destructive">{{ error }}</p>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const password = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  loading.value = true
  error.value = ''
  try {
    await authStore.login(password.value)
    router.push('/')
  } catch (e: any) {
    error.value = e.response?.data || 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page { min-height: 100vh; display: flex; align-items: center; justify-content: center; background-color: var(--background); }
.login-card { background-color: var(--card); border: 1px solid var(--border); border-radius: 16px; padding: 32px; width: 100%; max-width: 400px; box-shadow: 0 8px 32px rgba(0,0,0,0.3); }
.login-header { text-align: center; margin-bottom: 24px; }
.login-header h1 { font-size: 28px; font-weight: 700; margin: 0 0 8px; color: var(--text); }
.login-header p { font-size: 14px; margin: 0; }
.login-form { display: flex; flex-direction: column; gap: 16px; }
.form-group { display: flex; flex-direction: column; gap: 4px; }
.error-text { font-size: 14px; text-align: center; margin-top: 8px; }
.text-muted { color: var(--text-muted); }
.text-destructive { color: var(--destructive); }
.label { font-size: 12px; font-weight: 500; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); }
</style>
