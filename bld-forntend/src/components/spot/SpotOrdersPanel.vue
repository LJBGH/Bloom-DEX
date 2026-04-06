<script setup>
import { computed, ref, watch } from 'vue'
import { listSpotOrdersApi, listSpotTradesApi } from '../../api/orders.js'

const props = defineProps({
  userId: { type: Number, default: 0 },
  marketId: { type: Number, default: null },
  /** 父组件下单成功后自增，触发刷新当前委托 */
  refreshTrigger: { type: Number, default: 0 },
})

const mainTab = ref('open') // open | history | trades
const hideOtherPairs = ref(true)
const loading = ref(false)
const errMsg = ref('')
const openItems = ref([])
const historyItems = ref([])
const tradeItems = ref([])

const effectiveMarketId = computed(() => {
  if (!hideOtherPairs.value || props.marketId == null) return 0
  return Number(props.marketId) || 0
})

async function fetchOpen() {
  if (props.userId <= 0) {
    openItems.value = []
    return
  }
  loading.value = true
  errMsg.value = ''
  try {
    const resp = await listSpotOrdersApi({
      user_id: props.userId,
      scope: 'open',
      market_id: effectiveMarketId.value || undefined,
      limit: 100,
    })
    openItems.value = resp?.data?.items || []
  } catch (e) {
    errMsg.value = e?.response?.data?.message || e?.message || '加载当前委托失败'
    openItems.value = []
  } finally {
    loading.value = false
  }
}

async function fetchHistory() {
  if (props.userId <= 0) {
    historyItems.value = []
    return
  }
  loading.value = true
  errMsg.value = ''
  try {
    const resp = await listSpotOrdersApi({
      user_id: props.userId,
      scope: 'history',
      market_id: effectiveMarketId.value || undefined,
      limit: 100,
    })
    historyItems.value = resp?.data?.items || []
  } catch (e) {
    errMsg.value = e?.response?.data?.message || e?.message || '加载历史委托失败'
    historyItems.value = []
  } finally {
    loading.value = false
  }
}

async function fetchTrades() {
  if (props.userId <= 0) {
    tradeItems.value = []
    return
  }
  loading.value = true
  errMsg.value = ''
  try {
    const resp = await listSpotTradesApi({
      user_id: props.userId,
      market_id: effectiveMarketId.value || undefined,
      limit: 100,
    })
    tradeItems.value = resp?.data?.items || []
  } catch (e) {
    errMsg.value = e?.response?.data?.message || e?.message || '加载历史成交失败'
    tradeItems.value = []
  } finally {
    loading.value = false
  }
}

function reloadActive() {
  if (mainTab.value === 'open') return fetchOpen()
  if (mainTab.value === 'history') return fetchHistory()
  return fetchTrades()
}

watch(
  () => [props.userId, props.marketId, hideOtherPairs.value, mainTab.value],
  () => {
    reloadActive()
  },
  { immediate: true },
)

watch(
  () => props.refreshTrigger,
  () => {
    if (props.refreshTrigger > 0) {
      fetchOpen()
      if (mainTab.value === 'history') fetchHistory()
      if (mainTab.value === 'trades') fetchTrades()
    }
  },
)

function fmtTime(ms) {
  if (ms == null) return '—'
  try {
    return new Date(ms).toLocaleString()
  } catch {
    return '—'
  }
}

function orderTypeLabel(t) {
  if (t === 'LIMIT') return '限价'
  if (t === 'MARKET') return '市价'
  return t || '—'
}

function statusLabel(s) {
  const m = {
    PENDING: '待成交',
    PARTIALLY_FILLED: '部分成交',
    FILLED: '完全成交',
    CANCELED: '已撤销',
    REJECTED: '已拒绝',
  }
  return m[s] || s || '—'
}

function parseNum(s) {
  const n = parseFloat(String(s || '').replace(/,/g, ''))
  return Number.isFinite(n) ? n : 0
}

/** 历史委托：折合成交额估算（限价用委托价×已成交量；市价用均价×已成交量） */
const historyTurnoverSummary = computed(() => {
  let buyQ = 0
  let sellQ = 0
  for (const r of historyItems.value) {
    const filled = parseNum(r.filled_quantity)
    if (filled <= 0) continue
    let px = parseNum(r.avg_fill_price)
    if (px <= 0) px = parseNum(r.price)
    const quote = filled * px
    if (r.side === 'BUY') buyQ += quote
    else sellQ += quote
  }
  return { buyQ, sellQ, total: buyQ + sellQ }
})

