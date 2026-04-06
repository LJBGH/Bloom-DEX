package entity

import "time"

// NetworkOffset 对应表 network_offsets。
type NetworkOffset struct {
	ID        int       `db:"id"`
	NetworkID int       `db:"network_id"`
	LastBlock int64     `db:"last_block"`
	UpdatedAt time.Time `db:"updated_at"`
}
