package api

import (
	"application/model"
	"application/service"
	"application/utils"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AssetHandler struct {
	assetService *service.AssetService
}

func NewAssetHandler() *AssetHandler {
	return &AssetHandler{
		assetService: service.NewAssetService(model.GetDB()),
	}
}

func (h *AssetHandler) CreateAsset(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.ServerError(c, "用户信息获取失败")
		return
	}
	org, exists := c.Get("org")
	if !exists {
		utils.ServerError(c, "组织信息获取失败")
		return
	}
	if org.(int) != 2 {
		utils.ServerError(c, "只有属于NFT创建者组织的用户可以上传NFT")
		return
	}
	name := c.PostForm("name")
	if name == "" {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	description := c.PostForm("description")
	if description == "" {
		description = "暂无描述"
	}
	quality := c.PostForm("quality")
	wear := c.PostForm("wear")
	category := c.PostForm("category")   // 新增
	wearValue := c.PostForm("wearValue") // 新增（字符串）

	allowedQuality := map[string]bool{
		"白色": true, "浅蓝色": true, "深蓝色": true, "紫色": true, "粉紫色": true, "红色": true, "金色": true,
	}
	allowedWear := map[string]bool{
		"崭新出厂": true, "略有磨损": true, "久经沙场": true, "破损不堪": true, "战痕累累": true,
	}
	allowedCategory := map[string]bool{
		"匕首": true, "手套": true, "步枪": true, "手枪": true, "冲锋枪": true, "霰弹枪": true, "机枪": true, "印花": true, "探员": true, "其他": true,
	}
	if !allowedQuality[quality] {
		utils.BadRequest(c, "品质颜色非法")
		return
	}
	if !allowedWear[wear] {
		utils.BadRequest(c, "磨损度等级非法")
		return
	}
	if !allowedCategory[category] {
		utils.BadRequest(c, "枪械种类非法")
		return
	}
	if wearValue == "" {
		utils.BadRequest(c, "磨损度值不能为空")
		return
	}
	if v, err := strconv.ParseFloat(wearValue, 64); err != nil || v < 0 || v > 1 {
		utils.BadRequest(c, "磨损度值必须是 0~1 小数")
		return
	}

	// 允许没有图片：没有则用默认图
	imageName := model.DefaultImageName
	image, err := c.FormFile("image")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		utils.BadRequest(c, "图片上传失败")
		return
	}
	if err == nil {
		ext := filepath.Ext(image.Filename)
		imageName = uuid.New().String() + ext
		savePath := filepath.Join(model.DefaultImageFolder, imageName)
		_ = os.MkdirAll(model.DefaultImageFolder, 0o755)
		if err := c.SaveUploadedFile(image, savePath); err != nil {
			utils.ServerError(c, fmt.Sprintf("保存图片失败：%s", err))
			return
		}
	}

	asset, err := h.assetService.CreateAsset(
		name, imageName, userID.(int), userID.(int),
		description, quality, wear, category, wearValue, org.(int),
	)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, asset)
}

type deleteReq struct {
	ID string `json:"id" binding:"required"`
}

// 兜底：从上下文获取 org，缺失或类型错误时默认 Org1=1
func orgFromCtx(c *gin.Context) int {
	if v, ok := c.Get("org"); ok {
		if i, ok2 := v.(int); ok2 && i > 0 {
			return i
		}
	}
	return 1
}

func (h *AssetHandler) GetAssetByID(c *gin.Context) {
	// 原先这里直接取 org 并在缺失时 500
	// org, exists := c.Get("org")
	// if !exists { utils.ServerError(c, "组织信息获取失败"); return }
	id := c.Query("id")
	org := orgFromCtx(c)
	asset, err := h.assetService.GetAssetByID(id, org)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, asset)
}

func (h *AssetHandler) GetAssetByAuthorID(c *gin.Context) {
	authorId, err := strconv.Atoi(c.Query("authorId"))
	if err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	org := orgFromCtx(c)
	assets, err := h.assetService.GetAssetByAuthorID(authorId, org)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, assets)
}

func (h *AssetHandler) GetAssetByOwnerID(c *gin.Context) {
	ownerId, err := strconv.Atoi(c.Query("ownerId"))
	if err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	org := orgFromCtx(c)
	assets, err := h.assetService.GetAssetByOwnerID(ownerId, org)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, assets)
}

func (h *AssetHandler) TransferAsset(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		utils.ServerError(c, "用户信息获取失败")
		return
	}
	org, exists := c.Get("org")
	if !exists {
		utils.ServerError(c, "组织信息获取失败")
		return
	}
	var req model.TransferAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	err := h.assetService.TransferAsset(req.ID, req.NewOwnerId, userID.(int), org.(int))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

func (h *AssetHandler) GetAssetStatus(c *gin.Context) {
	id := c.Query("id")
	status, err := h.assetService.GetAssetStatus(id)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, status)
}

func (h *AssetHandler) DeleteAsset(c *gin.Context) {
	userID, ok := c.Get("userID")
	if !ok {
		utils.ServerError(c, "用户信息获取失败")
		return
	}
	org, ok := c.Get("org")
	if !ok {
		utils.ServerError(c, "组织信息获取失败")
		return
	}
	var req deleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.assetService.DeleteAsset(req.ID, userID.(int), org.(int)); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}
