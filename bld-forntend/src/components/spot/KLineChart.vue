<script setup>
import { CandlestickSeries, HistogramSeries, createChart } from 'lightweight-charts'
import axios from 'axios'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps({
  marketId: { type: Number, required: true },
  /** 如 ETH/USDT，展示在 OHLC 条 */
  pairLabel: { type: String, default: '—' },
})

const marketKlineHttp = axios.create({
  // 统一走网关：/api/marketws -> market-ws(9201)
  baseURL: '/api/marketws',
  timeout: 8000,
})

const TIMEFRAMES = [
  { label: '1m', step: 60 },
  { label: '5m', step: 300 },
  { label: '15m', step: 900 },
  { label: '30m', step: 1800 },
  { label: '1h', step: 3600 },
  { label: '4h', step: 14400 },
  { label: '1d', step: 86400 },
  { label: '1w', step: 604800 },
]

const RANGE_PRESETS = [
  { label: '1D', bars: 96 },
  { label: '5D', bars: 120 },
  { label: '1M', bars: 180 },
  { label: '3M', bars: 200 },
  { label: '6M', bars: 220 },
  { label: '1Y', bars: 260 },
  { label: '全部', bars: 320 },
]

const TOOL_ICONS = [
  { icon: '╋', title: '十字光标' },
  { icon: '╱', title: '趋势线' },
  { icon: '⌒', title: '斐波那契' },
  { icon: '▭', title: '矩形' },
  { icon: 'T', title: '文字' },
  { icon: '◎', title: '测量' },
]

const chartHostRef = ref(null)
const activeTf = ref('30m')
const activeRange = ref('1M')
const ohlcText = ref('')
const chartMode = ref('basic') // basic | tv | depth（后两者占位）

let chart = null
let candleSeries = null
let volumeSeries = null
let ro = null
let crosshairHandler = null
/** 十字光标移出时恢复展示 */
let lastCandleBar = null
let candles1mCache = []

/** 当前周期秒数 */
const stepSec = computed(() => {
  const f = TIMEFRAMES.find((x) => x.label === activeTf.value)
  return f ? f.step : 1800
})

/** 当前 K 线根数 */
const barCount = computed(() => {
  const r = RANGE_PRESETS.find((x) => x.label === activeRange.value)
  return r ? r.bars : 180
})

