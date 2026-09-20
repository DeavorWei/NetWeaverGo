package sshutil

import (
	"bytes"
	"io"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
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

func TestNewCharsetReader_GB18030_And_Big5(t *testing.T) {
	// 1. GB18030 测试
	srcGB := "华三交换机测试"
	gbData, err := io.ReadAll(transform.NewReader(bytes.NewReader([]byte(srcGB)), simplifiedchinese.GB18030.NewEncoder()))
	if err != nil {
		t.Fatalf("编码 GB18030 失败: %v", err)
	}
	readerGB := NewCharsetReader(bytes.NewReader(gbData), "gb18030")
	outGB, err := io.ReadAll(readerGB)
	if err != nil || string(outGB) != srcGB {
		t.Fatalf("GB18030 转码失败: %v, got %s", err, string(outGB))
	}

	// 2. Big5 繁体测试
	srcBig5 := "網絡設備狀態"
	big5Data, err := io.ReadAll(transform.NewReader(bytes.NewReader([]byte(srcBig5)), traditionalchinese.Big5.NewEncoder()))
	if err != nil {
		t.Fatalf("编码 Big5 失败: %v", err)
	}
	readerBig5 := NewCharsetReader(bytes.NewReader(big5Data), "big5")
	outBig5, err := io.ReadAll(readerBig5)
	if err != nil || string(outBig5) != srcBig5 {
		t.Fatalf("Big5 转码失败: %v, got %s", err, string(outBig5))
	}
}
