package group

import "github.com/cng1985/nwatch/internal/modules/monitoring/shared"

// GroupRequest 是分组的创建、修改和删除动作参数。
type GroupRequest struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}

// GroupSimple 与 GroupResponse 是分组的业务结果。
type GroupSimple = shared.Group
type GroupResponse = shared.Group

// GroupItem 是分组列表行，附带组内监控数量。
type GroupItem struct {
	shared.Group
	MonitorCount int64 `json:"monitorCount"`
}

// GroupListResponse 是分组列表结果。
type GroupListResponse struct {
	Items []GroupItem `json:"items"`
}

// GroupOKResponse 是删除动作的回执。
type GroupOKResponse struct {
	OK bool `json:"ok"`
}
