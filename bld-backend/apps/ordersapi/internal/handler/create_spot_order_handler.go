package handler

import (
	"net/http"

	"bld-backend/apps/ordersapi/internal/logic"
	"bld-backend/apps/ordersapi/internal/svc"
	"bld-backend/apps/ordersapi/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateSpotOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateSpotOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewCreateSpotOrderLogic(r.Context(), svcCtx)
		resp, err := l.CreateSpotOrder(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

