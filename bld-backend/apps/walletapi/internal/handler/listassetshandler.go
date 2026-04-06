package handler

import (
	"net/http"
	"strconv"

	"bld-backend/apps/walletapi/internal/logic"
	"bld-backend/apps/walletapi/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ListAssetsHandler 返回某个用户的资产列表（币种 + 余额）。
func ListAssetsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.URL.Query().Get("user_id")
		if userIDStr == "" {
			http.Error(w, "user_id required", http.StatusBadRequest)
			return
		}
		uid, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil || uid == 0 {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}

		assetIDStr := r.URL.Query().Get("asset_id") // optional
		assetID := 0
		if assetIDStr != "" {
			v, err := strconv.Atoi(assetIDStr)
			if err != nil || v <= 0 {
				http.Error(w, "invalid asset_id", http.StatusBadRequest)
				return
			}
			assetID = v
		}

		l := logic.NewListAssetsLogic(r.Context(), svcCtx)
		resp, err := l.ListAssets(uid, assetID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

