package ceas

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractESN_FromDisplayESN(t *testing.T) {
	tests := []struct {
		name     string
		rawESN   string
		expected string
	}{
		{
			name:     "ESN of master",
			rawESN:   "ESN of master: 2102352BBR10J8000099",
			expected: "2102352BBR10J8000099",
		},
		{
			name:     "Equipment Serial Number",
			rawESN:   "Equipment Serial Number : 2102311KLT10L3000088",
			expected: "2102311KLT10L3000088",
		},
		{
			name:     "ESN: simple",
			rawESN:   "ESN: 2103031AAA10P7000077",
			expected: "2103031AAA10P7000077",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esn := ExtractESN("huawei", "CE6800", "", tt.rawESN)
			assert.Equal(t, tt.expected, esn)
		})
	}
}

func TestExtractESN_FallbackToELabel(t *testing.T) {
	rawElabel := `[BackPlane_1]
BoardType=CE6866
BarCode=2102352BBR10J8000001
Item=02312DWR
`
	esn := ExtractESN("huawei", "CE6800", rawElabel, "")
	assert.Equal(t, "2102352BBR10J8000001", esn)
}
