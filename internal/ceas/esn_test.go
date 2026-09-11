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

func TestExtractESN_WLANAndMultiVendor(t *testing.T) {
	// 1. WLAN AC/AP 从 Unit_1 提取 ESN
	wlanElabel := `[Unit_1]
[Board Properties]
BoardType=AirEngine9700-M
BarCode=2102353WLAN10M1000001
Item=02353WLAN
`
	esnWLAN := ExtractESN("huawei", "AirEngine9700-M", wlanElabel, "")
	assert.Equal(t, "2102353WLAN10M1000001", esnWLAN)

	// 2. WLAN CLI AP SN
	esnAP := ExtractESN("huawei", "WLAN-AP", "", "AP SN : 2102354AP810N3000002")
	assert.Equal(t, "2102354AP810N3000002", esnAP)

	// 3. H3C 厂商
	esnH3C := ExtractESN("H3C", "S5500", "", "Device serial number : 210235A1234567890123")
	assert.Equal(t, "210235A1234567890123", esnH3C)

	// 4. Cisco 厂商
	esnCisco := ExtractESN("Cisco", "Catalyst3850", "", "System serial number : FOC12345678")
	assert.Equal(t, "FOC12345678", esnCisco)
}
