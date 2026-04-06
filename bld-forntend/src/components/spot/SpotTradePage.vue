<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { listSpotMarketsApi, createSpotOrderApi } from '../../api/orders.js'
import { listAssetsApi } from '../../api/wallet.js'
import SpotMarketSelector from './SpotMarketSelector.vue'
import KLineChart from './KLineChart.vue'
import OrderBook from './OrderBook.vue'
import SpotOrdersPanel from './SpotOrdersPanel.vue'

const userId = computed(() => Number(localStorage.getItem('bld_user_id') || 0))

const markets = ref([])
const loadingMarkets = ref(false)
const selectedMarketId = ref(null)

const selectedMarket = computed(() => {
  return markets.value.find((m) => m.market_id === selectedMarketId.value) || null
})

const chartPairLabel = computed(() => {
  const m = selectedMarket.value
  if (!m) return '—'
  return `${m.base_symbol}/${m.quote_symbol}`
})

const availQuote = ref('0')
const availBase = ref('0')

/** 盘口参考（市价数量/成交额换算） */
const bookRef = reactive({ bestBid: 0, bestAsk: 0, mid: 0 })

function onBookReference(payload) {
  bookRef.bestBid = Number(payload?.bestBid) || 0
  bookRef.bestAsk = Number(payload?.bestAsk) || 0
  bookRef.mid = Number(payload?.mid) || 0
}

const amountSliderPct = ref(0)

/** 限价：最近编辑的数量侧，用于随价格联动 */
const limitSyncSource = ref(null)

/** 市价买入：成交额 | 数量（数量模式由盘口推算 max_quote_amount） */
const marketBuyMode = ref('turnover')
/** 市价卖出：数量 | 成交额 */
const marketSellMode = ref('quantity')

const orderForm = reactive({
  side: 'BUY',
  order_type: 'LIMIT',
  price: '',
  quantity: '',
  quote_turnover: '',
  max_quote_amount: '',
  market_sell_turnover: '',
})

/** 买入展示报价币可用，卖出展示基币可用 */
const availableDisplay = computed(() => {
  if (userId.value <= 0) return '—'
  const m = selectedMarket.value
  if (!m) return '—'
  if (orderForm.side === 'BUY') {
    return `${m.quote_symbol} ${availQuote.value}`
  }
  return `${m.base_symbol} ${availBase.value}`
})

const submitting = ref(false)
const orderResultMsg = ref('')
/** 下单成功后递增，驱动底部订单面板刷新当前委托 */
const ordersRefreshKey = ref(0)

function parseBalanceNum(s) {
  const n = parseFloat(String(s || '').replace(/,/g, ''))
  return Number.isFinite(n) ? n : 0
}

function trimAmountString(n) {
  if (!Number.isFinite(n) || n <= 0) return ''
  let s = n.toFixed(12).replace(/\.?0+$/, '')
  return s || ''
}

function limitPriceNum() {
  return parseBalanceNum(orderForm.price)
}

function syncLimitTurnoverFromQty() {
  const p = limitPriceNum()
  const q = parseBalanceNum(orderForm.quantity)
  if (p > 0 && q > 0) orderForm.quote_turnover = trimAmountString(p * q)
  else if (q <= 0) orderForm.quote_turnover = ''
}

function syncLimitQtyFromTurnover() {
  const p = limitPriceNum()
  const t = parseBalanceNum(orderForm.quote_turnover)
  if (p > 0 && t > 0) orderForm.quantity = trimAmountString(t / p)
  else if (t <= 0) orderForm.quantity = ''
}

function onLimitQtyInput() {
  limitSyncSource.value = 'quantity'
  syncLimitTurnoverFromQty()
}

function onLimitTurnoverInput() {
  limitSyncSource.value = 'turnover'
  syncLimitQtyFromTurnover()
}

