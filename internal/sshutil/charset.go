package sshutil

import (
	"bytes"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// CharsetDecoder 字符集自动探测与转码
type CharsetDecoder struct {
	defaultCharset string // "utf-8" | "gbk" | "auto"
}

// NewCharsetDecoder 创建字符集转码器
func NewCharsetDecoder(defaultCharset string) *CharsetDecoder {
	if defaultCharset == "" {
		defaultCharset = "auto"
	}
	return &CharsetDecoder{
		defaultCharset: defaultCharset,
	}
}

// AutoDetect 基于高字节分布启发式判断字符集
// 返回 "utf-8" 或 "gbk"
func (d *CharsetDecoder) AutoDetect(data []byte) string {
	if len(data) == 0 || utf8.Valid(data) {
		return "utf-8"
	}
	// 检查是否包含合法的 GBK 编码双字节序列
	// GBK 编码范围: 首字节 0x81-0xFE, 尾字节 0x40-0x7E 或 0x80-0xFE
	gbkValid := false
	for i := 0; i < len(data); {
		b := data[i]
		if b < 0x80 {
			i++
			continue
		}
		if b >= 0x81 && b <= 0xFE && i+1 < len(data) {
			b2 := data[i+1]
			if (b2 >= 0x40 && b2 <= 0x7E) || (b2 >= 0x80 && b2 <= 0xFE) {
				gbkValid = true
				i += 2
				continue
			}
		}
		i++
	}
	if gbkValid {
		return "gbk"
	}
	return "utf-8"
}

// Decode 尝试 UTF-8 解码，若指定为 GBK 或自动探测判定为 GBK 时转码为 UTF-8
func (d *CharsetDecoder) Decode(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	charset := d.defaultCharset
	if charset == "auto" {
		charset = d.AutoDetect(data)
	}

	switch charset {
	case "gbk":
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return string(data), err
		}
		return string(decoded), nil
	default:
		if utf8.Valid(data) {
			return string(data), nil
		}
		// 非合法 UTF-8 尝试以 GBK 兜底
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err == nil {
			return string(decoded), nil
		}
		return string(data), nil
	}
}

// NewCharsetReader 创建带转码的 Reader
func NewCharsetReader(r io.Reader, charset string) io.Reader {
	if charset == "utf-8" || charset == "" {
		return r
	}
	if charset == "gbk" {
		return transform.NewReader(r, simplifiedchinese.GBK.NewDecoder())
	}
	return r
}
