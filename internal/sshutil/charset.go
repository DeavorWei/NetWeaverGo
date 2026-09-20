package sshutil

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// CharsetDecoder 字符集自动探测与转码
type CharsetDecoder struct {
	defaultCharset string // "utf-8" | "gbk" | "gb18030" | "big5" | "auto"
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

// Decode 尝试 UTF-8 解码，若指定为 GBK/GB18030/Big5 或自动探测判定为非 UTF-8 时转码为 UTF-8
func (d *CharsetDecoder) Decode(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	charset := d.defaultCharset
	if charset == "auto" {
		charset = d.AutoDetect(data)
	}

	c := strings.ToLower(strings.TrimSpace(charset))
	switch c {
	case "gbk", "cp936":
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return string(data), err
		}
		return string(decoded), nil
	case "gb18030":
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GB18030.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return string(data), err
		}
		return string(decoded), nil
	case "big5":
		reader := transform.NewReader(bytes.NewReader(data), traditionalchinese.Big5.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return string(data), err
		}
		return string(decoded), nil
	case "hz-gb2312", "hzgb2312":
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.HZGB2312.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return string(data), err
		}
		return string(decoded), nil
	case "utf-8", "utf8":
		return string(data), nil
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

// charsetTransformer 根据字符集名返回解码 Transformer；utf-8/空 返回 nil 表示无需转码。
func charsetTransformer(charset string) transform.Transformer {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8":
		return nil
	case "gbk", "cp936":
		return simplifiedchinese.GBK.NewDecoder()
	case "gb18030":
		return simplifiedchinese.GB18030.NewDecoder()
	case "big5":
		return traditionalchinese.Big5.NewDecoder()
	case "hz-gb2312", "hzgb2312":
		return simplifiedchinese.HZGB2312.NewDecoder()
	default:
		// 未知字符集按 GBK 兜底，保证中文设备可读
		return simplifiedchinese.GBK.NewDecoder()
	}
}

// autoDetectSampleSize 自动探测采样字节数（首块）
const autoDetectSampleSize = 4096

// autoDetectCharsetReader 支持 Charset="auto" 的首块自动探测 Reader。
// 首次 Read 时先读取至多 4KB 样本判定字符集，再把"已读样本 + 剩余流"拼接后统一转码，
// 避免在探测边界处切断多字节字符。
type autoDetectCharsetReader struct {
	src     io.Reader
	decoded io.Reader
	ready   bool
}

// Read 实现 io.Reader
func (a *autoDetectCharsetReader) Read(p []byte) (int, error) {
	if !a.ready {
		sample := make([]byte, autoDetectSampleSize)
		n, err := io.ReadAtLeast(a.src, sample, 1)
		if n == 0 {
			a.ready = true
			if err == nil {
				err = io.EOF
			}
			return 0, err
		}
		sample = sample[:n]

		charset := NewCharsetDecoder("auto").AutoDetect(sample)
		transformer := charsetTransformer(charset)
		merged := io.MultiReader(bytes.NewReader(sample), a.src)
		if transformer == nil {
			a.decoded = merged
		} else {
			a.decoded = transform.NewReader(merged, transformer)
		}
		a.ready = true
	}
	return a.decoded.Read(p)
}

// NewCharsetReader 创建带转码的 Reader；charset="auto" 时按首块字节自动探测。
func NewCharsetReader(r io.Reader, charset string) io.Reader {
	if strings.EqualFold(strings.TrimSpace(charset), "auto") {
		return &autoDetectCharsetReader{src: r}
	}
	transformer := charsetTransformer(charset)
	if transformer == nil {
		return r
	}
	return transform.NewReader(r, transformer)
}