function limitPriceAfterChange() {
  if (orderForm.order_type !== 'LIMIT') return
  if (limitSyncSource.value === 'quantity') syncLimitTurnoverFromQty()
  else if (limitSyncSource.value === 'turnover') syncLimitQtyFromTurnover()
}

function bumpLimitPrice(dir) {
  const p = limitPriceNum()
  if (p <= 0 && bookRef.mid > 0) {
    orderForm.price = trimAmountString(bookRef.mid)
    limitPriceAfterChange()
    return
  }
  if (p <= 0) return
  const factor = dir > 0 ? 1.001 : 0.999
  const next = p * factor
  orderForm.price = trimAmountString(next) || String(next)
  limitPriceAfterChange()
}

watch(
  () => orderForm.price,
  () => {
    if (orderForm.order_type !== 'LIMIT') return
    if (limitSyncSource.value === 'quantity') syncLimitTurnoverFromQty()
    else if (limitSyncSource.value === 'turnover') syncLimitQtyFromTurnover()
  },
)

function syncMarketSellQtyFromTurnover() {
  const bid = bookRef.bestBid
  const t = parseBalanceNum(orderForm.market_sell_turnover)
  if (bid > 0 && t > 0) orderForm.quantity = trimAmountString(t / bid)
  else if (t <= 0) orderForm.quantity = ''
}

function onMarketSellTurnoverInput() {
  syncMarketSellQtyFromTurnover()
}

function applyAmountSlider() {
  const pct = Math.min(100, Math.max(0, Number(amountSliderPct.value) || 0)) / 100

  if (orderForm.order_type === 'LIMIT') {
    if (orderForm.side === 'BUY') {
      limitSyncSource.value = 'turnover'
      const max = parseBalanceNum(availQuote.value)
      orderForm.quote_turnover = pct <= 0 ? '' : trimAmountString(max * pct)
      syncLimitQtyFromTurnover()
    } else {
      limitSyncSource.value = 'quantity'
      const maxB = parseBalanceNum(availBase.value)
      orderForm.quantity = pct <= 0 ? '' : trimAmountString(maxB * pct)
      syncLimitTurnoverFromQty()
    }
    return
  }

  if (orderForm.side === 'BUY') {
    if (marketBuyMode.value === 'turnover') {
      const max = parseBalanceNum(availQuote.value)
      orderForm.max_quote_amount = pct <= 0 ? '' : trimAmountString(max * pct)
    } else {
      const maxB = parseBalanceNum(availBase.value)
      orderForm.quantity = pct <= 0 ? '' : trimAmountString(maxB * pct)
    }
    return
  }

  if (marketSellMode.value === 'quantity') {
    const maxB = parseBalanceNum(availBase.value)
    orderForm.quantity = pct <= 0 ? '' : trimAmountString(maxB * pct)
  } else {
    const bid = bookRef.bestBid
    if (bid <= 0) {
      orderForm.market_sell_turnover = ''
      orderForm.quantity = ''
      return
    }
    const maxProceeds = parseBalanceNum(availBase.value) * bid
    orderForm.market_sell_turnover = pct <= 0 ? '' : trimAmountString(maxProceeds * pct)
    syncMarketSellQtyFromTurnover()
  }
}

function limitQuantityFromTurnover() {
  const qt = String(orderForm.quote_turnover || '').trim()
  const pr = String(orderForm.price || '').trim()
  if (!qt || parseBalanceNum(qt) <= 0) return { ok: false, msg: '请填写成交额（报价币）' }
  if (!pr || parseBalanceNum(pr) <= 0) return { ok: false, msg: '请先填写有效的价格' }
  const qn = parseBalanceNum(qt)
  const pn = parseBalanceNum(pr)
  const raw = qn / pn
  if (!Number.isFinite(raw) || raw <= 0) return { ok: false, msg: '成交额与价格无法算出有效数量' }
  return { ok: true, quantity: trimAmountString(raw) }
}

