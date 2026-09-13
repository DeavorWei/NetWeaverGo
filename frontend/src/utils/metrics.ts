/**
 * 运行指标快照解析工具（对应后端 internal/metrics registry 输出）。
 *
 * 后端结构：{"counters":{"cache.hit":3},"labels":{"device.handler":2}}
 * 说明：计数器为累计值；标签为按维度分桶的计数（label 基数溢出归 __overflow__）。
 */

export interface MetricsSnapshot {
  counters: Record<string, number>;
  labels: Record<string, number>;
}

/** 解析 metrics_json 文本；空值、非法 JSON 或全空结果返回 null（调用方据此隐藏卡片） */
export function parseMetrics(raw?: string | null): MetricsSnapshot | null {
  if (!raw) return null;
  try {
    const data = JSON.parse(raw) as Record<string, unknown>;
    if (!data || typeof data !== "object") return null;

    const counters = sanitizeNumberMap(data.counters);
    const labels = sanitizeNumberMap(data.labels);
    if (Object.keys(counters).length === 0 && Object.keys(labels).length === 0) {
      return null;
    }
    return { counters, labels };
  } catch {
    return null;
  }
}

function sanitizeNumberMap(value: unknown): Record<string, number> {
  if (!value || typeof value !== "object") return {};
  const out: Record<string, number> = {};
  for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
    const num = typeof raw === "number" ? raw : Number(raw);
    if (Number.isFinite(num)) out[key] = num;
  }
  return out;
}

/** 指标键的中文说明（未收录的键回退原始键名） */
const METRIC_LABELS: Record<string, string> = {
  "cache.hit": "命令缓存命中",
  "cache.miss": "命令缓存未命中",
  "confirm.triggered": "二次确认触发",
  "risk.hit": "风险命令命中",
  "echo.truncated": "回显截断",
  "device.handler": "设备形态处理器命中",
  "profile.match_path": "画像匹配路径",
  "ceas.node_count": "CEAS 节点数",
  "ceas.parse_fail": "CEAS 解析失败",
  "inspection.result_code": "巡检结果码分布",
};

export function metricLabel(key: string): string {
  return METRIC_LABELS[key] ?? key;
}
