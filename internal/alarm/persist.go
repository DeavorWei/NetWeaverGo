package alarm

import (
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// PersistMergedPhenomena 将一批原始告警记录执行归并并持久化为故障现象（P0-6 统一入口），
// 同时回填 AlarmRecord.MergedPhenomenonID。重复执行时按 run_id + device_ip 幂等清理旧结果。
func PersistMergedPhenomena(db *gorm.DB, merger *AlarmMerger, records []models.AlarmRecord) ([]models.MergedPhenomenon, error) {
	if db == nil || len(records) == 0 {
		return nil, nil
	}
	if merger == nil {
		merger = NewAlarmMerger()
	}

	// 幂等：清理同 run、同设备集合下的历史归并结果，避免重复分析产生重复现象
	runID := records[0].RunID
	deviceSet := make(map[string]struct{}, len(records))
	for _, r := range records {
		if ip := r.DeviceIP; ip != "" {
			deviceSet[ip] = struct{}{}
		}
	}
	if runID != "" && len(deviceSet) > 0 {
		ips := make([]string, 0, len(deviceSet))
		for ip := range deviceSet {
			ips = append(ips, ip)
		}
		if err := db.Where("run_id = ? AND device_ip IN ?", runID, ips).Delete(&models.MergedPhenomenon{}).Error; err != nil {
			return nil, err
		}
	}

	phenomena := merger.Merge(records)
	for i := range phenomena {
		if err := db.Create(&phenomena[i]).Error; err != nil {
			return nil, err
		}
		// 回填 AlarmRecord 的 MergedPhenomenonID
		if len(phenomena[i].RecordIDs) > 0 {
			if err := db.Model(&models.AlarmRecord{}).
				Where("id IN ?", phenomena[i].RecordIDs).
				Update("merged_phenomenon_id", phenomena[i].ID).Error; err != nil {
				return nil, err
			}
		}
	}
	return phenomena, nil
}