async function loadMarkets() {
  loadingMarkets.value = true
  orderResultMsg.value = ''
  try {
    const resp = await listSpotMarketsApi({ status: 'ACTIVE' })
    markets.value = resp?.data?.items || []
    if (!selectedMarketId.value && markets.value.length > 0) {
      selectedMarketId.value = markets.value[0].market_id
    }
  } catch (e) {
    markets.value = []
    orderResultMsg.value = e?.response?.data?.message || e?.message || '加载交易对失败'
  } finally {
    loadingMarkets.value = false
  }
}

async function submitOrder() {
  orderResultMsg.value = ''
  if (userId.value <= 0) {
    orderResultMsg.value = '请先登录'
    return
  }
  if (!selectedMarket.value) {
    orderResultMsg.value = '请先选择交易对'
    return
  }

  let quantityToSend = String(orderForm.quantity || '').trim()
  let maxQuoteForPayload = String(orderForm.max_quote_amount || '').trim()

  if (orderForm.order_type === 'LIMIT') {
    if (!orderForm.price || parseBalanceNum(orderForm.price) <= 0) {
      orderResultMsg.value = '请填写有效的委托价格'
      return
    }
    const pq = parseBalanceNum(quantityToSend)
    if (pq <= 0) {
      const r = limitQuantityFromTurnover()
      if (!r.ok) {
        orderResultMsg.value = r.msg
        return
      }
      quantityToSend = r.quantity
    } else if (parseBalanceNum(orderForm.quote_turnover) <= 0) {
      syncLimitTurnoverFromQty()
    }
  } else if (orderForm.order_type === 'MARKET' && orderForm.side === 'BUY') {
    if (marketBuyMode.value === 'turnover') {
      if (!maxQuoteForPayload || parseBalanceNum(maxQuoteForPayload) <= 0) {
        orderResultMsg.value = '请填写成交额（报价币上限）'
        return
      }
      const q = String(orderForm.quantity || '').trim()
      if (q !== '') {
        const n = parseFloat(q.replace(/,/g, ''))
        if (!Number.isFinite(n)) {
          orderResultMsg.value = '买入数量格式无效'
          return
        }
        if (n < 0) {
          orderResultMsg.value = '买入数量不能为负'
          return
        }
      }
      quantityToSend = !q || parseBalanceNum(q) <= 0 ? '0' : q
    } else {
      if (!quantityToSend || parseBalanceNum(quantityToSend) <= 0) {
        orderResultMsg.value = '请填写买入数量'
        return
      }
      const ask = bookRef.bestAsk
      if (ask <= 0) {
        orderResultMsg.value = '暂无卖盘参考价，无法按数量下市价买'
        return
      }
      maxQuoteForPayload = ''
    }
  } else if (orderForm.order_type === 'MARKET' && orderForm.side === 'SELL') {
    if (marketSellMode.value === 'quantity') {
      if (!quantityToSend || parseBalanceNum(quantityToSend) <= 0) {
        orderResultMsg.value = '请填写卖出数量'
        return
      }
    } else {
      const bid = bookRef.bestBid
      if (bid <= 0) {
        orderResultMsg.value = '暂无买盘参考价，无法按成交额下市价卖'
        return
      }
      const mq = String(orderForm.market_sell_turnover || '').trim()
      if (!mq || parseBalanceNum(mq) <= 0) {
        orderResultMsg.value = '请填写成交额（报价币）'
        return
      }
      const raw = parseBalanceNum(mq) / bid
      if (!Number.isFinite(raw) || raw <= 0) {
        orderResultMsg.value = '成交额与参考价无法得到有效数量'
        return
      }
      quantityToSend = trimAmountString(raw) || String(raw)
    }
  } else {
    if (!quantityToSend || parseBalanceNum(quantityToSend) <= 0) {
      orderResultMsg.value = '数量必须为正数'
      return
    }
  }

  submitting.value = true
  try {
    const clientOrderId =
      globalThis?.crypto?.randomUUID?.() ||
      `co_${Date.now()}_${Math.random().toString(16).slice(2)}`
    const isMarketBuy = orderForm.order_type === 'MARKET' && orderForm.side === 'BUY'
    const payload = {
      user_id: userId.value,
      market_id: selectedMarket.value.market_id,
      side: orderForm.side,
      order_type: orderForm.order_type,
      price: orderForm.order_type === 'LIMIT' ? orderForm.price : null,
      quantity: quantityToSend,
      client_order_id: clientOrderId,
    }
    if (isMarketBuy) {
      if (marketBuyMode.value === 'turnover') {
        payload.max_quote_amount = maxQuoteForPayload
      } else {
        payload.reference_price = String(bookRef.bestAsk)
      }
    }
    if (orderForm.order_type === 'MARKET') {
      payload.amount_input_mode =
        orderForm.side === 'BUY'
          ? marketBuyMode.value === 'turnover'
            ? 'TURNOVER'
            : 'QUANTITY'
          : marketSellMode.value === 'turnover'
            ? 'TURNOVER'
            : 'QUANTITY'
    }
    const resp = await createSpotOrderApi(payload)
    const data = resp?.data || {}
    orderResultMsg.value = `已提交订单：${data.order_id || ''}（${data.status || 'PENDING'}）`
    ordersRefreshKey.value += 1
  } catch (e) {
    orderResultMsg.value = e?.response?.data?.message || e?.message || '下单失败'
  } finally {
    submitting.value = false
  }
}

