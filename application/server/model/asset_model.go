package model

import (
	"time"
)

// Asset 资产信息
// 先不管稀有度，因为稀有度应该由平台给定，而不是由上传用户给定
type Asset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ImageName   string    `json:"imageName"`
	AuthorId    int       `json:"authorId"`
	OwnerId     int       `json:"ownerId"`
	Description string    `json:"description"`
	Quality     string    `json:"quality"`
	Wear        string    `json:"wear"`
	Category    string    `json:"category"`   // 新增：枪械种类
	WearValue   string    `json:"wearValue"`  // 新增：磨损度值(0~1)，字符串存储避免精度问题
	TimeStamp   time.Time `json:"timeStamp"`
}

type TransferAssetRequest struct {
	ID         string `json:"id"`
	NewOwnerId int    `json:"newOwnerId"`
}
