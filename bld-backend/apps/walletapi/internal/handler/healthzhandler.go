package handler

import (
	"net/http"

	"bld-backend/apps/walletapi/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

type healthzResp struct {
	Status string `json:"status"`
}

func HealthzHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.OkJsonCtx(r.Context(), w, &healthzResp{Status: "ok"})
	}
}

