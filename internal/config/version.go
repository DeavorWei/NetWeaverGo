package config

import "strings"

// appVersion 当前应用版本号，默认 dev。
// 由 cmd/netweaver 在启动时通过 SetAppVersion 注入构建期 ldflags（-X main.version）的值。
var appVersion = "dev"

// SetAppVersion 设置应用版本；空值忽略，保持原值。
func SetAppVersion(v string) {
	if trimmed := strings.TrimSpace(v); trimmed != "" {
		appVersion = trimmed
	}
}

// GetAppVersion 获取当前应用版本。
func GetAppVersion() string {
	return appVersion
}
