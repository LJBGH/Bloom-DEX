import { walletHttp } from './http'

export function listAssetsApi(userId, assetId) {
  const params = { user_id: userId }
  if (assetId) params.asset_id = assetId
  return walletHttp.get('/v1/assets', { params })
}

export function listNetworksApi() {
  return walletHttp.get('/v1/networks')
}

export function listTokensApi(networkId) {
  return walletHttp.get('/v1/tokens', { params: { network_id: networkId } })
}

export function createWalletApi(payload) {
  return walletHttp.post('/v1/wallet/create', payload)
}

/** 查看托管私钥（高危，生产环境应鉴权） */
export function getPrivateKeyApi(payload) {
  return walletHttp.post('/v1/wallet/private-key', payload)
}

export function getDepositAddressApi(payload) {
  return walletHttp.post('/v1/deposit/address', payload)
}

export function withdrawApi(payload) {
  return walletHttp.post('/v1/withdraw', payload)
}

export function sweepToHotApi(payload) {
  return walletHttp.post('/v1/wallet/sweep/hot', payload)
}

