package entity

import "time"

// Network 对应表 networks。
type Network struct {
	ID         int       `db:"id"`
	Symbol     string    `db:"symbol"`
	Name       string    `db:"name"`
	RpcURL     *string   `db:"rpc_url"`
	ChainID    *int64    `db:"chain_id"`
	CryptoType string    `db:"crypto_type"`
	CreatedAt  time.Time `db:"created_at"`
}
