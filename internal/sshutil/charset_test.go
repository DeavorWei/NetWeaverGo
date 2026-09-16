package sshutil

import (
	"bytes"
	"io"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestCharsetDecoder_UTF8(t *testing.T) {
	d := NewCharsetDecoder("auto")
	input := []byte("华为技术有限公司 Huawei Technologies Co., Ltd.")
	detected := d.AutoDetect(input)
	if detected != "utf-8" {
		t.Errorf("AutoDetect = %s, want utf-8", detected)
	}

	output, err := d.Decode(input)
	if err != nil {
		t.Fatalf("Decode 错误: %v", err)
	}
	if output != string(input) {
		t.Errorf("Decode = %s, want %s", output, string(input))
	}
}

func TestCharsetDecoder_GBK(t *testing.T) {
	d := NewCharsetDecoder("auto")

	// 构造 GBK 编码数据: "接口物理状态正常"
	src := "接口物理状态正常"
	gbkData, err := io.ReadAll(transform.NewReader(bytes.NewReader([]byte(src)), simplifiedchinese.GBK.NewEncoder()))
	if err != nil {
		t.Fatalf("编码 GBK 失败: %v", err)
	}

	detected := d.AutoDetect(gbkData)
	if detected != "gbk" {
		t.Errorf("AutoDetect = %s, want gbk", detected)
	}

	decoded, err := d.Decode(gbkData)
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}
	if decoded != src {
		t.Errorf("Decode = %s, want %s", decoded, src)
	}
}

func TestNewCharsetReader_GBK(t *testing.T) {
	src := "思科系统设备状态检查"
	gbkData, err := io.ReadAll(transform.NewReader(bytes.NewReader([]byte(src)), simplifiedchinese.GBK.NewEncoder()))
	if err != nil {
		t.Fatalf("编码 GBK 失败: %v", err)
	}

	reader := NewCharsetReader(bytes.NewReader(gbkData), "gbk")
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if string(out) != src {
		t.Errorf("转码结果 = %s, want %s", string(out), src)
	}
}
