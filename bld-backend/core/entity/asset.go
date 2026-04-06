package entity

import "database/sql"

// Asset 对应表 assets。
type Asset struct {
	ID              int            `db:"id"`
	Symbol          string         `db:"symbol"`
	Name            string         `db:"name"`
	Decimals        int            `db:"decimals"`
	IsActive        int            `db:"is_active"`
	IsAggregate     int            `db:"is_aggregate"`
	NetworkID       int            `db:"network_id"`
	ContractAddress sql.NullString `db:"contract_address"`
}
