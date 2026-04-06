package handler

import (
	"net/http"

	"bld-backend/apps/walletapi/internal/logic"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取充值地址
// @Summary      获取充值地址
// @Description  获取充值地址
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Param        user_id  query     uint64  true  "用户ID"
// @Param        symbol  query     string  true  "币种"
// @Success      200  {object}  types.GetDepositAddressResp
// @Router       /v1/deposit/address [post]
func DepositAddressHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetDepositAddressReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewGetDepositAddressLogic(r.Context(), svcCtx)
		resp, err := l.GetDepositAddress(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
