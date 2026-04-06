package handler

import (
	"net/http"

	"bld-backend/apps/walletapi/internal/logic"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 热钱包归集
// @Summary      热钱包归集
// @Description  热钱包归集
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Param        user_id  query     uint64  true  "用户ID"
// @Param        symbol  query     string  true  "币种"
// @Param        amount  query     string  true  "归集金额"
func SweepToHotHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SweepToHotReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewSweepToHotLogic(r.Context(), svcCtx)
		resp, err := l.SweepToHot(&req)
		if err != nil {
			// Keep detailed error in logs and return readable message for frontend.
			logx.WithContext(r.Context()).Errorf("SweepToHot failed, req=%+v, err=%v", req, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
