package handler

import (
	"net/http"

	"bloom-nft/api/request"
	"bloom-nft/model"
	"bloom-nft/services"

	"github.com/gin-gonic/gin"
	// log "github.com/sirupsen/logrus"
)

type LoginHandler struct {
	LoginService *services.LoginService
}

func NewLoginHandler(loginService *services.LoginService) *LoginHandler {
	return &LoginHandler{
		LoginService: loginService,
	}
}

// Login godoc
// @Summary      登录
// @Description  返回 token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      request.LoginRequest  true  "登录参数"
// @Success      200      {object}  map[string]string
// @Router       /auth/login [post]
func (h *LoginHandler) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.LoginService.Login(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.OkWithData(token))
}
