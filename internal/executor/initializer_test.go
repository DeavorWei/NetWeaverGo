package executor

import (
	"context"
	"testing"
)

func TestInitializer_DeprecatedDirectIOPath(t *testing.T) {
	i := NewInitializer(nil)

	result := i.RunWithResult(context.Background(), nil, nil)
	if result == nil {
		t.Fatalf("RunWithResult 返回 nil")
	}
	if result.Success {
		t.Fatalf("RunWithResult 不应成功")
	}
	if result.ErrorMessage == "" {
		t.Fatalf("RunWithResult 应返回废弃错误信息")
	}

	if err := i.Run(context.Background(), nil, nil); err == nil {
		t.Fatalf("Run 应返回废弃错误")
	}
	if err := i.QuickInit(context.Background(), nil, nil); err == nil {
		t.Fatalf("QuickInit 应返回废弃错误")
	}
	if err := i.SendPagerContinue(nil); err == nil {
		t.Fatalf("SendPagerContinue 应返回废弃错误")
	}
}
