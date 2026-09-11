package parser

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitBlocks(t *testing.T) {
	sample := `
Slot 1:
  Board Type : LPU
  Status     : Normal
Slot 2:
  Board Type : MPU
  Status     : Normal
Slot 3:
  Board Type : SFU
  Status     : Abnormal
`
	re := regexp.MustCompile(`(?m)^Slot\s+(\d+):`)

	t.Run("KeepHeader True", func(t *testing.T) {
		blocks := SplitBlocks(sample, re, true)
		assert.Equal(t, 3, len(blocks))

		assert.Equal(t, "Slot 1:", blocks[0].Match)
		assert.Contains(t, blocks[0].Body, "Slot 1:")
		assert.Contains(t, blocks[0].Body, "Board Type : LPU")

		assert.Equal(t, "Slot 2:", blocks[1].Match)
		assert.Contains(t, blocks[1].Body, "Slot 2:")
		assert.Contains(t, blocks[1].Body, "Board Type : MPU")

		assert.Equal(t, "Slot 3:", blocks[2].Match)
		assert.Contains(t, blocks[2].Body, "Slot 3:")
		assert.Contains(t, blocks[2].Body, "Board Type : SFU")
		assert.Contains(t, blocks[2].Body, "Status     : Abnormal")
	})

	t.Run("KeepHeader False", func(t *testing.T) {
		blocks := SplitBlocks(sample, re, false)
		assert.Equal(t, 3, len(blocks))

		assert.Equal(t, "Slot 1:", blocks[0].Match)
		assert.NotContains(t, blocks[0].Body, "Slot 1:")
		assert.Contains(t, blocks[0].Body, "Board Type : LPU")
	})

	t.Run("No Match and Empty Input", func(t *testing.T) {
		blocks := SplitBlocks("", re, true)
		assert.Nil(t, blocks)

		noMatchRe := regexp.MustCompile(`^NonExistent`)
		blocks = SplitBlocks(sample, noMatchRe, true)
		assert.Nil(t, blocks)
	})
}
