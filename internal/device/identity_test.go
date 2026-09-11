package device

import (
	"testing"
)

func TestIdentify_HuaweiDiverseModels(t *testing.T) {
	testCases := []struct {
		name           string
		raws           map[string]string
		expectedModel  string
		expectedSeries string
		expectedVer    string
		hasPatch       bool
	}{
		{
			name: "S5735 Switch",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (S5735 V200R019C00SPC500)
HUAWEI S5735-L24P4S-A2 uptime is 100 days
`,
			},
			expectedModel:  "S5735",
			expectedSeries: "S5700",
			expectedVer:    "V200R019C00SPC500",
		},
		{
			name: "CE6866 DataCenter Switch",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 8.180 (CE6866 V200R005C10SPC600)
HUAWEI CE6866-48S8CQ-EI uptime is 50 days
`,
			},
			expectedModel:  "CE6866",
			expectedSeries: "CE6800",
			expectedVer:    "V200R005C10SPC600",
		},
		{
			name: "CE16804 Chassis Switch",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 8.200 (CE16804 V200R019C10SPC800)
HUAWEI CE16804 uptime is 20 days
`,
			},
			expectedModel:  "CE16800",
			expectedSeries: "CE16800",
			expectedVer:    "V200R019C10SPC800",
		},
		{
			name: "AR6280 Router",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (AR6280 V300R019C11SPC200)
HUAWEI AR6280 uptime is 12 days
`,
			},
			expectedModel:  "AR6280",
			expectedSeries: "AR6000",
			expectedVer:    "V300R019C11SPC200",
		},
		{
			name: "NE40E Router",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 8.180 ((NE40E-X8) V800R011C00SPC200)
HUAWEI NE40E-X8 uptime is 300 days
`,
			},
			expectedModel:  "NE40E",
			expectedSeries: "NE40E",
			expectedVer:    "V800R011C00SPC200",
		},
		{
			name: "NE5000E Multi-chassis",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 8.160 ((NE5000E-X16) V800R010C00SPC100)
HUAWEI NE5000E-X16 uptime is 400 days
`,
			},
			expectedModel:  "NE5000E",
			expectedSeries: "NE5000",
			expectedVer:    "V800R010C00SPC100",
		},
		{
			name: "USG6600 Firewall Priority",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (USG6600 V500R005C20SPC500)
HUAWEI USG6680 uptime is 60 days
`,
				"display device": `USG6680's Device status:
Slot # Sub Type Online Power Register
`,
			},
			expectedModel:  "USG6680",
			expectedSeries: "USG6000",
			expectedVer:    "V500R005C20SPC500",
		},
		{
			name: "AC6005 Access Controller",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (AC6005-8 V200R019C00SPC500)
HUAWEI AC6005-8 uptime is 80 days
`,
			},
			expectedModel:  "AC6005",
			expectedSeries: "AC6000",
			expectedVer:    "V200R019C00SPC500",
		},
		{
			name: "AirEngine AP",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (AirEngine 5760-10 V200R019C00SPC500)
HUAWEI AirEngine 5760-10 uptime is 15 days
`,
			},
			expectedModel:  "AirEngine5760",
			expectedSeries: "AirEngine5700",
			expectedVer:    "V200R019C00SPC500",
		},
		{
			name: "AP6050DN Cloud AP Suffix Strip",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (AP6050DN-CLOUD V200R019C00SPC500)
HUAWEI AP6050DN-CLOUD uptime is 5 days
`,
			},
			expectedModel:  "AP6050",
			expectedSeries: "AP6000",
			expectedVer:    "V200R019C00SPC500",
		},
		{
			name: "Patch Information Included",
			raws: map[string]string{
				"display version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (S5700 V200R019C00SPC500)
HUAWEI S5720-LI uptime is 10 days
`,
				"display patch-information": `
Patch Package Name     : flash:/patch.pat
Patch Package Version  : V200R019SPH005
The state of the patch : Running
`,
			},
			expectedModel:  "S5720",
			expectedSeries: "S5700",
			expectedVer:    "V200R019C00SPC500",
			hasPatch:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := Identify("huawei", tc.raws)
			if err != nil {
				t.Fatalf("Identify 失败: %v", err)
			}
			if id.Model != tc.expectedModel {
				t.Errorf("Model = %q; want %q", id.Model, tc.expectedModel)
			}
			if id.Series != tc.expectedSeries {
				t.Errorf("Series = %q; want %q", id.Series, tc.expectedSeries)
			}
			if tc.expectedVer != "" && id.Version != tc.expectedVer {
				t.Errorf("Version = %q; want %q", id.Version, tc.expectedVer)
			}
			if tc.hasPatch && id.Patch == "" {
				t.Errorf("期望提取到补丁，但实际为空")
			}
			if len(id.Evidence) == 0 {
				t.Errorf("期望包含判定证据 (Evidence)，但为空")
			}
		})
	}
}

func TestIdentify_H3CAndCisco(t *testing.T) {
	// 1. H3C
	h3cRaw := map[string]string{
		"display version": `H3C Comware Software, Version 7.1.070, Release 2416P02
Copyright (c) 2004-2017 New H3C Technologies Co., Ltd. All rights reserved.
H3C S5500-28C-EI uptime is 12 weeks, 3 days
`,
	}
	idH3C, err := Identify("h3c", h3cRaw)
	if err != nil {
		t.Fatalf("H3C 识别失败: %v", err)
	}
	if idH3C.Model != "S5500" {
		t.Errorf("H3C Model = %q, want S5500", idH3C.Model)
	}
	if idH3C.Series != "S5500" {
		t.Errorf("H3C Series = %q, want S5500", idH3C.Series)
	}

	// 2. Cisco
	ciscoRaw := map[string]string{
		"show version": `Cisco IOS Software, IOS-XE Software, Catalyst L3 Switch Software (CAT3K_CAA-UNIVERSALK9-M), Version 16.9.4
cisco WS-C3850-24T (MIPS) processor with 2097152K bytes of physical memory.
`,
	}
	idCisco, err := Identify("cisco", ciscoRaw)
	if err != nil {
		t.Fatalf("Cisco 识别失败: %v", err)
	}
	if idCisco.Model != "WS-C3850-24T" {
		t.Errorf("Cisco Model = %q, want WS-C3850-24T", idCisco.Model)
	}
}

func TestIdentify_GenericAndAutoCorrect(t *testing.T) {
	// 1. 纯未知厂商与通用回显
	genericRaw := map[string]string{
		"version": `Linux server 5.15.0-78-generic #85-Ubuntu SMP Fri Jul 7 15:25:09 UTC 2023 x86_64
release: 22.04.2
`,
	}
	idGen, err := Identify("linux", genericRaw)
	if err != nil {
		t.Fatalf("通用识别失败: %v", err)
	}
	if idGen.Vendor != "linux" {
		t.Errorf("期望保留原始 vendor 'linux', 实际: %s", idGen.Vendor)
	}
	if idGen.Version != "22.04.2" && idGen.Version == "" {
		t.Errorf("通用识别期望提取到版本")
	}
	if len(idGen.Evidence) == 0 {
		t.Errorf("通用识别期望包含 evidence")
	}

	// 2. 未知厂商但回显包含华为特征自动校正
	unknownHwRaw := map[string]string{
		"version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (S5700 V200R019C00SPC500)
HUAWEI S5735-L24P4S-A2 uptime is 10 days
`,
	}
	idHwAuto, err := Identify("unknown_vendor", unknownHwRaw)
	if err != nil {
		t.Fatalf("自动校正识别失败: %v", err)
	}
	if idHwAuto.Vendor != "huawei" {
		t.Errorf("未识别出华为特征自动校正, vendor=%s", idHwAuto.Vendor)
	}
	if idHwAuto.Model != "S5735" {
		t.Errorf("自动校正后型号错误: %s", idHwAuto.Model)
	}

	// 3. patch_version 键名不与 version 冲突
	patchAndVerRaw := map[string]string{
		"patch_info": `Patch Package Version : V200R019SPH005`,
		"version": `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (S5700 V200R019C00SPC500)
HUAWEI S5720-LI uptime is 10 days
`,
	}
	idMulti, err := Identify("huawei", patchAndVerRaw)
	if err != nil {
		t.Fatalf("多键联合识别失败: %v", err)
	}
	if idMulti.Model != "S5720" {
		t.Errorf("多键识别 Model = %q, want S5720", idMulti.Model)
	}
	if idMulti.Patch != "V200R019SPH005" {
		t.Errorf("多键识别 Patch = %q, want V200R019SPH005", idMulti.Patch)
	}

	// 4. 包含 BIOS 文本但非 Cisco IOS 的设备，不应被误判为 Cisco
	biosRaw := map[string]string{
		"version": `Server BIOS Version 2.1.0, Build Date: 2023-01-01
Manufacturer: Generic Server
`,
	}
	idBios, err := Identify("generic", biosRaw)
	if err != nil {
		t.Fatalf("识别失败: %v", err)
	}
	if idBios.Vendor == "cisco" {
		t.Errorf("BIOS 文本不应误判为 Cisco, 实际得到 vendor: %s", idBios.Vendor)
	}
}
