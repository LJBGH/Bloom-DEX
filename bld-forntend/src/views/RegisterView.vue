<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { registerApi } from '../api/user'

const router = useRouter()
const loading = ref(false)
const okMsg = ref('')
const errorMsg = ref('')
const form = reactive({
  username: '',
  password: '',
})

async function onRegister() {
  loading.value = true
  okMsg.value = ''
  errorMsg.value = ''
  try {
    const { data } = await registerApi(form)
    okMsg.value = `Register success, user_id=${data.user_id}`
    setTimeout(() => router.push('/login'), 800)
  } catch (err) {
    const data = err?.response?.data
    if (typeof data === 'string' && data.trim()) {
      errorMsg.value = data
    } else if (data && typeof data === 'object') {
      errorMsg.value = data.message || data.error || JSON.stringify(data)
    } else {
      errorMsg.value = err?.message || 'Register failed'
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="card auth-card">
    <h2>Register</h2>
    <label>Username</label>
    <input v-model="form.username" placeholder="username" />
    <label>Password</label>
    <input v-model="form.password" type="password" placeholder="password" />
    <button :disabled="loading" @click="onRegister">
      {{ loading ? 'Registering...' : 'Register' }}
    </button>
    <p v-if="okMsg" class="ok">{{ okMsg }}</p>
    <p v-if="errorMsg" class="error">{{ errorMsg }}</p>
    <p class="hint">
      Already have account?
      <router-link to="/login">Back to login</router-link>
    </p>
  </section>
</template>

