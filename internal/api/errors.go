package api

import "errors"

var (
	errGroupMissing    = errors.New("监控分组不存在")
	errNotifierMissing = errors.New("通知渠道不存在")
)
