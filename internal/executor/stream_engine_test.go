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

func TestStreamEngineWaitAndClearInitResidual_SendsFirstCommand(t *testing.T) {
	reader := &scriptReader{
		chunks: []string{
			"Info: login ok\r\n<S1>",
			"\r\n<S1>",
		},
	}
	writer := &writeBuffer{}

	client := &sshutil.SSHClient{
		IP:     "192.168.58.200",
		Stdin:  writer,
		Stdout: reader,
	}

	engine := NewStreamEngine(nil, client, []string{"disp int b"}, 80)

	err := engine.waitAndClearInitResidual(context.Background(), 2*time.Second)
	if err != nil {
		t.Fatalf("简化初始化不应失败: %v", err)
	}

	got := writer.String()
	if !strings.Contains(got, "\n") {
		t.Fatalf("应至少发送过预热空行，实际输出为 %q", got)
	}
	if !strings.Contains(got, "disp int b\n") {
		t.Fatalf("应在初始化完成后立即发送首条命令，实际输出为 %q", got)
	}

	if engine.adapter.NewState() != NewStateRunning {
		t.Fatalf("首条命令发送后状态应为 Running，实际是 %s", engine.adapter.NewState())
	}

	current := engine.adapter.CurrentCommand()
	if current == nil || current.Command != "disp int b" {
		t.Fatalf("当前命令上下文异常: %+v", current)
	}
}
