package handler

import (
	"bloom-nft/model"
	"bloom-nft/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TestHandler struct {
	TestService *services.TestService
}

func NewTestHandler(testService *services.TestService) *TestHandler {
	return &TestHandler{
		TestService: testService,
	}
}

// PingExample godoc
// @Summary      ping 测试
// @Description  返回 pong
// @Tags         ping
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  map[string]string
// @Router       /test/ping [get]
func (h *TestHandler) Ping(c *gin.Context) {
	pingResult := h.TestService.Ping()
	c.JSON(http.StatusOK, model.OkWithData(pingResult))
}

// GetTime godoc
// @Summary      获取当前时间
// @Description  返回当前服务器时间（RFC3339 格式）
// @Tags         api
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  map[string]string
// @Router       /test/time [get]
func (h *TestHandler) GetTime(c *gin.Context) {
	timeResult := h.TestService.GetTime()
	c.JSON(http.StatusOK, model.OkWithData(timeResult))
}
