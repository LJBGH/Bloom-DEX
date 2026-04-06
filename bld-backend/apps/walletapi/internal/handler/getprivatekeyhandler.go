package handler

import (
	"net/http"

	"bld-backend/apps/walletapi/internal/logic"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetPrivateKeyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetPrivateKeyReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewGetPrivateKeyLogic(r.Context(), svcCtx)
		resp, err := l.GetPrivateKey(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
