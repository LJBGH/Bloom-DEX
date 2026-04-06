package handler

import (
	"net/http"

	"bld-backend/apps/walletapi/internal/logic"
	"bld-backend/apps/walletapi/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListNetworksHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewListNetworksLogic(r.Context(), svcCtx)
		resp, err := l.ListNetworks()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

