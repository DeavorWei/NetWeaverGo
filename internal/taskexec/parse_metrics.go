package taskexec

import (
	"github.com/NetWeaverGo/core/internal/metrics"
	"github.com/NetWeaverGo/core/internal/parser"
)

// parseWithMetrics 统一解析入口：在持有 RunID 的调用点按运行维度记录解析指标。
//
// 设计要点（方案 §10.2）：不在 ParserManager（全局单例）上取区间差值，
// 而是由调用方显式传入 RunID，多任务并发时指标互不串扰。
func parseWithMetrics(runID string, cliParser parser.CliParser, commandKey, rawText string) ([]map[string]string, error) {
	if cliParser == nil {
		metrics.Default.Inc(runID, "parse.miss", 1)
		return nil, nil
	}

	// 支持 DetailedParser 时可区分"命中模板"与"应急降级"
	if detailed, ok := cliParser.(parser.DetailedParser); ok {
		rows, outcome, err := detailed.ParseDetail(commandKey, rawText)
		switch {
		case err != nil:
			metrics.Default.Inc(runID, "parse.miss", 1)
			metrics.Default.LabelInc(runID, "parse.engine", outcome.Engine)
		case outcome.Fallback:
			metrics.Default.Inc(runID, "parse.fallback", 1)
			metrics.Default.LabelInc(runID, "parse.engine", outcome.Engine)
		default:
			metrics.Default.Inc(runID, "parse.template_hit", 1)
			metrics.Default.LabelInc(runID, "parse.engine", outcome.Engine)
		}
		return rows, err
	}

	rows, err := cliParser.Parse(commandKey, rawText)
	if err != nil {
		metrics.Default.Inc(runID, "parse.miss", 1)
	} else {
		metrics.Default.Inc(runID, "parse.template_hit", 1)
	}
	return rows, err
}