function setOrderType(t) {
  orderForm.order_type = t
  amountSliderPct.value = 0
  limitSyncSource.value = null
  if (t === 'MARKET') {
    orderForm.price = ''
    orderForm.quote_turnover = ''
    orderForm.quantity = ''
    orderForm.max_quote_amount = ''
    orderForm.market_sell_turnover = ''
    marketBuyMode.value = 'turnover'
    marketSellMode.value = 'quantity'
  } else {
    orderForm.max_quote_amount = ''
    orderForm.market_sell_turnover = ''
  }
}

watch(marketBuyMode, () => {
  amountSliderPct.value = 0
  if (marketBuyMode.value === 'turnover') orderForm.quantity = ''
  else orderForm.max_quote_amount = ''
})

watch(marketSellMode, () => {
  amountSliderPct.value = 0
  if (marketSellMode.value === 'quantity') {
    orderForm.market_sell_turnover = ''
  } else {
    orderForm.quantity = ''
    syncMarketSellQtyFromTurnover()
  }
})

watch(() => orderForm.side, () => {
  amountSliderPct.value = 0
  limitSyncSource.value = null
})

watch(
  () => bookRef.bestBid,
  () => {
    if (orderForm.order_type !== 'MARKET' || orderForm.side !== 'SELL') return
    if (marketSellMode.value !== 'turnover') return
    if (parseBalanceNum(orderForm.market_sell_turnover) > 0) syncMarketSellQtyFromTurnover()
  },
)

async function refreshAvailable() {
  availQuote.value = '0'
  availBase.value = '0'
  if (userId.value <= 0) return
  const m = selectedMarket.value
  if (!m?.quote_asset_id || !m?.base_asset_id) return

  try {
    const [rq, rb] = await Promise.all([
      listAssetsApi(userId.value, m.quote_asset_id),
      listAssetsApi(userId.value, m.base_asset_id),
    ])
    const qItem = rq?.data?.items?.[0]
    const bItem = rb?.data?.items?.[0]
    availQuote.value = qItem?.available_balance ?? '0'
    availBase.value = bItem?.available_balance ?? '0'
  } catch {
    // keep placeholder
  }
}

watch(selectedMarketId, () => {
  refreshAvailable()
})

onMounted(() => {
  loadMarkets()
})
</script>

