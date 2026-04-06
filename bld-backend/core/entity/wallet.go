package entity

import "time"

// Wallet 对应表 wallets。
type Wallet struct {
	ID         uint64    `db:"id"`
	UserID     uint64    `db:"user_id"`
	NetworkID  int       `db:"network_id"`
	Address    string    `db:"address"`
	PrivkeyEnc string    `db:"privkey_enc"`
	CreatedAt  time.Time `db:"created_at"`
}
