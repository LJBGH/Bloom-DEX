<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import {
  fetchDepthSnapshot,
  normalizeDepthPayload,
  resolveMarketWsUrl,
} from '../../api/marketDepth.js'

const props = defineProps({
  marketId: { type: Number, required: true },
  baseSymbol: { type: String, default: '—' },
  quoteSymbol: { type: String, default: '—' },
})

const emit = defineEmits(['book-reference'])

const book = ref({ bids: [], asks: [] })
/** @type {import('vue').Ref<{ tsMs: number, price: number, qty: number, side: string }[]>} */
const recentTrades = ref([])
const MAX_RECENT_TRADES = 80
const wsStatus = ref('offline')
const lastSeq = ref(null)
const lastError = ref('')
const activeTab = ref('book')
/** 合并价位（图1 右侧下拉）；0=不合并，其余为报价币 tick */
const mergeTick = ref(0)
/** both | bids | asks */
const viewMode = ref('both')

function mergePriceLevels(levels, tick) {
  if (!tick || tick <= 0 || !levels?.length) return levels.map((r) => ({ price: r.price, qty: r.qty }))
  const m = new Map()
  for (const r of levels) {
    const p = Math.round(r.price / tick) * tick
    if (!Number.isFinite(p) || p <= 0) continue
    m.set(p, (m.get(p) || 0) + r.qty)
  }
  return [...m.entries()].map(([price, qty]) => ({ price, qty }))
}

let ws = null
let reconnectTimer = null
let alive = true

function clearReconnect() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

function applyDepth(raw) {
  const { bids, asks } = normalizeDepthPayload(raw)
  book.value = { bids, asks }
  if (raw?.seq != null) lastSeq.value = raw.seq
}

function emitReference() {
  emit('book-reference', {
    bestBid: bestBid.value,
    bestAsk: bestAsk.value,
    mid: midPrice.value,
  })
}

function applyTrade(raw) {
  const mid = Number(raw?.market_id)
  if (mid !== Number(props.marketId)) return
  const price = parseFloat(String(raw?.price ?? '').replace(/,/g, ''))
  const qty = parseFloat(String(raw?.quantity ?? '').replace(/,/g, ''))
  if (!Number.isFinite(price) || !Number.isFinite(qty) || qty <= 0) return
  const side = String(raw?.side ?? '').toUpperCase()
  const tsMs = raw?.ts_ms != null ? Number(raw.ts_ms) : Date.now()
  const row = { tsMs, price, qty, side }
  recentTrades.value = [row, ...recentTrades.value].slice(0, MAX_RECENT_TRADES)
}

function handleWsMessage(ev) {
  try {
    const msg = JSON.parse(ev.data)
    if (msg.data == null) return
    const data = typeof msg.data === 'object' ? msg.data : JSON.parse(msg.data)

    if (msg.channel === 'depth') {
      if (Number(data.market_id) !== Number(props.marketId)) return
      applyDepth(data)
      return
    }
    if (msg.channel === 'trade') {
      applyTrade(data)
    }
  } catch {
    /* ignore */
  }
}

function subscribe() {
  if (!ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ op: 'subscribe', market_id: props.marketId }))
}

function teardownWs() {
  clearReconnect()
  if (ws) {
    ws.onopen = null
    ws.onmessage = null
    ws.onclose = null
    ws.onerror = null
    try {
      ws.close()
    } catch {
      /* noop */
    }
    ws = null
  }
}

function scheduleReconnect() {
  clearReconnect()
  if (!alive) return
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connectWs()
  }, 2500)
}

function connectWs() {
  teardownWs()
  if (!alive || !props.marketId) return

  wsStatus.value = 'connecting'
  lastError.value = ''

  let socket
  try {
    socket = new WebSocket(resolveMarketWsUrl())
  } catch (e) {
    wsStatus.value = 'offline'
    lastError.value = e?.message || 'WebSocket 创建失败'
    scheduleReconnect()
    return
  }

  ws = socket
  ws.onopen = () => {
    if (!alive || ws !== socket) return
    wsStatus.value = 'live'
    subscribe()
  }
  ws.onmessage = (ev) => {
    if (ws === socket) handleWsMessage(ev)
  }
  ws.onerror = () => {
    if (ws === socket) lastError.value = '连接异常'
  }
  ws.onclose = () => {
    if (ws === socket) {
      wsStatus.value = 'offline'
      ws = null
      scheduleReconnect()
    }
  }
}

