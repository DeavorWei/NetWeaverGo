package executor

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/sshutil"
)

type scriptReader struct {
	chunks []string
	index  int
}

func (r *scriptReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	copy(p, []byte(chunk))
	return len(chunk), nil
}

type writeBuffer struct {
	strings.Builder
}

func (w *writeBuffer) Close() error { return nil }

func TestStreamEngineRunPlaybook_UnifiedPathSendsWarmupAndCommand(t *testing.T) {
	reader := &scriptReader{
		chunks: []string{
			"Info: login ok\r\n<S1>",
			"\r\n<S1>",
			"disp int b\r\nline-1\r\n<S1>",
		},
	}
	writer := &writeBuffer{}

	client := &sshutil.SSHClient{
		IP:     "192.168.58.200",
		Stdin:  writer,
		Stdout: reader,
		Stderr: strings.NewReader(""),
	}

	engine := NewStreamEngine(nil, client, []string{"disp int b"}, 80)

	results, err := engine.RunPlaybook(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("统一执行路径不应失败: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("应返回 1 条命令结果，实际 %d", len(results))
	}

	got := writer.String()
	if !strings.Contains(got, "\n") {
		t.Fatalf("应至少发送过预热空行，实际输出为 %q", got)
	}
	if !strings.Contains(got, "disp int b\n") {
		t.Fatalf("应在初始化完成后立即发送首条命令，实际输出为 %q", got)
	}

	if engine.adapter.NewState() != NewStateCompleted {
		t.Fatalf("命令完成后状态应为 Completed，实际是 %s", engine.adapter.NewState())
	}

	if results[0].Command != "disp int b" {
		t.Fatalf("命令结果异常: %+v", results[0])
	}
}

func TestStreamEngineRunPlaybook_ContinueOnCmdErrorWaitsForPrompt(t *testing.T) {
	reader := &scriptReader{
		chunks: []string{
			"<SW>",
			"\r\n<SW>",
			"display arp all\r\n^\r\nError: Too many parameters found at '^' position.\r\n<SW>",
			"display device\r\nLSW's Device status:\r\nSlot  Card   Type\r\n1     -      LSW\r\n<SW>",
		},
	}
	writer := &writeBuffer{}

	client := &sshutil.SSHClient{
		IP:     "192.168.58.200",
		Stdin:  writer,
		Stdout: reader,
		Stderr: strings.NewReader(""),
	}

	engine := NewStreamEngine(nil, client, []string{"display arp all", "display device"}, 80)
	engine.adapter.SetContinueOnCmdError(true)

	results, err := engine.RunPlaybook(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("ContinueOnCmdError 场景不应失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("应返回 2 条命令结果，实际 %d", len(results))
	}
	if results[0].Success {
		t.Fatalf("第一条命令应失败: %+v", results[0])
	}
	if !strings.Contains(results[0].NormalizedText, "Error: Too many parameters") {
		t.Fatalf("第一条命令应保留设备错误输出，实际 %q", results[0].NormalizedText)
	}
	if !results[1].Success {
		t.Fatalf("第二条命令应成功: %+v", results[1])
	}
	if !strings.Contains(results[1].NormalizedText, "LSW's Device status:") {
		t.Fatalf("第二条命令应拿到 display device 回显，实际 %q", results[1].NormalizedText)
	}

	got := writer.String()
	if !strings.Contains(got, "display arp all\n") || !strings.Contains(got, "display device\n") {
		t.Fatalf("应按顺序发送两条命令，实际发送: %q", got)
	}
}
