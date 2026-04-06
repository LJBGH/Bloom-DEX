package logic

import (
	"context"
	"errors"
	"strings"

	"bld-backend/apps/userapi/internal/svc"
	"bld-backend/apps/userapi/internal/types"
	"bld-backend/apps/userapi/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegisterLogic) Register(in *types.RegisterReq) (*types.RegisterResp, error) {
	username := strings.TrimSpace(in.Username)
	password := in.Password
	if username == "" || password == "" {
		return nil, errors.New("username/password required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	uid, err := l.svcCtx.UserModel.Insert(l.ctx, username, string(hash))
	if err != nil {
		if errors.Is(err, model.ErrDuplicateUsername) {
			return nil, errors.New("username already exists")
		}
		return nil, err
	}

	return &types.RegisterResp{UserId: uid}, nil
}
