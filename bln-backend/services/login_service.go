package services

import (
	"bloom-nft/api/request"
	"bloom-nft/middleware"
)

type LoginService struct {
}

func NewLoginService() *LoginService {
	return &LoginService{}
}

// 登录
func (s *LoginService) Login(req request.LoginRequest) (string, error) {

	// 这里只是测试 JWT，直接用一个固定的 userID
	token, err := middleware.GenerateToken(1)
	if err != nil {
		return "", err
	}

	return token, nil
}