<template>
  <div class="trade-spot">
    <div class="trade-spot-grid">
      <div class="trade-panel trade-spot-module-1">
        <div class="trade-panel-head">币种选择</div>
        <SpotMarketSelector
          v-model="selectedMarketId"
          :markets="markets"
          :loading="loadingMarkets"
        />
      </div>

      <div class="trade-panel">
        <div class="trade-panel-head">图表</div>
        <KLineChart
          v-if="selectedMarketId"
          :market-id="selectedMarketId"
          :pair-label="chartPairLabel"
        />
        <div v-else class="trade-empty">请选择交易对</div>
      </div>

      <div class="trade-panel trade-panel-ob">
        <OrderBook
          v-if="selectedMarketId"
          :market-id="selectedMarketId"
          :base-symbol="selectedMarket?.base_symbol || '—'"
          :quote-symbol="selectedMarket?.quote_symbol || '—'"
          @book-reference="onBookReference"
        />
        <div v-else class="trade-empty">请选择交易对</div>
      </div>

      <div class="trade-panel trade-order-panel">
        <div class="trade-panel-head trade-order-head">买/卖交易</div>

        <div class="trade-form">
          <div class="trade-side-toggle">
            <button
              class="trade-side-btn"
              :class="{ active: orderForm.side === 'BUY' }"
              @click="orderForm.side = 'BUY'"
            >
              买入
            </button>
            <button
              class="trade-side-btn trade-side-sell"
              :class="{ active: orderForm.side === 'SELL' }"
              @click="orderForm.side = 'SELL'"
            >
              卖出
            </button>
          </div>

          <div class="trade-type-toggle">
            <button
              type="button"
              class="trade-type-btn"
              :class="{ active: orderForm.order_type === 'LIMIT' }"
              @click="setOrderType('LIMIT')"
            >
              限价委托
            </button>
            <button
              type="button"
              class="trade-type-btn"
              :class="{ active: orderForm.order_type === 'MARKET' }"
              @click="setOrderType('MARKET')"
            >
              市价委托
            </button>
          </div>

          <div class="trade-form-row">
            <label class="trade-label">可用</label>
            <div class="trade-avail">{{ availableDisplay }}</div>
          </div>

          <!-- 价格：限价可编辑 + 步进；市价只读「市价」 -->
          <div class="trade-form-row" v-if="orderForm.order_type === 'LIMIT'">
            <label class="trade-label">价格（{{ selectedMarket?.quote_symbol || '—' }}）</label>
            <div class="trade-input-stepper">
              <input
                v-model="orderForm.price"
                type="text"
                inputmode="decimal"
                placeholder="请输入价格"
                class="trade-input-dark trade-input-grow"
              />
              <div class="trade-step-btns">
                <button type="button" class="trade-step-btn" @click="bumpLimitPrice(1)">▲</button>
                <button type="button" class="trade-step-btn" @click="bumpLimitPrice(-1)">▼</button>
              </div>
            </div>
          </div>
          <div class="trade-form-row" v-else>
            <label class="trade-label">价格</label>
            <input
              type="text"
              readonly
              value="市价"
              class="trade-input-dark trade-input-readonly"
            />
          </div>

          <!-- 限价：数量 → 滑块 → 成交额 -->
          <template v-if="orderForm.order_type === 'LIMIT'">
            <div class="trade-form-row">
              <label class="trade-label">数量</label>
              <div class="trade-input-suffix-wrap">
                <input
                  v-model="orderForm.quantity"
                  type="text"
                  inputmode="decimal"
                  class="trade-input-dark trade-input-suffix-input"
                  placeholder="请输入数量"
                  @input="onLimitQtyInput"
                />
                <span class="trade-input-suffix">{{ selectedMarket?.base_symbol || '—' }}</span>
              </div>
            </div>
            <div class="trade-slider-block">
              <div class="trade-range-wrap">
                <input
                  v-model.number="amountSliderPct"
                  type="range"
                  min="0"
                  max="100"
                  step="1"
                  class="trade-range"
                  :style="{ '--range-pct': `${Number(amountSliderPct) || 0}%` }"
                  @input="applyAmountSlider"
                />
              </div>
              <div class="trade-slider-ticks">
                <span>0%</span>
                <span>25%</span>
                <span>50%</span>
                <span>75%</span>
                <span>100%</span>
              </div>
            </div>
            <div class="trade-form-row">
              <label class="trade-label">成交额</label>
              <div class="trade-input-suffix-wrap">
                <input
                  v-model="orderForm.quote_turnover"
                  type="text"
                  inputmode="decimal"
                  class="trade-input-dark trade-input-suffix-input"
                  placeholder="请输入成交额"
                  @input="onLimitTurnoverInput"
                />
                <span class="trade-input-suffix">{{ selectedMarket?.quote_symbol || '—' }}</span>
              </div>
            </div>
          </template>

          <!-- 市价：标签下拉 + 单一金额输入 + 滑块 -->
          <template v-else>
            <div class="trade-form-row trade-amount-mode-row">
              <label class="trade-label trade-amount-label">
                <select v-if="orderForm.side === 'BUY'" v-model="marketBuyMode" class="trade-mode-select">
                  <option value="turnover">成交额</option>
                  <option value="quantity">数量</option>
                </select>
                <select v-else v-model="marketSellMode" class="trade-mode-select">
                  <option value="quantity">数量</option>
                  <option value="turnover">成交额</option>
                </select>
              </label>
              <div v-if="orderForm.side === 'BUY'" class="trade-input-suffix-wrap">
                <input
                  v-if="marketBuyMode === 'turnover'"
                  v-model="orderForm.max_quote_amount"
                  type="text"
                  inputmode="decimal"
                  class="trade-input-dark trade-input-suffix-input"
                  placeholder="请输入成交额"
                />
                <template v-else>
                  <input
                    v-model="orderForm.quantity"
                    type="text"
                    inputmode="decimal"
                    class="trade-input-dark trade-input-suffix-input"
                    placeholder="请输入数量"
                  />
                  <span class="trade-input-suffix">{{ selectedMarket?.base_symbol || '—' }}</span>
                </template>
                <span
                  v-if="marketBuyMode === 'turnover'"
                  class="trade-input-suffix"
                >{{ selectedMarket?.quote_symbol || '—' }}</span>
              </div>
              <div v-else class="trade-input-suffix-wrap">
                <input
                  v-if="marketSellMode === 'quantity'"
                  v-model="orderForm.quantity"
                  type="text"
                  inputmode="decimal"
                  class="trade-input-dark trade-input-suffix-input"
                  placeholder="请输入数量"
                />
                <input
                  v-else
                  v-model="orderForm.market_sell_turnover"
                  type="text"
                  inputmode="decimal"
                  class="trade-input-dark trade-input-suffix-input"
                  placeholder="请输入成交额"
                  @input="onMarketSellTurnoverInput"
                />
                <span class="trade-input-suffix">{{
                  marketSellMode === 'quantity'
                    ? selectedMarket?.base_symbol || '—'
                    : selectedMarket?.quote_symbol || '—'
                }}</span>
              </div>
            </div>
            <div class="trade-slider-block">
              <div class="trade-range-wrap">
                <input
                  v-model.number="amountSliderPct"
                  type="range"
                  min="0"
                  max="100"
                  step="1"
                  class="trade-range"
                  :style="{ '--range-pct': `${Number(amountSliderPct) || 0}%` }"
                  @input="applyAmountSlider"
                />
              </div>
              <div class="trade-slider-ticks">
                <span>0%</span>
                <span>25%</span>
                <span>50%</span>
                <span>75%</span>
                <span>100%</span>
              </div>
            </div>
          </template>

          <button
            class="trade-submit"
            :class="{ 'trade-submit-sell': orderForm.side === 'SELL' }"
            :disabled="submitting"
            @click="submitOrder"
          >
            {{ submitting ? '提交中...' : orderForm.side === 'BUY' ? '买入' : '卖出' }}
          </button>

          <div
            v-if="orderResultMsg"
            class="trade-msg"
            :class="orderResultMsg.includes('失败') ? 'trade-err' : 'trade-ok'"
          >
            {{ orderResultMsg }}
          </div>
        </div>
      </div>

      <div class="trade-spot-module-5">
        <SpotOrdersPanel
          :user-id="userId"
          :market-id="selectedMarketId"
          :refresh-trigger="ordersRefreshKey"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.trade-spot {
  width: 100%;
}

