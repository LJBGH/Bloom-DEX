package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrDuplicateWallet = errors.New("duplicate wallet")

type CustodyWallet struct {
	ID        uint64 `db:"id"`
	UserID    uint64 `db:"user_id"`
	NetworkID int `db:"network_id"`
	Address   string `db:"address"`
	PrivKeyEnc string `db:"privkey_enc"`
}

type CustodyWalletModel interface {
	FindByUser(ctx context.Context, userId uint64) (*CustodyWallet, error)
	FindByUserNetwork(ctx context.Context, userId uint64, networkId int) (*CustodyWallet, error)
	// FindByUserAndCryptoType 用户在该加密类型下任意网络上的第一条钱包（用于同类型多链复用地址与密文）
	FindByUserAndCryptoType(ctx context.Context, userId uint64, cryptoType string) (*CustodyWallet, error)
	InsertWithNetwork(ctx context.Context, userId uint64, networkId int, address, privKeyEnc string) (uint64, error)
}

type defaultCustodyWalletModel struct {
	conn sqlx.SqlConn
}

func NewCustodyWalletModel(conn sqlx.SqlConn) CustodyWalletModel {
	return &defaultCustodyWalletModel{conn: conn}
}

func (m *defaultCustodyWalletModel) FindByUser(ctx context.Context, userId uint64) (*CustodyWallet, error) {
	var w CustodyWallet
	err := m.conn.QueryRowCtx(ctx, &w, "SELECT id,user_id,network_id,address,privkey_enc FROM wallets WHERE user_id=? ORDER BY id ASC LIMIT 1", userId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (m *defaultCustodyWalletModel) FindByUserNetwork(ctx context.Context, userId uint64, networkId int) (*CustodyWallet, error) {
	var w CustodyWallet
	err := m.conn.QueryRowCtx(ctx, &w, "SELECT id,user_id,network_id,address,privkey_enc FROM wallets WHERE user_id=? AND network_id=? ORDER BY id ASC LIMIT 1", userId, networkId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (m *defaultCustodyWalletModel) FindByUserAndCryptoType(ctx context.Context, userId uint64, cryptoType string) (*CustodyWallet, error) {
	var w CustodyWallet
	err := m.conn.QueryRowCtx(ctx, &w, `
		SELECT w.id, w.user_id, w.network_id, w.address, w.privkey_enc
		FROM wallets w
		INNER JOIN networks n ON n.id = w.network_id
		WHERE w.user_id = ? AND n.crypto_type = ?
		ORDER BY w.id ASC LIMIT 1`, userId, cryptoType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (m *defaultCustodyWalletModel) InsertWithNetwork(ctx context.Context, userId uint64, networkId int, address, privKeyEnc string) (uint64, error) {
	res, err := m.conn.ExecCtx(ctx,
		"INSERT INTO wallets(user_id,network_id,address,privkey_enc) VALUES(?,?,?,?)",
		userId, networkId, address, privKeyEnc,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return 0, ErrDuplicateWallet
		}
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

