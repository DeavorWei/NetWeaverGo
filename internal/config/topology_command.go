package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// GetTopologyVendorFieldCommands 返回指定厂商的字段命令映射。
func GetTopologyVendorFieldCommands(vendor string) ([]models.TopologyVendorFieldCommand, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	normalizedVendor := strings.ToLower(strings.TrimSpace(vendor))
	if normalizedVendor == "" {
		return nil, fmt.Errorf("厂商不能为空")
	}

	var records []models.TopologyVendorFieldCommand
	if err := DB.Where("vendor = ?", normalizedVendor).Order("field_key ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// ListTopologyVendorFieldCommands 返回全部厂商字段命令映射。
func ListTopologyVendorFieldCommands() ([]models.TopologyVendorFieldCommand, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var records []models.TopologyVendorFieldCommand
	if err := DB.Order("vendor ASC, field_key ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// SaveTopologyVendorFieldCommands 覆盖保存指定厂商的字段命令映射。
func SaveTopologyVendorFieldCommands(vendor string, commands []models.TopologyVendorFieldCommand) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	normalizedVendor := strings.ToLower(strings.TrimSpace(vendor))
	if normalizedVendor == "" {
		return fmt.Errorf("厂商不能为空")
	}

	normalized := normalizeTopologyVendorFieldCommands(normalizedVendor, commands)
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("vendor = ?", normalizedVendor).Delete(&models.TopologyVendorFieldCommand{}).Error; err != nil {
			return err
		}
		if len(normalized) == 0 {
			return nil
		}
		return tx.Create(&normalized).Error
	})
}

// EnsureTopologyVendorCommandSeeds 将内置画像命令作为配置域种子写入数据库缺失项。
func EnsureTopologyVendorCommandSeeds(seed map[string][]models.TopologyVendorFieldCommand) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(seed) == 0 {
		return nil
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		for vendor, items := range seed {
			normalizedVendor := strings.ToLower(strings.TrimSpace(vendor))
			if normalizedVendor == "" {
				continue
			}
			normalized := normalizeTopologyVendorFieldCommands(normalizedVendor, items)
			for _, item := range normalized {
				var count int64
				if err := tx.Model(&models.TopologyVendorFieldCommand{}).
					Where("vendor = ? AND field_key = ?", item.Vendor, item.FieldKey).
					Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					continue
				}
				copyItem := item
				if err := tx.Create(&copyItem).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func normalizeTopologyVendorFieldCommands(vendor string, commands []models.TopologyVendorFieldCommand) []models.TopologyVendorFieldCommand {
	result := make([]models.TopologyVendorFieldCommand, 0, len(commands))
	seen := make(map[string]struct{})
	for _, item := range commands {
		fieldKey := strings.TrimSpace(item.FieldKey)
		if fieldKey == "" {
			continue
		}
		if _, exists := seen[fieldKey]; exists {
			continue
		}
		seen[fieldKey] = struct{}{}
		result = append(result, models.TopologyVendorFieldCommand{
			Vendor:     vendor,
			FieldKey:   fieldKey,
			Command:    strings.TrimSpace(item.Command),
			TimeoutSec: item.TimeoutSec,
			Enabled:    item.Enabled,
			Notes:      strings.TrimSpace(item.Notes),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].FieldKey < result[j].FieldKey
	})
	return result
}
