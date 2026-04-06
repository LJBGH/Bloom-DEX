package logic

import (
	"context"

	"bld-backend/apps/walletapi/internal/model"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/walletgen"
)

// insertNewCustodyWallet 按网络的 crypto_type 生成密钥、加密后写入 wallets
func insertNewCustodyWallet(ctx context.Context, svcCtx *svc.ServiceContext, userId uint64, network *model.Network) (walletID uint64, address string, err error) {
	gen, err := walletgen.GenerateByCryptoType(network.CryptoType)
	if err != nil {
		return 0, "", err
	}
	enc, err := svcCtx.MasterKey.EncryptToBase64([]byte(gen.PrivKeyPlaintext))
	if err != nil {
		return 0, "", err
	}
	wid, err := svcCtx.WalletModel.InsertWithNetwork(ctx, userId, network.ID, gen.Address, enc)
	if err != nil {
		return 0, "", err
	}
	return wid, gen.Address, nil
}
