package handler

import (
	"bld-backend/apps/userapi/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  "GET",
				Path:    "/healthz",
				Handler: HealthzHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/v1/register",
				Handler: RegisterHandler(svcCtx),
			},
			{
				Method:  "POST",
				Path:    "/v1/login",
				Handler: LoginHandler(svcCtx),
			},
		},
	)
}

