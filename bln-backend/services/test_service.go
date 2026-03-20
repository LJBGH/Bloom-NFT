package services

import (
	"bloom-nft/api/response"
	"time"
)

type TestService struct {
}

func NewTestService() *TestService {
	return &TestService{}
}

// 测试
func (s *TestService) Ping() *response.PingResult {
	return &response.PingResult{
		Message: "ping",
	}
}

// 获取当前时间
func (s *TestService) GetTime() string {
	timeResult := time.Now().Format(time.RFC3339)
	return timeResult
}