function mulberry32(seed) {
  let a = seed >>> 0
  return function () {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function fmtPx(n) {
  if (!Number.isFinite(n)) return '—'
  const x = Math.abs(n)
  if (x >= 1000) return n.toFixed(2)
  if (x >= 1) return n.toFixed(4)
  return n.toFixed(6)
}

function buildOhlcLine(c, tfLabel) {
  if (!c || c.open == null) return ''
  const o = Number(c.open)
  const h = Number(c.high)
  const l = Number(c.low)
  const cl = Number(c.close)
  const chg = o ? ((cl - o) / o) * 100 : 0
  const sign = chg >= 0 ? '+' : ''
  const pair = props.pairLabel || '—'
  return `${pair} 现货 · ${tfLabel}   O ${fmtPx(o)}   H ${fmtPx(h)}   L ${fmtPx(l)}   C ${fmtPx(cl)}   ${sign}${chg.toFixed(2)}%`
}

function resample1mTo(step, count) {
  // buckets: { timeSec, open, high, low, close, volume }
  const buckets = new Map()
  for (const c of candles1mCache) {
    const timeSec = Math.floor(Number(c.open_time_ms) / 1000)
    const bucketTime = Math.floor(timeSec / step) * step
    if (!Number.isFinite(bucketTime)) continue
    let b = buckets.get(bucketTime)
    if (!b) {
      b = { time: bucketTime, open: Number(c.open), high: Number(c.high), low: Number(c.low), close: Number(c.close), volume: 0 }
      buckets.set(bucketTime, b)
    }
    const open = Number(c.open)
    const high = Number(c.high)
    const low = Number(c.low)
    const close = Number(c.close)
    const vol = Number(c.volume)
    if (!Number.isFinite(open) || !Number.isFinite(high) || !Number.isFinite(low) || !Number.isFinite(close) || !Number.isFinite(vol)) continue
    b.high = Math.max(b.high, high)
    b.low = Math.min(b.low, low)
    b.close = close
    b.volume += vol
  }

  const ordered = Array.from(buckets.values()).sort((a, b) => a.time - b.time)
  const sliced = ordered.slice(Math.max(0, ordered.length - count))
  const candles = sliced.map((b) => ({
    time: b.time,
    open: b.open,
    high: b.high,
    low: b.low,
    close: b.close,
  }))
  const volumes = sliced.map((b) => {
    const up = b.close >= b.open
    return {
      time: b.time,
      value: b.volume,
      color: up ? 'rgba(14, 203, 129, 0.55)' : 'rgba(246, 70, 93, 0.55)',
    }
  })

  return { candles, volumes, last: candles[candles.length - 1] || null }
}

let loadSeq = 0
async function loadAndApply() {
  if (!candleSeries || !volumeSeries) return
  const seq = ++loadSeq

  const step = stepSec.value
  const count = barCount.value
  const toMs = Date.now()
  const fromMs = toMs - step * count * 1000 - 60 * 1000 * 5
  const limit1m = Math.min(4000, Math.ceil((step * count) / 60) + 50)

  const resp = await marketKlineHttp.get('/api/v1/klines', {
    params: {
      market_id: props.marketId,
      interval: '1m',
      from_ms: fromMs,
      to_ms: toMs,
      limit: limit1m,
    },
  })

  if (seq !== loadSeq) return
  const items = resp?.data?.items || []
  candles1mCache = items

  const { candles, volumes, last } = resample1mTo(step, count)
  candleSeries.setData(candles)
  volumeSeries.setData(volumes)
  lastCandleBar = last
  ohlcText.value = last ? buildOhlcLine(last, activeTf.value) : ''
  chart?.timeScale().fitContent()
}

function applyData() {
  loadAndApply().catch((e) => {
    // 接口异常时不再用模拟数据，保留空图便于定位问题
    // eslint-disable-next-line no-console
    console.error('load klines failed', e)
    candleSeries?.setData([])
    volumeSeries?.setData([])
    lastCandleBar = null
    ohlcText.value = ''
  })
}

function initChart() {
  const el = chartHostRef.value
  if (!el) return

  const w = el.clientWidth
  const h = el.clientHeight || 380

  chart = createChart(el, {
    width: w,
    height: h,
    layout: {
      attributionLogo: false,
      background: { color: '#0b0e11' },
      textColor: '#848e9c',
      fontSize: 11,
    },
    grid: {
      vertLines: { color: 'rgba(42, 46, 54, 0.55)' },
      horzLines: { color: 'rgba(42, 46, 54, 0.55)' },
    },
    crosshair: {
      vertLine: { color: 'rgba(240, 185, 11, 0.35)', width: 1 },
      horzLine: { color: 'rgba(240, 185, 11, 0.35)', width: 1 },
    },
    rightPriceScale: {
      borderColor: '#2a2e36',
      scaleMargins: { top: 0.08, bottom: 0.1 },
    },
    timeScale: {
      borderColor: '#2a2e36',
      timeVisible: true,
      secondsVisible: false,
    },
  })

  candleSeries = chart.addSeries(CandlestickSeries, {
    upColor: '#0ecb81',
    downColor: '#f6465d',
    borderVisible: false,
    wickUpColor: '#0ecb81',
    wickDownColor: '#f6465d',
    lastValueVisible: true,
    priceLineVisible: true,
  })

  const volPane = chart.addPane()
  chart.panes()[0].setStretchFactor(3)
  volPane.setStretchFactor(1)

  volumeSeries = volPane.addSeries(HistogramSeries, {
    color: '#0ecb81',
    priceFormat: { type: 'volume' },
    lastValueVisible: false,
    priceLineVisible: false,
    base: 0,
  })

  applyData()

  crosshairHandler = (param) => {
    if (!candleSeries) return
    if (!param?.time) {
      if (lastCandleBar) ohlcText.value = buildOhlcLine(lastCandleBar, activeTf.value)
      return
    }
    const d = param.seriesData?.get(candleSeries)
    if (d && typeof d === 'object' && 'close' in d) {
      ohlcText.value = buildOhlcLine(d, activeTf.value)
    }
  }
  chart.subscribeCrosshairMove(crosshairHandler)

  ro = new ResizeObserver(() => {
    if (!chartHostRef.value || !chart) return
    chart.applyOptions({
      width: chartHostRef.value.clientWidth,
      height: chartHostRef.value.clientHeight || 380,
    })
  })
  ro.observe(el)
}

function destroyChart() {
  if (chart && crosshairHandler) {
    chart.unsubscribeCrosshairMove(crosshairHandler)
  }
  crosshairHandler = null
  if (ro) {
    ro.disconnect()
    ro = null
  }
  if (chart) {
    chart.remove()
    chart = null
  }
  candleSeries = null
  volumeSeries = null
}

onMounted(() => {
  initChart()
})

watch(
  () => [props.marketId, props.pairLabel],
  () => {
    if (candleSeries && volumeSeries) applyData()
  },
)

watch([activeTf, activeRange], () => {
  if (candleSeries && volumeSeries) applyData()
})

onBeforeUnmount(() => {
  destroyChart()
})
</script>

<template>
  <div class="kline-module">
    <!-- 顶栏：周期 + 模式（参考专业端） -->
    <div class="kline-toolbar">
      <div class="kline-toolbar-left">
        <button
          v-for="tf in TIMEFRAMES"
          :key="tf.label"
          type="button"
          class="kline-tf-btn"
          :class="{ active: activeTf === tf.label }"
          @click="activeTf = tf.label"
        >
          {{ tf.label }}
        </button>
      </div>
      <div class="kline-toolbar-right">
        <span
          class="kline-mode"
          :class="{ active: chartMode === 'basic' }"
          @click="chartMode = 'basic'"
        >基本版</span>
        <span class="kline-mode-sep">|</span>
        <span class="kline-mode dim" title="占位">TradingView</span>
        <span class="kline-mode-sep">|</span>
        <span class="kline-mode dim" title="占位">深度图</span>
      </div>
    </div>

    <div class="kline-body">
      <aside class="kline-sidebar" aria-label="画线工具">
        <button
          v-for="(t, i) in TOOL_ICONS"
          :key="i"
          type="button"
          class="kline-tool-btn"
          :title="t.title"
        >
          {{ t.icon }}
        </button>
      </aside>

      <div class="kline-main">
        <div class="kline-chart-stack">
          <div class="kline-ohlc" :title="ohlcText">{{ ohlcText }}</div>
          <div ref="chartHostRef" class="kline-chart-host" />
        </div>
        <div class="kline-vol-caption">成交量</div>
      </div>
    </div>

    <div class="kline-footer">
      <div class="kline-range-btns">
        <button
          v-for="r in RANGE_PRESETS"
          :key="r.label"
          type="button"
          class="kline-range-btn"
          :class="{ active: activeRange === r.label }"
          @click="activeRange = r.label"
        >
          {{ r.label }}
        </button>
      </div>
      <div class="kline-footer-meta">
        <span>UTC</span>
        <span class="kline-foot-sep">|</span>
        <span>%</span>
        <span class="kline-foot-sep">|</span>
        <span>对数</span>
        <span class="kline-foot-sep">|</span>
        <span>自动</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.kline-module {
  display: flex;
  flex-direction: column;
  min-height: 480px;
  background: #0b0e11;
  border: 1px solid #2a2e36;
  border-radius: 0;
  font-family: 'Segoe UI', system-ui, sans-serif;
  color: #eaecef;
}

.kline-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid #2a2e36;
  background: #14161a;
}

.kline-toolbar-left {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.kline-tf-btn {
  margin: 0;
  padding: 4px 8px;
  border: none;
  border-radius: 0;
  background: transparent;
  color: #848e9c;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.kline-tf-btn:hover {
  color: #eaecef;
}

.kline-tf-btn.active {
  color: #f0b90b;
}

.kline-toolbar-right {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.kline-mode {
  cursor: pointer;
  color: #848e9c;
}

.kline-mode.active {
  color: #f0b90b;
  font-weight: 700;
}

.kline-mode.dim {
  cursor: default;
  opacity: 0.45;
}

.kline-mode-sep {
  color: #474d57;
  user-select: none;
}

.kline-body {
  display: flex;
  flex: 1;
  min-height: 0;
  border-bottom: 1px solid #2a2e36;
}

.kline-sidebar {
  width: 36px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 8px 4px;
  border-right: 1px solid #2a2e36;
  background: #14161a;
}

.kline-tool-btn {
  width: 28px;
  height: 28px;
  margin: 0;
  padding: 0;
  border: 1px solid #2a2e36;
  border-radius: 0;
  background: #1e2026;
  color: #848e9c;
  font-size: 12px;
  line-height: 1;
  cursor: default;
}

.kline-tool-btn:hover {
  color: #eaecef;
  border-color: #474d57;
}

.kline-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.kline-chart-stack {
  position: relative;
  flex: 1;
  min-height: 360px;
}

.kline-ohlc {
  position: absolute;
  z-index: 2;
  top: 8px;
  left: 10px;
  right: 72px;
  font-size: 11px;
  line-height: 1.45;
  color: #b7bdc6;
  pointer-events: none;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.kline-chart-host {
  position: absolute;
  inset: 0;
}

.kline-vol-caption {
  flex-shrink: 0;
  padding: 2px 10px 4px;
  font-size: 10px;
  color: #5e6673;
  background: #0b0e11;
  border-top: 1px solid #2a2e36;
}

.kline-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 6px 10px;
  background: #14161a;
  font-size: 11px;
  color: #848e9c;
}

.kline-range-btns {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.kline-range-btn {
  margin: 0;
  padding: 3px 8px;
  border: 1px solid transparent;
  border-radius: 0;
  background: transparent;
  color: #848e9c;
  cursor: pointer;
  font-size: 11px;
}

.kline-range-btn:hover {
  color: #eaecef;
}

.kline-range-btn.active {
  color: #f0b90b;
  border-color: #2a2e36;
  background: #1e2026;
}

.kline-footer-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #5e6673;
}

.kline-foot-sep {
  color: #474d57;
}
</style>
