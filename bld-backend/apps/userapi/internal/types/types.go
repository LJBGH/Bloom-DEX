package types

// RegisterReq is the request body for POST /v1/register.
type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterResp struct {
	UserId uint64 `json:"user_id"`
}

// LoginReq is the request body for POST /v1/login.
type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResp struct {
	UserId uint64 `json:"user_id"`
}

