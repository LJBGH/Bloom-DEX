package svc

import (
	"bld-backend/apps/walletapi/internal/config"
	"bld-backend/apps/walletapi/internal/model"
	"bld-backend/apps/walletapi/internal/crypto"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config config.Config
	MasterKey *crypto.MasterKey
	WalletModel model.CustodyWalletModel
	AssetModel model.AssetModel
	NetworkModel model.NetworkModel
	WalletBalanceModel model.WalletBalanceModel
	AssetFreezeModel   model.AssetFreezeModel
	WithdrawOrderModel model.WithdrawOrderModel
	SpotTradeSettlementModel model.SpotTradeSettlementModel
	SpotFundFlowModel        model.SpotFundFlowModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	mk, err := crypto.NewMasterKeyFromBase64(c.CustodyMasterKey)
	if err != nil {
		logx.Must(err)
	}
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	return &ServiceContext{
		Config:      c,
		MasterKey:   mk,
		WalletModel: model.NewCustodyWalletModel(conn),
		AssetModel:  model.NewAssetModel(conn),
		NetworkModel: model.NewNetworkModel(conn),
		WalletBalanceModel: model.NewWalletBalanceModel(conn),
		AssetFreezeModel:   model.NewAssetFreezeModel(conn),
		WithdrawOrderModel: model.NewWithdrawOrderModel(conn),
		SpotTradeSettlementModel: model.NewSpotTradeSettlementModel(conn),
		SpotFundFlowModel:        model.NewSpotFundFlowModel(conn),
	}
}
