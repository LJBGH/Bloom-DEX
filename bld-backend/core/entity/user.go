package entity

import "time"

// User 对应表 users。
type User struct {
	ID           uint64    `db:"id"`
	Username     string    `db:"username"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}
