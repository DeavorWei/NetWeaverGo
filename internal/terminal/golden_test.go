package terminal

import (
	"os"
	"path/filepath"
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
