package xmlcfg

import (
	"strings"
	"testing"
)

// P2-1：正则语法改写必须留痕（可追溯、可人工复核）
func TestCleanAndCompileRegex_RecordsRewrite(t *testing.T) {
	ResetRewriteRecords()

	_, re, modified, err := CleanAndCompileRegex(`\u4e00{1,1024}`)
	if err != nil || re == nil {
		t.Fatalf("改写后应可编译: err=%v", err)
	}
	if !modified {
		t.Fatal("应标记为已改写")
	}

	records := RewriteRecords()
	if len(records) == 0 {
		t.Fatal("应记录改写留痕")
	}
	found := false
	for _, rec := range records {
		if strings.Contains(rec.Reason, "unicode-escape") &&
			strings.Contains(rec.Rewritten, `\x{4e00}`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("改写留痕内容不正确: %+v", records)
	}

	// 后行断言（RE2 不支持）改写必须留痕
	ResetRewriteRecords()
	_, re2, modified2, err2 := CleanAndCompileRegex(`(?<=abc)def`)
	if err2 != nil || re2 == nil {
		t.Fatalf("后行断言应被平改后可编译: err=%v", err2)
	}
	if !modified2 {
		t.Fatal("后行断言改写应标记 modified")
	}
	records2 := RewriteRecords()
	if len(records2) == 0 {
		t.Fatal("后行断言改写应留痕")
	}
	if !strings.Contains(records2[0].Reason, "lookbehind-removed") {
		t.Fatalf("后行断言改写原因不正确: %+v", records2[0])
	}
}
