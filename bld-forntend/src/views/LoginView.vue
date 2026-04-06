<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { loginApi } from '../api/user'

const router = useRouter()
const loading = ref(false)
const errorMsg = ref('')
const form = reactive({
  username: '',
  password: '',
})

async function onLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await loginApi(form)
    localStorage.setItem('bld_user_id', String(data.user_id))
    router.push('/dashboard')
  } catch (err) {
    const data = err?.response?.data
    if (typeof data === 'string' && data.trim()) {
      errorMsg.value = data
    } else if (data && typeof data === 'object') {
      errorMsg.value = data.message || data.error || JSON.stringify(data)
    } else {
      errorMsg.value = err?.message || 'Login failed'
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="card auth-card">
    <h2>Login</h2>
    <label>Username</label>
    <input v-model="form.username" placeholder="username" />
    <label>Password</label>
    <input v-model="form.password" type="password" placeholder="password" />
    <button :disabled="loading" @click="onLogin">
      {{ loading ? 'Logging in...' : 'Login' }}
    </button>
    <p v-if="errorMsg" class="error">{{ errorMsg }}</p>
    <p class="hint">
      No account?
      <router-link to="/register">Register now</router-link>
    </p>
  </section>
</template>