.trade-spot-grid {
  display: grid;
  grid-template-columns: 250px 1fr 300px 300px;
  grid-template-rows: auto auto;
  gap: 12px;
}
.trade-spot-grid > .trade-panel:not(.trade-spot-module-5) {
  min-height: 600px;
}
.trade-spot-module-1 :deep(.trade-market-select) {
  justify-content: flex-start;
}
.trade-spot-module-1 :deep(.trade-market-select select) {
  max-width: 100%;
  width: 100%;
}
.trade-spot-module-5 {
  grid-column: 1 / -1;
  grid-row: 2;
}

.trade-panel {
  background: var(--dex-bg-panel, #14161a);
  border: 1px solid var(--dex-border, #2a2e36);
  border-radius: 0;
  padding: 10px;
  color: var(--dex-text, #eaecef);
}
.trade-panel-ob {
  padding: 0;
  overflow: hidden;
  background: #14161a;
  border-color: #2a2e36;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.trade-panel-ob :deep(.ob-root) {
  flex: 1;
  min-height: 0;
}
.trade-panel-muted {
  background: var(--dex-bg-elevated, #1e2026);
}
.trade-panel-head {
  font-weight: 700;
  color: var(--dex-text, #eaecef);
  font-size: 14px;
  padding: 6px 4px 10px;
  border-bottom: 1px solid var(--dex-border, #2a2e36);
  margin: -2px -2px 10px;
}
.trade-empty {
  color: var(--dex-text-secondary, #848e9c);
  font-size: 13px;
  padding: 16px 8px;
}

.trade-order-panel {
  background: #1b1e26;
  border-color: #2d313c;
  color: #e5e7eb;
}
.trade-order-head {
  color: #f9fafb !important;
  border-bottom-color: #2d313c !important;
}
.trade-form {
  display: grid;
  gap: 12px;
}
.trade-side-toggle {
  display: flex;
  gap: 8px;
}
.trade-type-toggle {
  display: flex;
  gap: 16px;
  align-items: center;
  padding: 4px 0;
}
.trade-type-btn {
  padding: 4px 0;
  border: none;
  background: transparent;
  color: #9ca3af;
  cursor: pointer;
  font-weight: 700;
  font-size: 14px;
  font-family: Arial, Helvetica, sans-serif;
}
.trade-type-btn.active {
  color: #f0b90b;
}
.trade-avail {
  font-size: 13px;
  color: #cbd5e1;
  font-family: Arial, Helvetica, sans-serif;
}
.trade-side-btn {
  flex: 1;
  padding: 10px 0;
  border-radius: 0;
  border: 1px solid #3d4454;
  background: #252830;
  color: #d1d5db;
  font-weight: 700;
  cursor: pointer;
}
.trade-side-btn.active {
  background: #0ecb81;
  border-color: #0ecb81;
  color: #fff;
}
.trade-side-sell.active {
  background: #f6465d;
  border-color: #f6465d;
}
.trade-form-row {
  display: grid;
  gap: 6px;
}
.trade-label {
  font-size: 12px;
  color: #9ca3af;
}
.trade-amount-mode-row .trade-amount-label {
  margin: 0;
}
.trade-mode-select {
  width: 100%;
  padding: 8px 10px;
  border-radius: 0;
  border: 1px solid #3d4454;
  background: #252830;
  color: #f0b90b;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}
.trade-input-dark {
  width: 100%;
  padding: 10px 10px;
  border-radius: 0;
  border: 1px solid #3d4454;
  background: #12141a;
  color: #f9fafb;
}
.trade-input-readonly {
  opacity: 0.85;
  cursor: default;
  color: #94a3b8;
}
.trade-input-dark::placeholder {
  color: #6b7280;
}
.trade-input-stepper {
  display: flex;
  gap: 6px;
  align-items: stretch;
}
.trade-input-grow {
  flex: 1;
  min-width: 0;
}
.trade-step-btns {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 32px;
}
.trade-step-btn {
  flex: 1;
  min-height: 0;
  padding: 0;
  border-radius: 0;
  border: 1px solid #3d4454;
  background: #252830;
  color: #cbd5e1;
  font-size: 10px;
  line-height: 1;
  cursor: pointer;
}
.trade-input-suffix-wrap {
  display: flex;
  align-items: center;
  border-radius: 0;
  border: 1px solid #3d4454;
  background: #12141a;
  overflow: hidden;
}
.trade-input-suffix-input {
  flex: 1;
  min-width: 0;
  border: none !important;
  border-radius: 0 !important;
  background: transparent !important;
}
.trade-input-suffix {
  flex-shrink: 0;
  padding: 0 12px 0 8px;
  font-size: 13px;
  color: #94a3b8;
  font-weight: 600;
}
.trade-slider-block {
  display: grid;
  gap: 4px;
}
/* 滑块直径 18px：左右各留 9px，使圆心与轨道端点对齐，拇指能贴齐视觉上的 0%/100% */
.trade-range-wrap {
  padding: 4px 9px 0;
  box-sizing: border-box;
}
.trade-range {
  --range-pct: 0%;
  width: 100%;
  height: 22px;
  margin: 0;
  -webkit-appearance: none;
  appearance: none;
  background: transparent;
  cursor: pointer;
}
.trade-range:focus {
  outline: none;
}
.trade-range:focus-visible {
  outline: 2px solid rgba(234, 179, 8, 0.45);
  outline-offset: 2px;
  border-radius: 0;
}
.trade-range::-webkit-slider-runnable-track {
  height: 6px;
  border-radius: 0;
  border: 1px solid #4b5563;
  background: linear-gradient(
    to right,
    #eab308 0%,
    #eab308 var(--range-pct),
    #252830 var(--range-pct),
    #252830 100%
  );
}
.trade-range::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 18px;
  height: 18px;
  margin-top: -6px;
  border-radius: 50%;
  background: #eab308;
  border: 2px solid #1b1e26;
  box-sizing: border-box;
  box-shadow: 0 0 0 1px rgba(234, 179, 8, 0.35);
}
.trade-range::-moz-range-track {
  height: 6px;
  border-radius: 0;
  border: 1px solid #4b5563;
  background: #252830;
}
.trade-range::-moz-range-progress {
  height: 6px;
  border-radius: 0;
  background: #eab308;
}
.trade-range::-moz-range-thumb {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #eab308;
  border: 2px solid #1b1e26;
  box-sizing: border-box;
  box-shadow: 0 0 0 1px rgba(234, 179, 8, 0.35);
}
.trade-slider-ticks {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #6b7280;
  padding: 0 9px;
}
.trade-submit {
  width: 100%;
  padding: 12px 0;
  border-radius: 0;
  background: #0ecb81;
  border: none;
  color: #fff;
  font-weight: 800;
  cursor: pointer;
}
.trade-submit-sell {
  background: #f6465d;
}
.trade-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.trade-msg {
  padding: 10px 10px;
  border-radius: 0;
  font-size: 13px;
}
.trade-ok {
  background: rgba(22, 163, 74, 0.2);
  color: #86efac;
}
.trade-err {
  background: rgba(220, 38, 38, 0.2);
  color: #fca5a5;
}

@media (max-width: 980px) {
  .trade-spot-grid {
    grid-template-columns: 1fr;
  }
  .trade-spot-module-5 {
    grid-row: auto;
  }
}
</style>
