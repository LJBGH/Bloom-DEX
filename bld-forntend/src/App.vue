<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const navItems = [
  { path: '/markets', label: '行情' },
  { path: '/trade/spot', label: '现货' },
  { path: '/trade/perp', label: '合约' },
  { path: '/assets', label: '资产' },
  { path: '/wallet', label: '钱包' },
]

const isAuthPage = computed(() => ['login', 'register'].includes(route.name))
const isAuthed = computed(() => Number(localStorage.getItem('bld_user_id') || 0) > 0)

function logout() {
  localStorage.removeItem('bld_user_id')
  router.push('/login')
}
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="topbar-left">
        <router-link class="brand" to="/home">Bloom DEX</router-link>
        <nav class="nav" v-if="!isAuthPage">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="nav-link"
          >
            {{ item.label }}
          </router-link>
        </nav>
      </div>

      <div class="topbar-right">
        <span class="lang">简体中文</span>
        <span class="sep">|</span>
        <template v-if="!isAuthPage && isAuthed">
          <button class="btn link" @click="logout">退出登录</button>
        </template>
        <template v-else>
          <router-link class="nav-link" to="/login">登录</router-link>
          <router-link class="nav-link" to="/register">注册</router-link>
        </template>
      </div>
    </header>
    <main class="page">
      <router-view />
    </main>
  </div>
</template>