function tradeQuoteAmount(r) {
  return parseNum(r.price) * parseNum(r.quantity)
}
</script>

<template>
  <div class="sop-root">
    <div class="sop-head">
      <div class="sop-tabs">
        <button
          type="button"
          class="sop-tab"
          :class="{ active: mainTab === 'open' }"
          @click="mainTab = 'open'"
        >
          当前委托 ({{ openItems.length }})
        </button>
        <button
          type="button"
          class="sop-tab"
          :class="{ active: mainTab === 'history' }"
          @click="mainTab = 'history'"
        >
          历史委托 ({{ historyItems.length }})
        </button>
        <button
          type="button"
          class="sop-tab"
          :class="{ active: mainTab === 'trades' }"
          @click="mainTab = 'trades'"
        >
          历史成交 ({{ tradeItems.length }})
        </button>
      </div>
      <label class="sop-hide-pairs">
        <input v-model="hideOtherPairs" type="checkbox" />
        <span>隐藏其他交易对</span>
      </label>
    </div>

    <div v-if="mainTab === 'history'" class="sop-subbar">
      <span class="sop-sum">
        成交总额折合 {{ historyTurnoverSummary.total.toFixed(2) }} {{ historyItems[0]?.quote_symbol || 'USDT' }}
      </span>
      <span class="sop-sum-buy">（买入折合 {{ historyTurnoverSummary.buyQ.toFixed(2) }}</span>
      <span class="sop-sum-sep"> · </span>
      <span class="sop-sum-sell">卖出折合 {{ historyTurnoverSummary.sellQ.toFixed(2) }}）</span>
    </div>

    <div v-if="userId <= 0" class="sop-empty sop-hint">请登录后查看委托与成交</div>
    <template v-else>
      <div v-if="errMsg" class="sop-err">{{ errMsg }}</div>
      <div v-if="loading" class="sop-loading">加载中…</div>

      <div v-show="mainTab === 'open'" class="sop-table-wrap">
        <table v-if="openItems.length" class="sop-table">
          <thead>
            <tr>
              <th>交易对</th>
              <th>时间</th>
              <th>类型</th>
              <th>方向</th>
              <th>价格</th>
              <th>已成交</th>
              <th>剩余</th>
              <th>状态</th>
              <th>订单号</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in openItems" :key="r.order_id">
              <td>{{ r.symbol }}</td>
              <td class="sop-mono">{{ fmtTime(r.created_at_ms) }}</td>
              <td>{{ orderTypeLabel(r.order_type) }}</td>
              <td :class="r.side === 'BUY' ? 'sop-buy' : 'sop-sell'">
                {{ r.side === 'BUY' ? '买入' : '卖出' }}
              </td>
              <td class="sop-mono">{{ r.price ?? '市价' }}</td>
              <td class="sop-mono">{{ r.filled_quantity }}</td>
              <td class="sop-mono">{{ r.remaining_quantity }}</td>
              <td>{{ statusLabel(r.status) }}</td>
              <td class="sop-mono sop-id">{{ r.order_id }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else-if="!loading" class="sop-empty">
          <div class="sop-empty-icon" aria-hidden="true">📄</div>
          <p>暂无当前委托</p>
        </div>
      </div>

      <div v-show="mainTab === 'history'" class="sop-table-wrap">
        <table v-if="historyItems.length" class="sop-table">
          <thead>
            <tr>
              <th>交易对</th>
              <th>时间</th>
              <th>类型</th>
              <th>方向</th>
              <th>均价</th>
              <th>价格</th>
              <th>已成交</th>
              <th>委托量</th>
              <th>状态</th>
              <th>订单号</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in historyItems" :key="r.order_id">
              <td>{{ r.symbol }}</td>
              <td class="sop-mono">{{ fmtTime(r.created_at_ms) }}</td>
              <td>{{ orderTypeLabel(r.order_type) }}</td>
              <td :class="r.side === 'BUY' ? 'sop-buy' : 'sop-sell'">
                {{ r.side === 'BUY' ? '买入' : '卖出' }}
              </td>
              <td class="sop-mono">{{ r.avg_fill_price ?? '—' }}</td>
              <td class="sop-mono">{{ r.price ?? '—' }}</td>
              <td class="sop-mono">{{ r.filled_quantity }}</td>
              <td class="sop-mono">{{ r.quantity }}</td>
              <td>{{ statusLabel(r.status) }}</td>
              <td class="sop-mono sop-id">{{ r.order_id }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else-if="!loading" class="sop-empty">
          <div class="sop-empty-icon" aria-hidden="true">📄</div>
          <p>暂无历史委托</p>
          <p class="sop-empty-foot">
            仅展示近期订单记录；更多请前往
            <span class="sop-link">订单中心</span>
            （占位）
          </p>
        </div>
      </div>

      <div v-show="mainTab === 'trades'" class="sop-table-wrap">
        <table v-if="tradeItems.length" class="sop-table">
          <thead>
            <tr>
              <th>交易对</th>
              <th>时间</th>
              <th>方向</th>
              <th>角色</th>
              <th>成交价</th>
              <th>数量</th>
              <th>成交额</th>
              <th>手续费</th>
              <th>成交号</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in tradeItems" :key="r.trade_id">
              <td>{{ r.symbol }}</td>
              <td class="sop-mono">{{ fmtTime(r.created_at_ms) }}</td>
              <td :class="r.side === 'BUY' ? 'sop-buy' : 'sop-sell'">
                {{ r.side === 'BUY' ? '买入' : '卖出' }}
              </td>
              <td>{{ r.role === 'TAKER' ? '吃单' : '挂单' }}</td>
              <td class="sop-mono">{{ r.price }}</td>
              <td class="sop-mono">{{ r.quantity }}</td>
              <td class="sop-mono">{{ tradeQuoteAmount(r).toFixed(8).replace(/\.?0+$/, '') }}</td>
              <td class="sop-mono">{{ r.fee_amount }}</td>
              <td class="sop-mono sop-id">{{ r.trade_id }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else-if="!loading" class="sop-empty">
          <div class="sop-empty-icon" aria-hidden="true">📄</div>
          <p>暂无成交记录</p>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.sop-root {
  display: flex;
  flex-direction: column;
  min-height: 260px;
  background: #0b0e11;
  border: 1px solid #2a2e36;
  border-radius: 0;
  color: #eaecef;
  font-family: 'Segoe UI', system-ui, sans-serif;
  font-size: 12px;
}

.sop-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0 12px;
  border-bottom: 1px solid #2a2e36;
  background: #14161a;
}

.sop-tabs {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.sop-tab {
  margin: 0;
  padding: 12px 10px 10px;
  border: none;
  border-bottom: 2px solid transparent;
  background: none;
  color: #848e9c;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  border-radius: 0;
}

.sop-tab:hover {
  color: #b7bdc6;
}

.sop-tab.active {
  color: #eaecef;
  border-bottom-color: #eaecef;
}

.sop-hide-pairs {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #848e9c;
  font-size: 12px;
  cursor: pointer;
  user-select: none;
}

.sop-hide-pairs input {
  accent-color: #f0b90b;
}

.sop-subbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px;
  padding: 8px 12px;
  border-bottom: 1px solid #2a2e36;
  font-size: 11px;
  color: #848e9c;
}

.sop-sum-buy {
  color: #0ecb81;
}

.sop-sum-sell {
  color: #f6465d;
}

.sop-sum-sep {
  color: #5e6673;
}

.sop-err {
  padding: 8px 12px;
  color: #f6465d;
  font-size: 12px;
}

.sop-loading {
  padding: 8px 12px;
  color: #848e9c;
}

.sop-hint {
  padding: 32px 12px;
}

.sop-table-wrap {
  flex: 1;
  overflow: auto;
  min-height: 160px;
}

.sop-table {
  width: 100%;
  border-collapse: collapse;
  font-variant-numeric: tabular-nums;
}

.sop-table th,
.sop-table td {
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid #1e2026;
  white-space: nowrap;
}

.sop-table th {
  font-weight: 600;
  color: #848e9c;
  font-size: 11px;
  background: #0b0e11;
  position: sticky;
  top: 0;
  z-index: 1;
}

.sop-table tbody tr:hover {
  background: rgba(255, 255, 255, 0.02);
}

.sop-mono {
  font-family: ui-monospace, 'Cascadia Code', 'Consolas', monospace;
  font-size: 11px;
}

.sop-id {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sop-buy {
  color: #0ecb81;
  font-weight: 600;
}

.sop-sell {
  color: #f6465d;
  font-weight: 600;
}

.sop-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 16px 32px;
  color: #5e6673;
  text-align: center;
}

.sop-empty-icon {
  font-size: 40px;
  opacity: 0.25;
  margin-bottom: 8px;
}

.sop-empty-foot {
  margin-top: 12px;
  font-size: 11px;
  max-width: 480px;
  line-height: 1.5;
}

.sop-link {
  color: #3861fb;
  cursor: default;
}
</style>
