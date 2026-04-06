// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"bld-backend/apps/gatewayapi/internal/logic"
	"bld-backend/apps/gatewayapi/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 健康检查处理器
func HealthzHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewHealthzLogic(r.Context(), svcCtx)
		resp, err := l.Healthz()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
