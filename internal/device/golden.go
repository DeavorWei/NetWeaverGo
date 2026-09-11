package device

import "sort"

// NormalizeIdentityForGolden 生成稳定可比的身份快照，供 golden 测试使用。
//
// 与硬件树同理：Evidence 是切片、Raws 是 map，直接序列化会引入顺序抖动，
// 且 Raws 只是原始回显缓存（json:"-"），不参与比对。
func NormalizeIdentityForGolden(id *Identity) *Identity {
	if id == nil {
		return nil
	}

	cp := *id
	if len(id.Evidence) > 0 {
		evidence := append([]string(nil), id.Evidence...)
		sort.Strings(evidence)
		cp.Evidence = evidence
	} else {
		cp.Evidence = nil
	}
	cp.Raws = nil
	return &cp
}