async function loadSnapshot() {
  if (!props.marketId) return
  try {
    const resp = await fetchDepthSnapshot(props.marketId)
    const data = resp?.data
    if (data && typeof data === 'object' && Number(data.market_id) === Number(props.marketId)) {
      applyDepth(data)
    }
  } catch {
    /* 仅依赖 WS */
  }
}

async function bootstrap() {
  book.value = { bids: [], asks: [] }
  recentTrades.value = []
  lastSeq.value = null
  await loadSnapshot()
  connectWs()
}

watch(
  () => props.marketId,
  () => {
    alive = true
    bootstrap()
  },
  { immediate: true },
)

onUnmounted(() => {
  alive = false
  teardownWs()
})

const bestBid = computed(() => book.value.bids[0]?.price || 0)
const bestAsk = computed(() => book.value.asks[0]?.price || 0)
const midPrice = computed(() =>
  bestBid.value > 0 && bestAsk.value > 0 ? (bestBid.value + bestAsk.value) / 2 : 0,
)
const spreadPct = computed(() => {
  if (midPrice.value <= 0) return 0
  return ((bestAsk.value - bestBid.value) / midPrice.value) * 100
})

/** 卖盘：价低→价高累计后，展示价高在上 */
const askRows = computed(() => {
  const tick = Number(mergeTick.value) || 0
  const raw = mergePriceLevels(book.value.asks, tick)
  if (!raw.length) return []
  raw.sort((a, b) => a.price - b.price)
  let cum = 0
  const withCum = raw.map((r) => {
    cum += r.qty
    return { price: r.price, qty: r.qty, cumBase: cum }
  })
  const maxCum = cum || 1
  return [...withCum].reverse().map((r) => ({
    ...r,
    depthPct: Math.min(100, (r.cumBase / maxCum) * 100),
  }))
})

/** 买盘：价高→价低 */
const bidRows = computed(() => {
  const tick = Number(mergeTick.value) || 0
  const raw = mergePriceLevels(book.value.bids, tick)
  if (!raw.length) return []
  raw.sort((a, b) => b.price - a.price)
  let cum = 0
  const maxCum = raw.reduce((s, r) => s + r.qty, 0) || 1
  return raw.map((r) => {
    cum += r.qty
    return {
      price: r.price,
      qty: r.qty,
      cumBase: cum,
      depthPct: Math.min(100, (cum / maxCum) * 100),
    }
  })
})

const askQtyTotal = computed(() => askRows.value.reduce((acc, r) => acc + r.qty, 0))
const bidQtyTotal = computed(() => bidRows.value.reduce((acc, r) => acc + r.qty, 0))
const buyRatio = computed(() => {
  const sum = askQtyTotal.value + bidQtyTotal.value
  if (sum <= 0) return 50
  return (bidQtyTotal.value / sum) * 100
})

const isBookEmpty = computed(
  () => !book.value.asks?.length && !book.value.bids?.length,
)

const statusLabel = computed(() => {
  if (wsStatus.value === 'live') return '实时'
  if (wsStatus.value === 'connecting') return '连接中…'
  return '未连接'
})

const pairLabel = computed(() => {
  const b = props.baseSymbol || '—'
  const q = props.quoteSymbol || '—'
  if (b === '—' && q === '—') return ''
  return `${b}/${q}`
})

const headerPrice = computed(() => `价格(${props.quoteSymbol || '—'})`)
const headerQty = computed(() => `数量(${props.baseSymbol || '—'})`)
const headerCum = computed(() => `合计(${props.baseSymbol || '—'})`)

function fmtPrice(n) {
  const d = Number(mergeTick.value) > 0 ? 0 : 2
  return Number.isFinite(n) ? n.toFixed(d) : '—'
}

/** 成交价（不受合并档位影响） */
function fmtTradePrice(n) {
  if (!Number.isFinite(n)) return '—'
  const s = n.toFixed(8).replace(/\.?0+$/, '')
  return s || '0'
}

function fmtQty(n) {
  if (!Number.isFinite(n)) return '—'
  const s = n.toFixed(8).replace(/\.?0+$/, '')
  return s || '0'
}

function fmtCum(n) {
  return fmtQty(n)
}

