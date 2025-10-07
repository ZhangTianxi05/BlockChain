package service

import (
	"application/model"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SteamService struct {
	http *http.Client
	as   *AssetService
}

func NewSteamService(db interface{}) *SteamService {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment, // 先用系统代理
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}
	// 可选：支持专用环境变量覆盖（如不方便改全局）
	if pu := os.Getenv("STEAM_HTTP_PROXY"); pu != "" {
		if u, err := url.Parse(pu); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &SteamService{
		http: &http.Client{
			Timeout:   25 * time.Second, // 加长整体超时
			Transport: tr,
		},
		as: NewAssetService(model.GetDB()),
	}
}

type steamInventoryResp struct {
	Assets          []steamAsset          `json:"assets"`
	Descriptions    []steamDesc           `json:"descriptions"`
	AssetProperties []steamAssetPropsWrap `json:"asset_properties"`
	Success         int                   `json:"success"`
}
type steamAsset struct {
	Appid      int    `json:"appid"`
	ContextID  string `json:"contextid"`
	AssetID    string `json:"assetid"`
	ClassID    string `json:"classid"`
	InstanceID string `json:"instanceid"`
	Amount     string `json:"amount"`
}
type steamDesc struct {
	Appid        int           `json:"appid"`
	ClassID      string        `json:"classid"`
	InstanceID   string        `json:"instanceid"`
	IconURL      string        `json:"icon_url"`
	Name         string        `json:"name"`
	Type         string        `json:"type"`        // 如 "军规级 手枪"
	MarketName   string        `json:"market_name"` // 如 "xxx (久经沙场)"
	MarketHash   string        `json:"market_hash_name"`
	Descriptions []interface{} `json:"descriptions"` // 不用细解
	Tags         []steamTag    `json:"tags"`
}
type steamTag struct {
	Category              string `json:"category"`                // Type/Rarity/Exterior 等
	LocalizedCategoryName string `json:"localized_category_name"` // "类型"/"品质"/"外观"
	LocalizedTagName      string `json:"localized_tag_name"`      // "手枪"/"军规级"/"久经沙场"
}
type steamAssetPropsWrap struct {
	Appid           int                `json:"appid"`
	ContextID       string             `json:"contextid"`
	AssetID         string             `json:"assetid"`
	AssetProperties []steamAssetPropKV `json:"asset_properties"`
}
type steamAssetPropKV struct {
	PropertyID int    `json:"propertyid"`
	FloatValue string `json:"float_value"`
	IntValue   string `json:"int_value"`
	Name       string `json:"name"`
}

// 返回给前端预览的数据
type SteamItem struct {
	AssetID   string `json:"assetId"`
	Name      string `json:"name"`
	ImageURL  string `json:"imageUrl"`
	Quality   string `json:"quality"`   // 白色/浅蓝色/…/金色
	Wear      string `json:"wear"`      // 崭新出厂/略有磨损/…
	Category  string `json:"category"`  // 手枪/步枪/冲锋枪/霰弹枪/机枪/匕首/手套/印花/探员/其他
	WearValue string `json:"wearValue"` // 0~1 字符串
}

// 预览：抓取并解析
func (s *SteamService) FetchInventory(steamID string, count int) ([]SteamItem, error) {
	if steamID == "" {
		return nil, errors.New("steamID 不能为空")
	}
	if strings.Contains(steamID, "steamcommunity.com/inventory/") {
		// 支持整段 URL，取第 4 段 path 为 steamID
		parts := strings.Split(steamID, "/")
		for i, p := range parts {
			if p == "inventory" && i+1 < len(parts) {
				steamID = parts[i+1]
				break
			}
		}
	}
	if count <= 0 || count > 2000 {
		count = 1000
	}
	url := fmt.Sprintf("https://steamcommunity.com/inventory/%s/730/2?l=schinese&count=%d", steamID, count)
	resp, err := s.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求 steam 失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("steam 返回 %d: %s", resp.StatusCode, string(b))
	}
	var inv steamInventoryResp
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return nil, fmt.Errorf("解析 steam JSON 失败: %v", err)
	}
	if inv.Success != 1 {
		return nil, fmt.Errorf("steam 返回失败")
	}

	// 建索引
	descIdx := map[string]steamDesc{} // key: classid_instanceid
	for _, d := range inv.Descriptions {
		key := d.ClassID + "_" + d.InstanceID
		descIdx[key] = d
	}
	propsIdx := map[string]steamAssetPropsWrap{} // key: assetid
	for _, p := range inv.AssetProperties {
		propsIdx[p.AssetID] = p
	}

	var items []SteamItem
	for _, a := range inv.Assets {
		d, ok := descIdx[a.ClassID+"_"+a.InstanceID]
		if !ok {
			continue
		}
		item := SteamItem{
			AssetID:   a.AssetID,
			Name:      d.Name,
			ImageURL:  steamIcon(d.IconURL),
			Quality:   mapRarityToQuality(findTag(d.Tags, "Rarity")),
			Wear:      findTag(d.Tags, "Exterior"),
			Category:  mapTypeToCategory(findTag(d.Tags, "Type")),
			WearValue: findWearValue(propsIdx[a.AssetID]),
		}
		// 兜底
		if item.Category == "" {
			item.Category = "其他"
		}
		items = append(items, item)
	}
	return items, nil
}

