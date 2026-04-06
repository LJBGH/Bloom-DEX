package handler

import (
	"net/http"

	"bld-backend/apps/ordersapi/internal/logic"
	"bld-backend/apps/ordersapi/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListSpotMarketsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status") // optional, default ACTIVE

		l := logic.NewListSpotMarketsLogic(r.Context(), svcCtx)
		resp, err := l.List(status)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

