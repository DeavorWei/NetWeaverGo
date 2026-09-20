package taskexec

import "testing"

// P0-6：AR/NE 型号必须归入内置规则的 "Router" 族，否则 Router 专属规则不可达
func TestResolveAlarmFamily(t *testing.T) {
	cases := map[string]string{
		"NE40E":    "Router",
		"NE9000":   "Router",
		"AR6140":   "Router",
		"CE6865":   "CE",
		"S5735":    "S",
		"USG6600":  "USG",
		"":         "COMMON",
		"Unknown":  "COMMON",
		"AR-6120":  "Router",
		"ne20e-x6": "Router",
	}
	for model, want := range cases {
		if got := resolveAlarmFamily(model); got != want {
			t.Errorf("resolveAlarmFamily(%q) = %q, want %q", model, got, want)
		}
	}
}
