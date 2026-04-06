package handler

import (
	"bld-backend/apps/walletapi/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  "GET",
				Path:    "/healthz",
				Handler: HealthzHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/v1/assets", // 查询用户资产列表 ?user_id=xx
				Handler: ListAssetsHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/v1/networks", // 支持的网络列表
				Handler: ListNetworksHandler(svcCtx),
			},
			{
				Method:  "GET",
				Path:    "/v1/tokens", // 支持的代币列表 ?network_id=xx
				Handler: ListTokensHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/v1/wallet/create", // 创建钱包 body: user_id, network_id（见 GET /v1/networks）
				Handler: CreateWalletHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/v1/wallet/private-key", // 查看私钥 body: user_id, network_id
				Handler: GetPrivateKeyHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/v1/deposit/address", // 获取充值地址
				Handler: DepositAddressHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/v1/withdraw", // 提现
				Handler: WithdrawHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/v1/wallet/sweep/hot", // 热钱包归集
				Handler: SweepToHotHandler(svcCtx),
			},
		},
	)
}
