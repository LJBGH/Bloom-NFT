package handler

import (
	"bloom-nft/api/request"
	"bloom-nft/enums"
	middleware "bloom-nft/middleware/exception"
	"bloom-nft/model"
	"bloom-nft/services"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type NftHandler struct {
	NftService *services.NftService
}

func NewNftHandler(nftService *services.NftService) *NftHandler {
	return &NftHandler{
		NftService: nftService,
	}
}

// Mint godoc
// @Summary      铸造 NFT（Pinata 上传）
// @Description  上传表单文件到 Pinata，并返回生成所需的 CID 与 tokenURI URL
// @Tags         nft
// @Accept       multipart/form-data
// @Produce      json
// @Security     ApiKeyAuth
// @Param        name         header   string  true  "NFT 名称"
// @Param        description  header   string  true  "NFT 描述"
// @Param        file         formData file    true  "上传文件（用于生成 tokenURI 内容）"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Router       /nft/mint [post]
func (n *NftHandler) Mint(c *gin.Context) {

	var request request.MintRequest
	// multipart/form-data 下，header 很容易触发编码限制（非 ISO-8859-1）。
	// 这里改为直接从表单字段读取 name/description。
	request.Name = c.PostForm("name")
	request.Description = c.PostForm("description")
	if request.Name == "" || request.Description == "" {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	file, err := c.FormFile("file")
	if err != nil {
		fmt.Printf("no exist failed: %v\n", err)
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	src, err := file.Open()
	if err != nil {
		fmt.Printf("file open failed: %v\n", err)
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	defer src.Close()

	result, err := n.NftService.Mint(&request, src, true)
	if err != nil {
		fmt.Printf("upload failed: %v\n", err)
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}
	c.JSON(http.StatusOK, model.OkWithData(result))
}

// 更新NFT
// @Summary      更新NFT
// @Description  更新NFT NFT
// @Tags         nft
// @Accept       json
// @Produce      json
// @Param        tokenUrl  path  int  true  "NFT tokenUrl"
// @Param        tokenId  path  int  true  "NFT tokenId"
// @Param        owner  path  int  true  "NFT owner"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Router       /nft/listall/{nftId} [post]
func (n *NftHandler) UpdateNftList(c *gin.Context) {
	// 路由定义为：GET /api/nft/listall/:nftId
	tokenUrl := c.PostForm("tokenUrl")
	tokenId := c.PostForm("tokenId")
	owner := c.PostForm("owner")
	tokenIdParsed, err := strconv.ParseUint(tokenId, 10, 0)
	if err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	err = n.NftService.UpdateNftList(tokenUrl, uint(tokenIdParsed), owner)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData("success"))
}

// 获取所有系列NFT
// @Summary      获取所有系列NFT
// @Description  返回所有系列 NFT 列表
// @Tags         nft
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Router       /nft/all [get]
func (n *NftHandler) AllNft(c *gin.Context) {
	allNft, err := n.NftService.AllNft()
	if err != nil {
		fmt.Printf("file open failed: %v\n", err)
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(allNft))
}

// 根据NFT系列Id 获取所有NFT
// @Summary      根据NFT系列Id获取所有NFT
// @Description  返回指定系列下的所有 NFT
// @Tags         nft
// @Accept       json
// @Produce      json
// @Param        nftId  path  int  true  "NFT 系列Id"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Router       /nft/listall/{nftId} [get]
func (n *NftHandler) AllNftList(c *gin.Context) {
	// 路由定义为：GET /api/nft/listall/:nftId
	nftId := c.Param("nftId")
	nftIdParsed, err := strconv.ParseUint(nftId, 10, 0)
	if err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}
	allNftList, err := n.NftService.AllNftList(uint(nftIdParsed))
	if err != nil {
		fmt.Printf("file open failed: %v\n", err)
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(allNftList))
}

// 用户拥有的 NFT 类目列表
// @Summary      用户所拥有的NFT类目列表
// @Description  根据 owner 地址返回其拥有的 NFT 类目（Nft）列表
// @Tags         nft
// @Accept       json
// @Produce      json
// @Param        owner  query  string  true  "用户钱包地址"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Router       /nft/user/categories [get]
func (n *NftHandler) UserCategories(c *gin.Context) {
	owner := c.Query("owner")
	if owner == "" {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	categories, err := n.NftService.CategoriesByOwner(owner)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}
	c.JSON(http.StatusOK, model.OkWithData(categories))
}

// 用户在某个 NFT 类目下的所有 NFT 列表
// @Summary      用户该NFT类目下的所有NFT列表
// @Description  根据 owner 地址和 nftId 返回该用户在该类目下的所有 NFT
// @Tags         nft
// @Accept       json
// @Produce      json
// @Param        owner  query  string  true  "用户钱包地址"
// @Param        nftId  path   int     true  "NFT 系列Id"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Router       /nft/user/list/{nftId} [get]
func (n *NftHandler) UserNftList(c *gin.Context) {
	owner := c.Query("owner")
	if owner == "" {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	nftId := c.Param("nftId")
	nftIdParsed, err := strconv.ParseUint(nftId, 10, 0)
	if err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	list, err := n.NftService.NftListByOwnerAndNftId(owner, uint(nftIdParsed))
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(list))
}
