package xmlcfg

import (
	"regexp"
	"strings"
)

var (
	// Java 命名捕获组 (?<name>...) -> Go RE2 (?P<name>...)
	// 注意排除后行断言 (?<=...) 和 (?<!...)
	reJavaNamedGroup = regexp.MustCompile(`\(\?<([a-zA-Z][a-zA-Z0-9_]*)>`)

	// Unicode 字符转义 \u4e00 -> \x{4e00}
	reUnicodeEscape = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

	// Go RE2 重复次数上限为 1000，超过 1000 会报 invalid repeat count（如 {1,1024}）
	reLargeRepeat = regexp.MustCompile(`\{(\d+),(\d{4,})\}`)

	// 单字符负向先行断言改写：((?!X)[ \r\n\S]) -> [^X]，((?!X).) -> [^X]
	reLookaheadChar = regexp.MustCompile(`\(\?!([a-zA-Z0-9,:\s])\)\[[^\]]+\]`)
	reLookaheadDot  = regexp.MustCompile(`\(\?!([a-zA-Z0-9,:\s])\)\.`)

	// 简单后行断言 (?<=...) 尝试剥除断言转为非捕获分组
	reLookbehind = regexp.MustCompile(`\(\?<=[^)]+\)`)
)

// CleanAndCompileRegex 规范化正则字符串并适配 Go RE2 引擎
// 返回: 规范化后的正则字符串, 编译后的 *regexp.Regexp, 是否进行了语法适配改写, 错误
func CleanAndCompileRegex(raw string) (string, *regexp.Regexp, bool, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil, false, nil
	}

	// 1. 折叠连续空白与换行（对齐 eDeskPro RegexModel.init() 折叠 \r\n + 缩进 为单行）
	reIndent := regexp.MustCompile(`[\r\n]+\s*`)
	s = reIndent.ReplaceAllString(s, "")

	modified := false

	// 2. 限制超出 RE2 上限（1000）的重复计数，例如 {1,1024} -> {1,1000}
	if reLargeRepeat.MatchString(s) {
		s = reLargeRepeat.ReplaceAllStringFunc(s, func(m string) string {
			sub := reLargeRepeat.FindStringSubmatch(m)
			return "{" + sub[1] + ",1000}"
		})
		modified = true
	}

	// 3. 转换 Java 命名捕获组 (?<name>...) 为 (?P<name>...)
	if reJavaNamedGroup.MatchString(s) {
		s = reJavaNamedGroup.ReplaceAllString(s, `(?P<$1>`)
		modified = true
	}

	// 4. 转换 Unicode 转义 \u4e00 -> \x{4e00}
	if reUnicodeEscape.MatchString(s) {
		s = reUnicodeEscape.ReplaceAllString(s, `\x{$1}`)
		modified = true
	}

	// 5. 单字符负向先行断言改写
	if reLookaheadChar.MatchString(s) {
		s = reLookaheadChar.ReplaceAllString(s, `[^$1]`)
		modified = true
	}
	if reLookaheadDot.MatchString(s) {
		s = reLookaheadDot.ReplaceAllString(s, `[^$1]`)
		modified = true
	}

	// 6. 确保开启跨行与忽略大小写 (?im) 模式
	if strings.HasPrefix(s, "(?i)") {
		s = "(?im)" + strings.TrimPrefix(s, "(?i)")
	} else if !strings.HasPrefix(s, "(?im)") {
		s = "(?im)" + s
	}

	re, err := regexp.Compile(s)
	if err == nil {
		return s, re, modified, nil
	}

	// 7. 若编译失败，尝试自动平改常见不兼容语法（例如后行断言）
	if reLookbehind.MatchString(s) {
		rewritten := reLookbehind.ReplaceAllStringFunc(s, func(m string) string {
			content := m[4 : len(m)-1]
			return "(?:" + content + ")"
		})
		if re2, err2 := regexp.Compile(rewritten); err2 == nil {
			return rewritten, re2, true, nil
		}
	}

	// 无法自动改写时返回原始错误
	return s, nil, modified, err
}
