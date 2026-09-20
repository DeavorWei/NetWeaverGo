package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayer_GoldenCisco(t *testing.T) {
	goldenPath := filepath.Join("..", "..", "testdata", "ansi", "cisco_golden.txt")
	content, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	replayer := NewReplayer(80)
	events := replayer.Process(string(content))
	assert.NotEmpty(t, events)

	lines := replayer.Lines()
	assert.NotEmpty(t, lines)
	assert.Contains(t, lines[0], "Router#terminal length 0")
}

// P1-8：DEC 私有模式与 OSC 必须被真实消费（备用屏/光标显隐/标题）并计入统计
func TestReplayer_GoldenDecOsc(t *testing.T) {
	goldenPath := filepath.Join("..", "..", "testdata", "ansi", "dec_osc_golden.txt")
	content, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	replayer := NewReplayer(80)
	events := replayer.Process(string(content))
	require.NotEmpty(t, events)

	assert.False(t, replayer.AltScreenActive(), "序列闭合后不应处于备用屏")
	assert.True(t, replayer.CursorVisible(), "序列闭合后光标应可见")
	assert.Equal(t, "Huawei VRP Console", replayer.OSCTitle())

	stats := replayer.ControlSequenceStats()
	assert.GreaterOrEqual(t, stats.DecSetCount, 1)
	assert.GreaterOrEqual(t, stats.DecResetCount, 1)
	assert.Equal(t, 1, stats.OSCCount)
	assert.Equal(t, 0, stats.UnknownCount, "本样本不应包含未支持序列")

	lines := strings.Join(replayer.Lines(), "\n")
	assert.Contains(t, lines, "Info: The max number of VTY users is 10.")
}

func TestReplayer_GoldenHuawei(t *testing.T) {
	goldenPath := filepath.Join("..", "..", "testdata", "ansi", "huawei_golden.txt")
	content, err := os.ReadFile(goldenPath)
	require.NoError(t, err)

	replayer := NewReplayer(80)
	events := replayer.Process(string(content))
	assert.NotEmpty(t, events)

	lines := replayer.Lines()
	assert.NotEmpty(t, lines)
	assert.Contains(t, lines[0], "<Huawei>screen-length 0 temporary")
}
