package handler

import (
	"bloom-nft/api/request"
	"bloom-nft/api/response"
	"bloom-nft/model"
	"bloom-nft/services"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	WalletService *services.WalletService
}

func NewWalletHandler(walletService *services.WalletService) *WalletHandler {
	return &WalletHandler{
		WalletService: walletService,
	}
}

// Create godoc
// @Summary      创建助记词
// @Description  创建一个助记词
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  response.WalletResult
// @Router       /wallet/create/mnemonic [get]
func (h *WalletHandler) CreateMnemonic(c *gin.Context) {
	result := h.WalletService.CreateMnemonic()
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// Create godoc
// @Summary      创建钱包
// @Description  创建一个钱包,如果有助记词根据助记词创建,否者创建一个新钱包并返回助记词/地址/密钥等信息
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Param        mnemonic  query     string  false  "助记词（空格分隔）；不传则自动生成"
// @Success      200  {object}  response.WalletAccountResult
// @Router       /wallet/create/wallet [get]
func (h *WalletHandler) CreateWallet(c *gin.Context) {
	mnemonic := c.Query("mnemonic")
	result := h.WalletService.CreateWallet(mnemonic)
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// Import godoc
// @Summary      导入钱包
// @Description  上传钱包 JSON 文件导入钱包并返回钱包信息
// @Tags         wallet
// @Accept       multipart/form-data
// @Produce      json
// @Security     ApiKeyAuth
// @Param        file  formData  file  true  "钱包 JSON 文件"
// @Success      200  {object}  response.WalletResult
// @Failure      400  {object}  response.WalletResult
// @Router       /wallet/import [post]
func (h *WalletHandler) Import(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Fail[*response.WalletResult]("missing form file: file"))
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Fail[*response.WalletResult]("failed to open uploaded file"))
		return
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Fail[*response.WalletResult]("failed to read uploaded file"))
		return
	}

	var req request.WalletImportRequest
	if err := json.Unmarshal(b, &req); err != nil {
		c.JSON(http.StatusBadRequest, model.Fail[*response.WalletResult]("invalid wallet json"))
		return
	}

	result := h.WalletService.Import(&req)
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// Backup godoc
// @Summary      备份钱包
// @Description  备份当前钱包信息（具体内容取决于实现）
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  response.WalletAccountResult
// @Router       /wallet/backup [get]
func (h *WalletHandler) Backup(c *gin.Context) {
	result := h.WalletService.Backup()
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// Restore godoc
// @Summary      恢复钱包
// @Description  从默认备份源恢复钱包（具体内容取决于实现）
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  response.WalletAccountResult
// @Router       /wallet/restore [get]
func (h *WalletHandler) Restore(c *gin.Context) {
	result := h.WalletService.Restore()
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// RestoreWithPrivateKey godoc
// @Summary      根据私钥恢复钱包
// @Description  通过私钥恢复钱包并返回钱包信息
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Param        privateKey  query     string  true  "私钥（hex 字符串）"
// @Success      200  {object}  response.WalletAccountResult
// @Router       /wallet/restore/privateKey [get]
func (h *WalletHandler) RestoreWithPrivateKey(c *gin.Context) {
	privateKey := c.Query("privateKey")
	result := h.WalletService.RestoreWithPrivateKey(privateKey)
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// RestoreWithMnemonic godoc
// @Summary      根据助记词恢复钱包
// @Description  通过助记词恢复钱包并返回钱包信息
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Param        mnemonic  query     string  true  "助记词（空格分隔）"
// @Success      200  {object}  response.WalletAccountResult
// @Router       /wallet/restore/mnemonic [get]
func (h *WalletHandler) RestoreWithMnemonic(c *gin.Context) {
	mnemonic := c.Query("mnemonic")
	result := h.WalletService.RestoreWithMnemonic(mnemonic)
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// CurrentAccount godoc
// @Summary      查看当前账户
// @Description  查看当前选中的账户
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  response.WalletAccountResult
// @Router       /wallet/account/current [get]
func (h *WalletHandler) CurrentAccount(c *gin.Context) {
	result := h.WalletService.CurrentAccount()
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// AllAccounts godoc
// @Summary      查看所有账户
// @Description  查看本地保存的全部账户列表
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200  {object}  response.WalletAccountsResult
// @Router       /wallet/account/all [get]
func (h *WalletHandler) AllAccounts(c *gin.Context) {
	accounts := h.WalletService.AllAccounts()
	c.JSON(http.StatusOK, model.OkWithData(&response.WalletAccountsResult{Accounts: accounts}))
}

// SwitchCurrentAccount godoc
// @Summary      切换当前账户
// @Description  按 index 切换当前账户
// @Tags         wallet
// @Produce      json
// @Security     ApiKeyAuth
// @Param        index  query     int  true  "账户 index"
// @Success      200  {object}  response.WalletAccountResult
// @Failure      400  {object}  response.WalletAccountResult
// @Router       /wallet/account/switch [get]
func (h *WalletHandler) SwitchCurrentAccount(c *gin.Context) {
	raw := c.Query("index")
	if strings.TrimSpace(raw) == "" {
		c.JSON(http.StatusBadRequest, model.Fail[*response.WalletAccountResult]("missing query: index"))
		return
	}
	idx, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Fail[*response.WalletAccountResult]("invalid query: index"))
		return
	}
	result := h.WalletService.SwitchCurrentAccount(idx)
	c.JSON(http.StatusOK, model.OkWithData(result))
}
