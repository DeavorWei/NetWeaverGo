package parser

import (
	"regexp"
)

// Block 分块文本单元
type Block struct {
	Match string // 块头原文（splitRegex 命中的文本）
	Body  string // 块体（含或不含块头，由 keepHeader 决定）
	Start int    // 在原始文本中的起始偏移
	End   int    // 在原始文本中的结束偏移
}

// SplitBlocks 按正则表达式将原始回显切分成块。
// 算法严格借鉴 eDesk Pro pre_parse.find_pos_by_pattern 的精髓：
// 1. 保留块头（由 keepHeader 控制 Body 是否包含块头原文）；
// 2. 保证尾块完整闭合（直到文本末尾，不吞字符）；
// 3. 无匹配时安全返回空切片，不抛异常。
func SplitBlocks(echo string, re *regexp.Regexp, keepHeader bool) []Block {
	if len(echo) == 0 || re == nil {
		return nil
	}

	indices := re.FindAllStringIndex(echo, -1)
	if len(indices) == 0 {
		return nil
	}

	blocks := make([]Block, 0, len(indices))
	echoLen := len(echo)

	for i := 0; i < len(indices); i++ {
		matchStart := indices[i][0]
		matchEnd := indices[i][1]

		var blockEnd int
		if i+1 < len(indices) {
			blockEnd = indices[i+1][0]
		} else {
			blockEnd = echoLen
		}

		matchText := echo[matchStart:matchEnd]
		var bodyText string
		var actualStart int

		if keepHeader {
			actualStart = matchStart
			bodyText = echo[matchStart:blockEnd]
		} else {
			actualStart = matchEnd
			bodyText = echo[matchEnd:blockEnd]
		}

		blocks = append(blocks, Block{
			Match: matchText,
			Body:  bodyText,
			Start: actualStart,
			End:   blockEnd,
		})
	}

	return blocks
}
