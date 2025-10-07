package api

import (
	"application/service"
	"application/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

type SteamHandler struct {
	svc *service.SteamService
}

func NewSteamHandler() *SteamHandler {
	return &SteamHandler{svc: service.NewSteamService(nil)}
}

// 预览库存（无需登录）
func (h *SteamHandler) Inventory(c *gin.Context) {
	steamID := c.Query("steamId")
	if steamID == "" {
		utils.BadRequest(c, "steamId 不能为空")
		return
	}
	items, err := h.svc.FetchInventory(steamID, 1000)
	if err != nil {
		utils.ServerError(c, "获取失败："+err.Error())
		return
	}
	utils.Success(c, gin.H{"items": items})
}

type importReq struct {
	SteamID        string   `json:"steamId" binding:"required"`
	SelectAssetIDs []string `json:"assetIds"` // 为空表示全部导入
}

// 批量导入（需登录，org=2）
func (h *SteamHandler) Import(c *gin.Context) {
	uidVal, ok := c.Get("userID")
	if !ok {
		utils.ServerError(c, "无法获取用户")
		return
	}
	orgVal, ok := c.Get("org")
	if !ok {
		utils.ServerError(c, "无法获取组织")
		return
	}
	var req importReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	// 兼容传入整段 URL
	if strings.Contains(req.SteamID, "steamcommunity.com") {
		// 透传给 service 处理
	}
	assets, err := h.svc.ImportForUser(uidVal.(int), orgVal.(int), req.SteamID, req.SelectAssetIDs)
	if err != nil {
		utils.ServerError(c, "导入失败："+err.Error())
		return
	}
	utils.Success(c, gin.H{"count": len(assets), "items": assets})
}
