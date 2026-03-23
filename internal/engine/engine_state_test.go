package engine

import "testing"

func TestEngineStateTransitionsWithoutPaused(t *testing.T) {
	sm := NewEngineStateManager()

	if err := sm.TransitionTo(StateStarting); err != nil {
		t.Fatalf("Idle -> Starting 应成功: %v", err)
	}
	if err := sm.TransitionTo(StateRunning); err != nil {
		t.Fatalf("Starting -> Running 应成功: %v", err)
	}
	if err := sm.TransitionTo(StateClosing); err != nil {
		t.Fatalf("Running -> Closing 应成功: %v", err)
	}
	if err := sm.TransitionTo(StateClosed); err != nil {
		t.Fatalf("Closing -> Closed 应成功: %v", err)
	}
}

func TestEngineStateStringWithoutPaused(t *testing.T) {
	tests := []struct {
		state EngineState
		want  string
	}{
		{StateIdle, "Idle"},
		{StateStarting, "Starting"},
		{StateRunning, "Running"},
		{StateClosing, "Closing"},
		{StateClosed, "Closed"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Fatalf("state=%v string=%q want=%q", tt.state, got, tt.want)
		}
	}
}
