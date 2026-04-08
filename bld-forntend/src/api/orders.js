import axios from 'axios'

export const ordersHttp = axios.create({
  baseURL: '/ordersapi',
  timeout: 15000,
})

export function listSpotMarketsApi({ status } = {}) {
  const params = {}
  if (status) params.status = status
  return ordersHttp.get('/v1/spot/markets', { params })
}

export function createSpotOrderApi(payload) {
  return ordersHttp.post('/v1/spot/orders', payload)
}

/** @param {{ user_id: number, order_id: string }} payload */
export function cancelSpotOrderApi(payload) {
  return ordersHttp.post('/v1/spot/orders/cancel', payload)
}

/** @param {{ user_id: number, scope: 'open'|'history', market_id?: number, limit?: number }} params */
export function listSpotOrdersApi(params) {
  const p = {
    user_id: params.user_id,
    scope: params.scope,
    limit: params.limit ?? 100,
  }
  if (params.market_id != null && params.market_id > 0) {
    p.market_id = params.market_id
  }
  return ordersHttp.get('/v1/spot/orders', { params: p })
}

/** @param {{ user_id: number, market_id?: number, limit?: number }} params */
export function listSpotTradesApi(params) {
  const p = {
    user_id: params.user_id,
    limit: params.limit ?? 100,
  }
  if (params.market_id != null && params.market_id > 0) {
    p.market_id = params.market_id
  }
  return ordersHttp.get('/v1/spot/trades', { params: p })
}

