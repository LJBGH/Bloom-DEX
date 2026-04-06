package logic

import (
	"context"
	"errors"
	"strings"

	"bld-backend/apps/userapi/internal/svc"
	"bld-backend/apps/userapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *types.LoginReq) (*types.LoginResp, error) {
	username := strings.TrimSpace(in.Username)
	password := in.Password
	if username == "" || password == "" {
		return nil, errors.New("username/password required")
	}

	u, err := l.svcCtx.UserModel.FindByUsername(l.ctx, username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		// 不区分用户名不存在还是密码错误
		if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			// 其他错误直接返回
			return nil, err
		}
		return nil, errors.New("invalid username or password")
	}

	return &types.LoginResp{UserId: u.ID}, nil
}

