package matcher

import "testing"

func TestIsPromptRejectsStandaloneConfigSeparator(t *testing.T) {
	m := NewStreamMatcher()

	if m.IsPrompt("#\r\n") {
		t.Fatalf("standalone # should not be treated as a device prompt")
	}
}

func TestIsPromptAcceptsDevicePromptWithAnsiSequence(t *testing.T) {
	m := NewStreamMatcher()

	if !m.IsPrompt("\x1b[16D<Huawei>\r\n") {
		t.Fatalf("device prompt wrapped by ANSI cursor control should be detected")
	}
}

func TestIsPaginationPromptIgnoresAnsiNoise(t *testing.T) {
	m := NewStreamMatcher()

	if !m.IsPaginationPrompt("\x1b[16D---- More ----") {
		t.Fatalf("pagination prompt should still be detected after ANSI cleanup")
	}
}
