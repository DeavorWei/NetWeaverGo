package device

import (
	"testing"
)

func TestConvertSeries_FrozenContractTable(t *testing.T) {
	testCases := []struct {
		inputModel     string
		expectedSeries string
		comment        string
	}{
		{"S5735-S", "S5700", "S系列交换机（保留前2位，其余置0）"},
		{"S5720-LI", "S5700", "S系列交换机"},
		{"S6730-H", "S6700", "S系列交换机"},
		{"CE6866", "CE6800", "数据中心交换机（4位数字）"},
		{"CE16804", "CE16800", "5位数字：len=5, offset=1 -> 保留 3 位数字"},
		{"CE16808", "CE16800", "5位数字：len=5, offset=1 -> 保留 3 位数字"},
		{"AR6280", "AR6000", "AR 路由器（第2组：保留首位数字）"},
		{"AR1220", "AR1000", "AR 路由器"},
		{"AC6005", "AC6000", "AC 控制器（Model 由 handle_final 得 AC6005，Series 得 AC6000）"},
		{"AP7060DN", "AP7000", "AP 终端（保留首位数字）"},
		{"AirEngine 5760-10", "AirEngine5700", "去空格 + 4位保留前2位"},
		{"S9700D", "S9700D", "特例：含 9700D 后缀加 D"},
		{"E600", "E000", "E6 特例组"},
		{"NE5000E", "NE5000", "NE 路由器系列"},
	}

	for _, tc := range testCases {
		actual := ConvertSeries(tc.inputModel)
		if actual != tc.expectedSeries {
			t.Errorf("ConvertSeries(%q) = %q; want %q (%s)", tc.inputModel, actual, tc.expectedSeries, tc.comment)
		}
	}
}
