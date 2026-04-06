<script setup>
defineProps({
  markets: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
})

const model = defineModel({ type: [Number, null], required: true })
</script>

<template>
  <div class="trade-market-select">
    <select v-model="model" :disabled="loading || markets.length === 0">
      <option v-for="m in markets" :key="m.market_id" :value="m.market_id">
        {{ m.symbol }}
      </option>
    </select>
  </div>
</template>

<style scoped>
.trade-market-select select {
  width: 100%;
  padding: 8px 10px;
  border-radius: 0;
  border: 1px solid var(--dex-border, #2a2e36);
  background: var(--dex-bg-elevated, #1e2026);
  color: var(--dex-text, #eaecef);
  font-size: 13px;
  cursor: pointer;
}
.trade-market-select select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>

