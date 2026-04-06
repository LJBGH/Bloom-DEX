package handler

import (
	"net/http"
	"strconv"

	"bld-backend/apps/walletapi/internal/logic"
	"bld-backend/apps/walletapi/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListTokensHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		networkIDStr := r.URL.Query().Get("network_id")
		networkID, _ := strconv.Atoi(networkIDStr)
		if networkID <= 0 {
			http.Error(w, "network_id required", http.StatusBadRequest)
			return
		}

		l := logic.NewListTokensLogic(r.Context(), svcCtx)
		resp, err := l.ListTokens(networkID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

