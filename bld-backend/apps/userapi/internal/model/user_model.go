package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrDuplicateUsername = errors.New("duplicate username")

type User struct {
	ID           uint64 `db:"id"`
	Username     string `db:"username"`
	PasswordHash string `db:"password_hash"`
}

type UserModel interface {
	Insert(ctx context.Context, username, passwordHash string) (uint64, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
}

type defaultUserModel struct {
	conn sqlx.SqlConn
}

func NewUserModel(conn sqlx.SqlConn) UserModel {
	return &defaultUserModel{conn: conn}
}

func (m *defaultUserModel) Insert(ctx context.Context, username, passwordHash string) (uint64, error) {
	res, err := m.conn.ExecCtx(ctx, "INSERT INTO users(username,password_hash) VALUES(?,?)", username, passwordHash)
	if err != nil {
		// MySQL duplicate key error text usually contains "Duplicate entry"
		if strings.Contains(err.Error(), "Duplicate entry") {
			return 0, ErrDuplicateUsername
		}
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (m *defaultUserModel) FindByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := m.conn.QueryRowCtx(ctx, &u, "SELECT id,username,password_hash FROM users WHERE username=? LIMIT 1", username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

