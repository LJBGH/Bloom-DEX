<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import SpotTradePage from '../components/spot/SpotTradePage.vue'

const route = useRoute()
const activeTab = computed(() => (route.path.endsWith('/perp') ? 'perp' : 'spot'))
</script>

<template>
  <section class="trade-page">
    <div v-if="activeTab === 'spot'">
      <SpotTradePage />
    </div>
    <div v-else class="card" style="max-width: 720px; margin: 0 auto;">
      <p class="hint">合约交易区域（后续可接入永续合约模块）。</p>
    </div>
  </section>
</template>

