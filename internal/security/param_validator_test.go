package security

import (
	"testing"
)

func TestValidateNetworkParams(t *testing.T) {
	// 1. IP 校验
	if !ValidateIPv4("192.168.1.1") {
		t.Errorf("expected 192.168.1.1 valid")
	}
	if ValidateIPv4("256.0.0.1") || ValidateIPv4("abc") {
		t.Errorf("expected invalid IP rejected")
	}

	// 2. CIDR 校验
	if !ValidateIPOrCIDR("10.0.0.0/24") || !ValidateIPOrCIDR("172.16.0.1") {
		t.Errorf("expected valid IP/CIDR accepted")
	}
	if ValidateIPOrCIDR("10.0.0.0/33") || ValidateIPOrCIDR("invalid") {
		t.Errorf("expected invalid CIDR rejected")
	}

	// 3. Port 校验
	if !ValidatePort(22) || !ValidatePort(80) || !ValidatePort(65535) {
		t.Errorf("expected valid ports accepted")
	}
	if ValidatePort(0) || ValidatePort(65536) || ValidatePort(-1) {
		t.Errorf("expected out-of-range ports rejected")
	}

	// 4. VLAN 校验
	if !ValidateVlanID(1) || !ValidateVlanID(4094) {
		t.Errorf("expected valid vlan accepted")
	}
	if ValidateVlanID(0) || ValidateVlanID(4095) {
		t.Errorf("expected invalid vlan rejected")
	}

	// 5. 接口名校验
	if !ValidateInterfaceName("GE1/0/1") || !ValidateInterfaceName("100GE1/0/1:1") || !ValidateInterfaceName("Eth-Trunk10") {
		t.Errorf("expected valid interface names accepted")
	}
	if ValidateInterfaceName("GE1/0/1; rm -rf") || ValidateInterfaceName("") {
		t.Errorf("expected malicious interface rejected")
	}

	// 6. 安全命令白名单校验
	if err := ValidateSafeCommand("display version"); err != nil {
		t.Errorf("expected safe command allowed: %v", err)
	}
	if err := ValidateSafeCommand("display ip interface brief | include Up"); err == nil {
		t.Errorf("expected command with pipe rejected by safe validator")
	}
	if err := ValidateSafeCommand("display version; reboot"); err == nil {
		t.Errorf("expected command with semicolon rejected")
	}

	// 7. 路径穿越防护
	if !SanitizeFilePath("exports/report.csv") {
		t.Errorf("expected valid path accepted")
	}
	if SanitizeFilePath("../../etc/passwd") || SanitizeFilePath("..\\windows\\system32") {
		t.Errorf("expected traversal path rejected")
	}

	// 8. 槽位号校验
	if !ValidateSlotID("0") || !ValidateSlotID("1/0") || !ValidateSlotID("0/1/2") {
		t.Errorf("expected valid slot accepted")
	}
	if ValidateSlotID("") || ValidateSlotID("slotA") || ValidateSlotID("1/999") {
		t.Errorf("expected invalid slot rejected")
	}
}
