package ui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/executor"
)

func TestSuspendManager_ConcurrentFinishAndResolve_NoPanic(t *testing.T) {
	m := &SuspendManager{
		sessions:     make(map[string]*SuspendSession),
		sessionsByIP: make(map[string]string),
	}

	handler := m.CreateHandler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan executor.ErrorAction, 1)
	go func() {
		resultCh <- handler(ctx, "192.168.1.1", "error", "display version")
	}()

	var sessionID string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		sessionID = m.sessionsByIP["192.168.1.1"]
		m.mu.Unlock()
		if sessionID != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if sessionID == "" {
		t.Fatal("挂起会话未创建")
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		cancel()
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			m.Resolve(sessionID, "C")
		}
	}()

	wg.Wait()

	select {
	case action := <-resultCh:
		if action != executor.ActionAbort && action != executor.ActionContinue {
			t.Fatalf("unexpected action: %v", action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler 未按预期退出")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sessions) != 0 {
		t.Fatalf("挂起会话未清理: %d", len(m.sessions))
	}
	if len(m.sessionsByIP) != 0 {
		t.Fatalf("IP 索引未清理: %d", len(m.sessionsByIP))
	}
}

func TestSuspendManager_ResolveAfterFinish_IsIgnored(t *testing.T) {
	m := &SuspendManager{
		sessions:     make(map[string]*SuspendSession),
		sessionsByIP: make(map[string]string),
	}

	session := &SuspendSession{
		ID:       "finished-session",
		IP:       "192.168.1.2",
		ActionCh: make(chan executor.ErrorAction, 1),
	}
	session.finished.Store(true)

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.sessionsByIP[session.IP] = session.ID
	m.mu.Unlock()

	m.Resolve(session.ID, "C")

	select {
	case <-session.ActionCh:
		t.Fatal("finished 会话不应再收到动作")
	default:
	}
}