function fmtTradeTime(tsMs) {
  if (!Number.isFinite(tsMs)) return '—'
  const d = new Date(tsMs)
  const pad = (n) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function tradePriceClass(side) {
  const s = String(side || '').toUpperCase()
  if (s === 'BUY') return 'ob-trade-buy'
  if (s === 'SELL') return 'ob-trade-sell'
  return ''
}

watch([bestBid, bestAsk, midPrice], emitReference, { immediate: true })
</script>

<template>
  <div class="ob-root">
    <div class="ob-tabs">
      <button
        type="button"
        class="ob-tab"
        :class="{ active: activeTab === 'book' }"
        @click="activeTab = 'book'"
      >
        订单簿
      </button>
      <button
        type="button"
        class="ob-tab"
        :class="{ active: activeTab === 'trades' }"
        @click="activeTab = 'trades'"
      >
        实时成交
      </button>
    </div>

    <div v-show="activeTab === 'book'" class="ob-book">
      <div class="ob-book-main">
      <!-- 图1：视图切换 + 右侧合并档位 -->
      <div class="ob-controls">
        <div class="ob-view-toggles" role="group" aria-label="订单簿视图">
          <button
            type="button"
            class="ob-view-btn"
            :class="{ active: viewMode === 'both' }"
            title="买卖盘"
            @click="viewMode = 'both'"
          >
            <span class="ob-view-icon" aria-hidden="true">▤</span>
          </button>
          <button
            type="button"
            class="ob-view-btn ob-view-bid"
            :class="{ active: viewMode === 'bids' }"
            title="仅买盘"
            @click="viewMode = 'bids'"
          >
            <span class="ob-view-icon" aria-hidden="true">▲</span>
          </button>
          <button
            type="button"
            class="ob-view-btn ob-view-ask"
            :class="{ active: viewMode === 'asks' }"
            title="仅卖盘"
            @click="viewMode = 'asks'"
          >
            <span class="ob-view-icon" aria-hidden="true">▼</span>
          </button>
        </div>
        <div class="ob-merge-wrap">
          <select v-model.number="mergeTick" class="ob-merge-select" title="价位合并">
            <option :value="0">—</option>
            <option :value="1">1</option>
            <option :value="5">5</option>
            <option :value="10">10</option>
            <option :value="50">50</option>
            <option :value="100">100</option>
          </select>
        </div>
      </div>
      <div class="ob-meta" v-if="pairLabel || lastSeq != null">
        <span v-if="pairLabel" class="ob-meta-pair">{{ pairLabel }}</span>
        <span class="ob-meta-dot" v-if="pairLabel">·</span>
        <span class="ob-status" :class="'ob-status-' + wsStatus">{{ statusLabel }}</span>
        <template v-if="lastSeq != null">
          <span class="ob-meta-dot">·</span>
          <span class="ob-seq">seq {{ lastSeq }}</span>
        </template>
      </div>
      <span v-if="lastError" class="ob-err">{{ lastError }}</span>

      <div class="ob-table">
        <div v-if="isBookEmpty" class="ob-placeholder">
          暂无盘口（请确认 market-ws 已启动且已订阅 market_id={{ marketId }}）
        </div>

        <div v-else class="ob-depth-split" :class="'ob-depth-split--' + viewMode">
          <!-- 表头独占一行，卖/买滚动区才可等分高度 -->
          <div class="ob-head ob-depth-head">
            <span class="ob-cell ob-col-price">{{ headerPrice }}</span>
            <span class="ob-cell ob-col-qty">{{ headerQty }}</span>
            <span class="ob-cell ob-col-cum">{{ headerCum }}</span>
          </div>

          <!-- 上：卖单区（与买单区等分 1fr） -->
          <div
            v-if="viewMode === 'both' || viewMode === 'asks'"
            class="ob-zone ob-zone--asks"
          >
            <div class="ob-zone-scroll ob-zone-scroll--asks">
              <div class="ob-zone-fill ob-zone-fill--asks">
                <div
                  v-for="(r, idx) in askRows"
                  :key="'ask-' + idx"
                  class="ob-row-wrap ob-ask"
                >
                  <div
                    class="ob-depth-bg ob-depth-ask"
                    :style="{ width: r.depthPct + '%' }"
                  />
                  <div class="ob-row">
                    <span class="ob-cell ob-col-price ob-ask-txt">{{ fmtPrice(r.price) }}</span>
                    <span class="ob-cell ob-col-qty ob-ask-txt">{{ fmtQty(r.qty) }}</span>
                    <span class="ob-cell ob-col-cum ob-ask-txt">{{ fmtCum(r.cumBase) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 中间价：与上下区留小间距（grid row-gap） -->
          <div
            v-if="viewMode === 'both' || viewMode === 'asks' || viewMode === 'bids'"
            class="ob-mid ob-mid--strip"
          >
            <div class="ob-mid-left">
              <span class="ob-mid-price">{{ midPrice > 0 ? fmtPrice(midPrice) : '—' }}</span>
              <span class="ob-mid-arrow" aria-hidden="true">↓</span>
            </div>
            <div
              class="ob-mid-right"
              :class="{ 'ob-mid-na': midPrice <= 0 }"
            >
              {{ midPrice > 0 ? spreadPct.toFixed(2) + '%' : '—' }}
            </div>
          </div>

          <!-- 下：买单区（与卖单区等分 1fr） -->
          <div
            v-if="viewMode === 'both' || viewMode === 'bids'"
            class="ob-zone ob-zone--bids"
          >
            <div class="ob-zone-scroll ob-zone-scroll--bids">
              <div
                v-for="(r, idx) in bidRows"
                :key="'bid-' + idx"
                class="ob-row-wrap ob-bid"
              >
                <div
                  class="ob-depth-bg ob-depth-bid"
                  :style="{ width: r.depthPct + '%' }"
                />
                <div class="ob-row">
                  <span class="ob-cell ob-col-price ob-bid-txt">{{ fmtPrice(r.price) }}</span>
                  <span class="ob-cell ob-col-qty ob-bid-txt">{{ fmtQty(r.qty) }}</span>
                  <span class="ob-cell ob-col-cum ob-bid-txt">{{ fmtCum(r.cumBase) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      </div>

      <div class="ob-ratio-bar">
        <div class="ob-ratio-buy" :style="{ width: buyRatio.toFixed(2) + '%' }">
          买 {{ buyRatio.toFixed(2) }}%
        </div>
        <div class="ob-ratio-sell">
          {{ (100 - buyRatio).toFixed(2) }}% 卖
        </div>
      </div>
    </div>

    <div v-show="activeTab === 'trades'" class="ob-trades">
      <div class="ob-trades-meta">
        <span class="ob-status" :class="'ob-status-' + wsStatus">{{ statusLabel }}</span>
      </div>
      <div class="ob-trades-table">
        <div class="ob-trades-head">
          <span class="ob-trades-cell ob-trades-time">时间</span>
          <span class="ob-trades-cell ob-trades-price">{{ headerPrice }}</span>
          <span class="ob-trades-cell ob-trades-qty">{{ headerQty }}</span>
        </div>
        <div class="ob-trades-body">
          <div
            v-for="(r, idx) in recentTrades"
            :key="idx + '-' + r.tsMs + '-' + r.price"
            class="ob-trades-row"
          >
            <span class="ob-trades-cell ob-trades-time">{{ fmtTradeTime(r.tsMs) }}</span>
            <span
              class="ob-trades-cell ob-trades-price"
              :class="tradePriceClass(r.side)"
            >{{ fmtTradePrice(r.price) }}</span>
            <span class="ob-trades-cell ob-trades-qty">{{ fmtQty(r.qty) }}</span>
          </div>
          <div v-if="recentTrades.length === 0" class="ob-trades-empty">
            暂无成交（有撮合后由 WebSocket 推送）
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ob-root {
  display: flex;
  flex-direction: column;
  min-height: 520px;
  height: 100%;
  background: #14161a;
  color: #eaecef;
  font-family: 'Segoe UI', system-ui, sans-serif;
  border-radius: 0;
  overflow: hidden;
  --ob-gutter-x: 12px;
}

.ob-tabs {
  display: flex;
  gap: 20px;
  padding: 0 var(--ob-gutter-x);
  border-bottom: 1px solid #2a2e36;
  background: #14161a;
}
.ob-tab {
  padding: 12px 2px 10px;
  border: none;
  background: none;
  color: #848e9c;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}
.ob-tab:hover {
  color: #b7bdc6;
}
.ob-tab.active {
  color: #f0b90b;
  border-bottom-color: #f0b90b;
}

.ob-trades {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  padding: 8px var(--ob-gutter-x) 12px;
}
.ob-trades-meta {
  padding-bottom: 8px;
  font-size: 11px;
}
.ob-trades-table {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.ob-trades-head,
.ob-trades-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr) minmax(0, 1fr);
  column-gap: 8px;
  align-items: center;
}
.ob-trades-head {
  padding: 4px 4px 8px;
  font-size: 12px;
  font-weight: 500;
  color: #848e9c;
  border-bottom: 1px solid #2a2e36;
}
.ob-trades-body {
  flex: 1;
  overflow-y: auto;
  max-height: 440px;
  padding-top: 4px;
}
.ob-trades-row {
  padding: 4px;
  font-size: 13px;
}
.ob-trades-cell {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  font-feature-settings: 'tnum' 1;
}
.ob-trades-time {
  color: #5e6673;
  font-size: 12px;
}
.ob-trades-price {
  text-align: center;
}
.ob-trades-qty {
  text-align: right;
  color: #eaecef;
}
.ob-trade-buy {
  color: #0ecb81;
}
.ob-trade-sell {
  color: #f6465d;
}
.ob-trades-empty {
  padding: 24px 8px;
  text-align: center;
  font-size: 12px;
  color: #5e6673;
}

.ob-book {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
/* 盘口区占满中间并内部滚动；买卖占比条固定在面板底部 */
.ob-book-main {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 图1：视图切换 + 合并档位（同高、垂直中线对齐；下拉收窄避免被浏览器撑满） */
.ob-controls {
  --ob-control-h: 32px;
  --ob-merge-w: 48px;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
  padding: 8px var(--ob-gutter-x) 6px;
}
.ob-view-toggles {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex: 0 0 auto;
  align-self: center;
}
.ob-view-btn {
  box-sizing: border-box;
  width: var(--ob-control-h);
  height: var(--ob-control-h);
  min-width: var(--ob-control-h);
  min-height: var(--ob-control-h);
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #2a2e36;
  border-radius: 0;
  background: #1e2026;
  color: #848e9c;
  cursor: pointer;
  line-height: 1;
  flex-shrink: 0;
}
.ob-view-btn:hover {
  color: #eaecef;
  border-color: #474d57;
}
.ob-view-btn.active {
  border-color: #f0b90b;
  color: #f0b90b;
  background: rgba(240, 185, 11, 0.08);
}
.ob-view-bid.active {
  border-color: #0ecb81;
  color: #0ecb81;
  background: rgba(14, 203, 129, 0.1);
}
.ob-view-ask.active {
  border-color: #f6465d;
  color: #f6465d;
  background: rgba(246, 70, 93, 0.1);
}
.ob-view-icon {
  font-size: 12px;
}
.ob-merge-wrap {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  margin-left: auto;
  height: var(--ob-control-h);
}
.ob-merge-select {
  box-sizing: border-box;
  display: block;
  width: var(--ob-merge-w);
  min-width: 0;
  max-width: var(--ob-merge-w);
  height: var(--ob-control-h);
  margin: 0;
  padding: 0 16px 0 6px;
  border-radius: 0;
  border: 1px solid #2a2e36;
  background-color: #1e2026;
  color: #eaecef;
  font-size: 12px;
  line-height: 1;
  cursor: pointer;
  text-align: center;
  font-variant-numeric: tabular-nums;
  -webkit-appearance: none;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='5' viewBox='0 0 8 5'%3E%3Cpath fill='%23848e9c' d='M0 0h8L4 5z'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 5px center;
}
.ob-merge-select::-ms-expand {
  display: none;
}
.ob-merge-select:focus {
  outline: 1px solid #474d57;
  outline-offset: 0;
}

.ob-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  padding: 0 var(--ob-gutter-x) 10px;
  font-size: 11px;
  color: #5e6673;
}
.ob-meta-pair {
  font-weight: 700;
  color: #b7bdc6;
}
.ob-meta-dot {
  color: #474d57;
  user-select: none;
}
.ob-status {
  font-weight: 600;
}
.ob-status-live {
  color: #0ecb81;
}
.ob-status-connecting {
  color: #f0b90b;
}
.ob-status-offline {
  color: #848e9c;
}
.ob-seq {
  color: #5e6673;
}

.ob-err {
  display: block;
  padding: 0 var(--ob-gutter-x) 6px;
  color: #f6465d;
  font-size: 11px;
}

/* 图1：价左、量中、合计右；等宽三列 */
.ob-table {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  padding: 2px var(--ob-gutter-x) 8px;
}
.ob-head,
.ob-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr);
  column-gap: 8px;
  align-items: center;
}
.ob-head {
  padding: 6px 0 8px;
  font-size: 12px;
  font-weight: 500;
  color: #848e9c;
  border-bottom: 1px solid #2a2e36;
  flex-shrink: 0;
}

/* 表头 + 卖/买 1fr 等分 + 中间价；row-gap 与最优价留缝 */
.ob-depth-split {
  flex: 1;
  min-height: 0;
  align-content: stretch;
}
.ob-depth-split--both {
  display: grid;
  grid-template-columns: 1fr;
  grid-template-rows: auto minmax(0, 1fr) auto minmax(0, 1fr);
  row-gap: 0;
}
.ob-depth-split--asks {
  display: grid;
  grid-template-columns: 1fr;
  grid-template-rows: auto minmax(0, 1fr) auto;
  row-gap: 0;
}
.ob-depth-split--bids {
  display: grid;
  grid-template-columns: 1fr;
  grid-template-rows: auto auto minmax(0, 1fr);
  row-gap: 0;
}
.ob-depth-head {
  grid-column: 1;
}
.ob-zone {
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
}
.ob-zone-scroll {
  flex: 1;
  min-height: 56px;
  overflow-y: auto;
  padding-top: 2px;
}
/* 买单：从上往下排（最优买价紧贴中间价下方） */
.ob-zone-scroll--bids {
  display: flex;
  flex-direction: column;
  gap: 1px;
}
/* 卖单：容器内底部对齐，少档位时上方留白 */
.ob-zone-scroll--asks {
  display: block;
}
.ob-zone-fill--asks {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  gap: 1px;
  box-sizing: border-box;
}
.ob-cell {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  font-feature-settings: 'tnum' 1;
}
.ob-col-price {
  text-align: left;
}
.ob-col-qty {
  text-align: center;
}
.ob-col-cum {
  text-align: right;
}

.ob-row-wrap {
  position: relative;
  border-radius: 0;
  min-height: 22px;
}
.ob-depth-bg {
  position: absolute;
  top: 0;
  bottom: 0;
  height: 100%;
  pointer-events: none;
  border-radius: 0;
  right: 0;
  left: auto;
}
.ob-depth-ask {
  background: rgba(246, 70, 93, 0.14);
}
.ob-depth-bid {
  background: rgba(14, 203, 129, 0.14);
}
.ob-row {
  position: relative;
  z-index: 1;
  padding: 3px 0;
  font-size: 12px;
  line-height: 1.35;
  font-family: ui-monospace, 'Cascadia Code', 'Courier New', monospace;
}
.ob-ask-txt {
  color: #f6465d;
  font-weight: 600;
}
.ob-bid-txt {
  color: #0ecb81;
  font-weight: 600;
}

.ob-placeholder {
  padding: 24px var(--ob-gutter-x);
  text-align: center;
  font-size: 12px;
  color: #5e6673;
}

/* 左大价+箭头，右价差% */
.ob-mid {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  border-top: 1px solid #2a2e36;
  border-bottom: 1px solid #2a2e36;
  margin: 4px 0;
  gap: 12px;
}
.ob-mid--strip {
  flex-shrink: 0;
  margin: 6px 0;
  padding: 6px 0;
  border-radius: 0;
  border: 1px solid #2a2e36;
  background: rgba(30, 32, 38, 0.6);
}
.ob-mid--strip .ob-mid-price {
  font-size: 18px;
}
.ob-mid-left {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.ob-mid-price {
  font-size: 20px;
  font-weight: 700;
  color: #f6465d;
  font-variant-numeric: tabular-nums;
}
.ob-mid-arrow {
  font-size: 14px;
  font-weight: 700;
  color: #f6465d;
  line-height: 1;
}
.ob-mid-right {
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: #f6465d;
}
.ob-mid-right.ob-mid-na {
  color: #5e6673;
}

.ob-ratio-bar {
  flex-shrink: 0;
  display: flex;
  margin: 8px var(--ob-gutter-x) 10px;
  border-radius: 0;
  overflow: hidden;
  border: 1px solid #2a2e36;
  font-size: 11px;
  font-weight: 600;
}
.ob-ratio-buy {
  text-align: center;
  padding: 5px 8px;
  background: rgba(14, 203, 129, 0.2);
  color: #0ecb81;
  min-width: 72px;
}
.ob-ratio-sell {
  flex: 1;
  text-align: center;
  padding: 5px 8px;
  background: rgba(246, 70, 93, 0.15);
  color: #f6465d;
}
</style>
