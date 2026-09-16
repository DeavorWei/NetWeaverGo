package terminal

import (
	"testing"
)

func TestANSIParser_DEC_AltScreen(t *testing.T) {
	parser := NewANSIParser()

	// 开启 Alt Screen: \x1b[?1049h
	tokens := parser.Parse("\x1b[?1049hHello")
	if len(tokens) != 2 {
		t.Fatalf("预期 2 个 tokens，实际: %d", len(tokens))
	}
	if tokens[0].IsText {
		t.Errorf("token 0 应为命令")
	}
	cmd := tokens[0].Cmd
	if cmd.Type != CmdDecSet {
		t.Errorf("预期 CmdDecSet，实际: %v", cmd.Type)
	}
	if !cmd.IsAltScreen() {
		t.Errorf("预期 IsAltScreen 为 true")
	}
	if string(tokens[1].Text) != "Hello" {
		t.Errorf("预期文本 Hello，实际: %s", string(tokens[1].Text))
	}

	// 退出 Alt Screen: \x1b[?1049l
	tokens2 := parser.Parse("\x1b[?1049lWorld")
	if len(tokens2) != 2 {
		t.Fatalf("预期 2 个 tokens")
	}
	cmd2 := tokens2[0].Cmd
	if cmd2.Type != CmdDecReset {
		t.Errorf("预期 CmdDecReset，实际: %v", cmd2.Type)
	}
	if !cmd2.IsAltScreen() {
		t.Errorf("预期 IsAltScreen 为 true")
	}
}

func TestANSIParser_DEC_Cursor(t *testing.T) {
	parser := NewANSIParser()

	// 隐藏光标: \x1b[?25l
	tokensHide := parser.Parse("\x1b[?25l")
	if len(tokensHide) != 1 || tokensHide[0].IsText {
		t.Fatalf("解析失败")
	}
	vis, isCursor := tokensHide[0].Cmd.CursorVisible()
	if !isCursor || vis {
		t.Errorf("预期 isCursor=true, visible=false，实际: isCursor=%v, vis=%v", isCursor, vis)
	}

	// 显示光标: \x1b[?25h
	tokensShow := parser.Parse("\x1b[?25h")
	if len(tokensShow) != 1 || tokensShow[0].IsText {
		t.Fatalf("解析失败")
	}
	vis, isCursor = tokensShow[0].Cmd.CursorVisible()
	if !isCursor || !vis {
		t.Errorf("预期 isCursor=true, visible=true，实际: isCursor=%v, vis=%v", isCursor, vis)
	}
}

func TestANSIParser_OSC_Title(t *testing.T) {
	parser := NewANSIParser()

	// OSC 0 BEL
	inputBEL := "\x1b]0;NetWeaver-Core-Switch\x07TextContent"
	tokensBEL := parser.Parse(inputBEL)
	if len(tokensBEL) != 2 {
		t.Fatalf("预期 2 个 tokens，实际: %d", len(tokensBEL))
	}
	if tokensBEL[0].Cmd.Type != CmdOSC {
		t.Errorf("预期 CmdOSC，实际: %v", tokensBEL[0].Cmd.Type)
	}
	if title := tokensBEL[0].Cmd.OSCTitle(); title != "NetWeaver-Core-Switch" {
		t.Errorf("预期标题 NetWeaver-Core-Switch，实际: %s", title)
	}
	if string(tokensBEL[1].Text) != "TextContent" {
		t.Errorf("预期文本 TextContent，实际: %s", string(tokensBEL[1].Text))
	}

	// OSC 2 ST (\x1b\)
	inputST := "\x1b]2;Device-H3C-S6520\x1b\\OutputLine"
	tokensST := parser.Parse(inputST)
	if len(tokensST) != 2 {
		t.Fatalf("预期 2 个 tokens，实际: %d", len(tokensST))
	}
	if tokensST[0].Cmd.Type != CmdOSC {
		t.Errorf("预期 CmdOSC，实际: %v", tokensST[0].Cmd.Type)
	}
	if title := tokensST[0].Cmd.OSCTitle(); title != "Device-H3C-S6520" {
		t.Errorf("预期标题 Device-H3C-S6520，实际: %s", title)
	}
}

func TestANSIParser_StandardCSI(t *testing.T) {
	parser := NewANSIParser()

	// 擦除整行 \x1b[2K
	tokens := parser.Parse("\x1b[2KPrompt>")
	if len(tokens) != 2 {
		t.Fatalf("预期 2 个 tokens")
	}
	if tokens[0].Cmd.Type != CmdEraseInLine {
		t.Errorf("预期 CmdEraseInLine，实际: %v", tokens[0].Cmd.Type)
	}
	if len(tokens[0].Cmd.Params) != 1 || tokens[0].Cmd.Params[0] != 2 {
		t.Errorf("预期参数 [2]，实际: %v", tokens[0].Cmd.Params)
	}
	if string(tokens[1].Text) != "Prompt>" {
		t.Errorf("预期 Prompt>，实际: %s", string(tokens[1].Text))
	}
}
