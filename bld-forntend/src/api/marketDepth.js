import axios from 'axios'

/** 与 vite 代理 `/marketws` → market-ws (9201) 对齐 */
export const marketDepthHttp = axios.create({
  baseURL: '/api/marketws',
  timeout: 8000,
})

/**
 * HTTP 快照（有数据时返回内层 depth JSON）
 * @param {number} marketId
 */
export function fetchDepthSnapshot(marketId) {
  return marketDepthHttp.get('/api/v1/depth', { params: { market_id: marketId } })
}

/**
 * WebSocket URL。
 * - 开发环境默认直连 `ws://{当前页 hostname}:9201/ws`，避免 Vite 对 WS 代理升级失败导致收不到推送。
 * - 若需走代理：`VITE_MARKET_WS_USE_PROXY=true`
 * - 生产或其它：`VITE_MARKET_WS_URL=ws://...` 完整地址
 */
export function resolveMarketWsUrl() {
  const full = import.meta.env.VITE_MARKET_WS_URL
  if (full && String(full).trim()) {
    return String(full).trim()
  }

  if (import.meta.env.DEV && import.meta.env.VITE_MARKET_WS_USE_PROXY !== 'true') {
    const host = typeof window !== 'undefined' ? window.location.hostname : '127.0.0.1'
    return `ws://${host}:9201/ws`
  }

  const proto = typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = typeof window !== 'undefined' ? window.location.host : 'localhost:5173'
  // Keep WS direct to market-ws for now (gateway doesn't proxy WS reliably).
  return `${proto}//${host}/marketws/ws`
}

/**
 * @param {object} raw — MarketDepthKafkaMsg
 * @returns {{ bids: { price: number, qty: number }[], asks: { price: number, qty: number }[] }}
 */
export function normalizeDepthPayload(raw) {
  const bids = Array.isArray(raw?.bids) ? raw.bids : []
  const asks = Array.isArray(raw?.asks) ? raw.asks : []
  const toRow = (lv) => ({
    price: parseFloat(String(lv.price ?? '').replace(/,/g, '')) || 0,
    qty: parseFloat(String(lv.quantity ?? '').replace(/,/g, '')) || 0,
  })
  return {
    bids: bids.map(toRow).filter((r) => r.price > 0 && r.qty > 0),
    asks: asks.map(toRow).filter((r) => r.price > 0 && r.qty > 0),
  }
}
