// Package version 记录构建版本信息，由构建脚本通过 ldflags 注入。
package version

// Version 是当前发布版本。构建时可用 -ldflags "-X ...version.Version=v1.2.3" 覆盖。
var Version = "1.0.0"