// 导入：为当前用户批量创建资产（尝试下载图片，失败则用默认图）
func (s *SteamService) ImportForUser(userID int, org int, steamID string, selectAssetIDs []string) ([]model.Asset, error) {
	if org != 2 {
		return nil, errors.New("仅创作者组织(Org2)可导入")
	}
	items, err := s.FetchInventory(steamID, 1000)
	if err != nil {
		return nil, err
	}
	chooseAll := len(selectAssetIDs) == 0
	need := map[string]bool{}
	for _, id := range selectAssetIDs {
		need[id] = true
	}
	var created []model.Asset
	for _, it := range items {
		if !chooseAll && !need[it.AssetID] {
			continue
		}
		imageName := model.DefaultImageName
		if it.ImageURL != "" {
			if name, derr := s.downloadImage(it.ImageURL); derr == nil {
				imageName = name
			}
		}
		asset, cerr := s.as.CreateAsset(
			it.Name, imageName, userID, userID,
			"从 Steam 导入", it.Quality, it.Wear, it.Category, it.WearValue, org,
		)
		if cerr != nil {
			// 不中断，继续下一条
			continue
		}
		created = append(created, asset)
	}
	return created, nil
}

func steamIcon(icon string) string {
	if icon == "" {
		return ""
	}
	// 标准画像 CDN
	return "https://steamcommunity-a.akamaihd.net/economy/image/" + icon
}

func findTag(tags []steamTag, cat string) string {
	for _, t := range tags {
		if t.Category == cat {
			return t.LocalizedTagName
		}
	}
	return ""
}

func mapRarityToQuality(rarity string) string {
	switch rarity {
	case "消费级":
		return "白色"
	case "工业级":
		return "浅蓝色"
	case "军规级":
		return "深蓝色"
	case "受限":
		return "紫色"
	case "保密":
		return "粉紫色"
	case "隐秘":
		return "红色"
	case "违禁":
		return "金色"
	default:
		return ""
	}
}

func mapTypeToCategory(typ string) string {
	switch typ {
	case "手枪":
		return "手枪"
	case "步枪":
		return "步枪"
	case "狙击步枪":
		return "步枪"
	case "微型冲锋枪", "冲锋枪":
		return "冲锋枪"
	case "霰弹枪":
		return "霰弹枪"
	case "机枪":
		return "机枪"
	case "匕首":
		return "匕首"
	case "手套":
		return "手套"
	case "贴纸", "印花":
		return "印花"
	case "特工", "探员":
		return "探员"
	default:
		return "其他"
	}
}

func findWearValue(w steamAssetPropsWrap) string {
	for _, kv := range w.AssetProperties {
		if kv.Name == "磨损率" && kv.FloatValue != "" {
			return kv.FloatValue
		}
	}
	return ""
}

// 下载图片保存到 public/images
func (s *SteamService) downloadImage(url string) (string, error) {
	resp, err := s.http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("下载失败：%d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "png") {
		ext = ".png"
	}
	name := uuid.New().String() + ext
	path := filepath.Join(model.DefaultImageFolder, name)
	if err := model.EnsureDir(model.DefaultImageFolder); err != nil {
		return "", err
	}
	if err := model.WriteFile(path, data); err != nil {
		return "", err
	}
	return name, nil
}
