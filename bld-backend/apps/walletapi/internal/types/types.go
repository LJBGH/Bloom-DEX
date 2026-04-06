package types

// CreateCustodyWalletReq is the request body for POST /v1/wallet/create.
// network_id 必填，取值见 GET /v1/networks。链类型由 networks.crypto_type 决定，无需传 chain。
type CreateCustodyWalletReq struct {
	UserId    uint64 `json:"user_id"`
	NetworkId int    `json:"network_id"`
}

type CreateCustodyWalletResp struct {
	WalletId   uint64 `json:"wallet_id"`
	UserId     uint64 `json:"user_id"`
	NetworkId  int    `json:"network_id"`
	NetworkSym string `json:"network_symbol"`
	Chain      string `json:"chain"` // networks.crypto_type
	Address    string `json:"address"`
}

// GetPrivateKeyReq POST /v1/wallet/private-key
type GetPrivateKeyReq struct {
	UserId    uint64 `json:"user_id"`
	NetworkId int    `json:"network_id"`
}

// GetPrivateKeyResp 私钥格式与 walletgen 存库约定一致：EVM=hex，BTC=WIF，SOL=ed25519 私钥 hex
type GetPrivateKeyResp struct {
	NetworkId   int    `json:"network_id"`
	NetworkSym  string `json:"network_symbol"`
	CryptoType  string `json:"crypto_type"`
	Address     string `json:"address"`
	PrivateKey  string `json:"private_key"`
}

type GetDepositAddressReq struct {
	UserId uint64 `json:"user_id"` // 用户ID
	Symbol string `json:"symbol"`  // 币种
	NetworkId int `json:"network_id"` // 网络ID
}

type GetDepositAddressResp struct {
	WalletId        uint64 `json:"wallet_id"`
	Chain           string `json:"chain"`
	Address         string `json:"address"`
	Decimals        uint32 `json:"decimals"`
	ContractAddress string `json:"contract_address"`
}

type NetworkItem struct {
	Id         int    `json:"id"`
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	RpcUrl     string `json:"rpc_url"`
	ChainId    int64  `json:"chain_id"`
	CryptoType string `json:"crypto_type"`
}

type NetworkListResp struct {
	Items []NetworkItem `json:"items"`
}

type TokenItem struct {
	AssetId int `json:"asset_id"`
	Symbol string `json:"symbol"`
	Decimals uint32 `json:"decimals"`
	ContractAddress string `json:"contract_address"`
	NetworkId int `json:"network_id"`
}

type TokenListResp struct {
	NetworkId int `json:"network_id"`
	Items []TokenItem `json:"items"`
}

type WithdrawReq struct {
	UserId      uint64 `json:"user_id"`
	Symbol      string `json:"symbol"`
	DestAddress string `json:"dest_address"`
	Amount      string `json:"amount"`
	Chain       string `json:"chain"`
}

type WithdrawResp struct {
	WithdrawId uint64 `json:"withdraw_id"`
	TxHash     string `json:"tx_hash"`
	Status     string `json:"status"`
}

type SweepToHotReq struct {
	UserId uint64 `json:"user_id"`
	Symbol string `json:"symbol"`
	Amount string `json:"amount"`
	Chain  string `json:"chain"`
}

type SweepToHotResp struct {
	TxHash      string `json:"tx_hash"`
	SweptAmount string `json:"swept_amount"`
	Status      string `json:"status"`
}

// AssetItem 描述单个币种的资产信息。
type AssetItem struct {
	Symbol           string `json:"symbol"`
	AssetId          int    `json:"asset_id"`
	AvailableBalance string `json:"available_balance"`
	FrozenBalance    string `json:"frozen_balance"`
}

// AssetListResp 是 GET/POST /v1/assets 的返回。
type AssetListResp struct {
	UserId uint64       `json:"user_id"`
	Items  []AssetItem  `json:"items"`
}
