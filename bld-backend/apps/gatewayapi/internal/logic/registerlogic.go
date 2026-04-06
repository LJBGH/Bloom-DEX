// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bld-backend/apps/gatewayapi/internal/svc"
	"bld-backend/apps/gatewayapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterReq) (resp *types.RegisterResp, err error) {
	username := strings.TrimSpace(req.Username)
	password := req.Password
	if username == "" || password == "" {
		return nil, errors.New("username/password required")
	}

	base := strings.TrimRight(l.svcCtx.Config.UserRestUrl, "/")
	url := fmt.Sprintf("%s/v1/register", base)

	var out types.RegisterResp
	err = postJSON(l.ctx, url, &types.RegisterReq{
		Username: username,
		Password: password,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
