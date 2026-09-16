package executor

import (
	"errors"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// BypassRequest 紧急放行请求
type BypassRequest struct {
	RunID           string `json:"runId"`
	DeviceIP        string `json:"deviceIp"`
	Command         string `json:"command"`
	RuleID          uint   `json:"ruleId"`
	Operator        string `json:"operator"`
	Reason          string `json:"reason"`
	SecondConfirmed bool   `json:"secondConfirmed"`
}

// ValidateBypass 校验放行请求合法性（二次确认 + 理由不少于5个字符）
func ValidateBypass(req BypassRequest) error {
	if !req.SecondConfirmed {
		return errors.New("放行失败: 必须经过二次确认")
	}
	trimmedReason := strings.TrimSpace(req.Reason)
	if len([]rune(trimmedReason)) < 5 {
		return errors.New("放行失败: 必须填写有效的放行理由（不少于5个字）")
	}
	if strings.TrimSpace(req.Operator) == "" {
		return errors.New("放行失败: 操作人不能为空")
	}
	return nil
}

// RecordRiskLog 记录高危命令审计日志（留痕）
func RecordRiskLog(log models.RiskCommandLog) {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	logger.Info("RiskAudit", log.DeviceIP, "[高危审计] runID=%s action=%s operator=%s ruleID=%d cmd=%q reason=%q",
		log.RunID, log.Action, log.Operator, log.RuleID, log.Command, log.Reason)

	db := config.GetDB()
	if db != nil {
		go func(entry models.RiskCommandLog) {
			if err := db.Create(&entry).Error; err != nil {
				logger.Warn("RiskAudit", entry.DeviceIP, "写入高危审计日志失败: %v", err)
			}
		}(log)
	}
}
