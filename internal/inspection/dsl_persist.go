package inspection

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NetWeaverGo/core/internal/logger"
)

// dslRuleFileName 导入 DSL 规则的持久化文件名（位于 storageRoot/inspection/ 下）
const dslRuleFileName = "dsl_rules_imported.json"

// DefaultImportedRulesPath 返回导入规则的默认持久化路径。
// 由调用方（ui / main）传入 storageRoot，避免 inspection 反向依赖 config 造成循环导入。
func DefaultImportedRulesPath(storageRoot string) string {
	if storageRoot == "" {
		return ""
	}
	return filepath.Join(storageRoot, "inspection", dslRuleFileName)
}

// SaveImportedRules 将当前生效的导入规则写入 JSON 文件（P2-8：重启后仍生效）
func SaveImportedRules(path string) error {
	if path == "" {
		return fmt.Errorf("导入规则持久化路径为空")
	}
	interp := GetGlobalDSLInterpreter()
	data, err := interp.ExportImportedRulesJSON()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

// LoadImportedRulesFromFile 从文件恢复导入规则，返回恢复条数；文件不存在时返回 0, nil
func LoadImportedRulesFromFile(path string) (int, error) {
	if path == "" {
		return 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count, err := GetGlobalDSLInterpreter().ImportRulesJSON(data)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		logger.Info("DSL", "-", "已从 %s 恢复 %d 条导入规则", path, count)
	}
	return count, nil
}
