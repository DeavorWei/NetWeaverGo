package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type executionManager struct {
	appMu    sync.RWMutex
	wailsApp *application.App

	cancelFunc context.CancelFunc
	cancelMu   sync.Mutex

	currentTracker *report.ProgressTracker
	trackerMu      sync.RWMutex

	snapshotTicker *time.Ticker
	snapshotStop   chan struct{}
	stopOnce       sync.Once
	stopMu         sync.Mutex
}

type managedExecution struct {
	manager   *executionManager
	lifecycle *engine.Engine
	tracker   *report.ProgressTracker
	ctx       context.Context
}

var sharedExecutionManager = &executionManager{}

func getExecutionManager() *executionManager {
	return sharedExecutionManager
}

func (m *executionManager) SetWailsApp(app *application.App) {
	if app == nil {
		return
	}

	m.appMu.Lock()
	m.wailsApp = app
	m.appMu.Unlock()
}

func (m *executionManager) getWailsApp() *application.App {
	m.appMu.RLock()
	defer m.appMu.RUnlock()
	return m.wailsApp
}

func (m *executionManager) IsRunning() bool {
	return engine.IsEngineRunning()
}

func (m *executionManager) StopEngine() error {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()

	if m.cancelFunc == nil {
		return fmt.Errorf("引擎未运行")
	}

	m.cancelFunc()
	m.cancelFunc = nil
	logger.Info("Engine", "-", "已发送停止信号")

	return nil
}

func (m *executionManager) GetEngineState() map[string]interface{} {
	return engine.GetGlobalState().GetStatus()
}

func (m *executionManager) GetExecutionSnapshot() *report.ExecutionSnapshot {
	m.trackerMu.RLock()
	defer m.trackerMu.RUnlock()

	if m.currentTracker == nil {
		return nil
	}

	return m.currentTracker.GetSnapshot()
}

func (m *executionManager) RunEngine(
	ng *engine.Engine,
	runnerSrc string,
	runnerID string,
	taskName string,
	runFn func(context.Context) error,
) (*report.ProgressTracker, error) {
	tracker := report.NewProgressTracker(len(ng.Devices))
	tracker.SetTaskName(taskName)
	ng.SetTracker(tracker)

	session, err := m.beginExecution(ng, runnerSrc, runnerID, tracker)
	if err != nil {
		return nil, err
	}

	frontendDone := m.listenEvents(ng.FrontendBus, nil)
	runErr := runFn(session.ctx)
	<-frontendDone
	session.Finish()

	return tracker, runErr
}

func (m *executionManager) BeginCompositeExecution(
	lifecycle *engine.Engine,
	runnerSrc string,
	runnerID string,
	taskName string,
	totalDevices int,
) (*managedExecution, error) {
	tracker := report.NewProgressTracker(totalDevices)
	tracker.SetTaskName(taskName)

	return m.beginExecution(lifecycle, runnerSrc, runnerID, tracker)
}

func (m *executionManager) beginExecution(
	lifecycle *engine.Engine,
	runnerSrc string,
	runnerID string,
	tracker *report.ProgressTracker,
) (*managedExecution, error) {
	if lifecycle == nil {
		return nil, fmt.Errorf("引擎实例不能为空")
	}
	if tracker == nil {
		return nil, fmt.Errorf("进度追踪器不能为空")
	}

	if err := engine.GetGlobalState().SetActiveEngine(lifecycle, runnerSrc, runnerID); err != nil {
		tracker.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.setCancelFunc(cancel)
	m.setCurrentTracker(tracker)
	m.startSnapshotTicker(ctx)

	return &managedExecution{
		manager:   m,
		lifecycle: lifecycle,
		tracker:   tracker,
		ctx:       ctx,
	}, nil
}

func (m *executionManager) listenEvents(
	frontendBus <-chan report.ExecutorEvent,
	onEvent func(report.ExecutorEvent),
) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		for ev := range frontendBus {
			if onEvent != nil {
				onEvent(ev)
			}

			if app := m.getWailsApp(); app != nil {
				app.Event.Emit("device:event", ev)
			}
		}
	}()

	return done
}

func (m *executionManager) setCancelFunc(cancel context.CancelFunc) {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.cancelFunc = cancel
}

func (m *executionManager) clearCancelFunc() {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.cancelFunc = nil
}

func (m *executionManager) setCurrentTracker(tracker *report.ProgressTracker) {
	m.trackerMu.Lock()
	defer m.trackerMu.Unlock()
	m.currentTracker = tracker
}

func (m *executionManager) clearCurrentTracker() {
	m.trackerMu.Lock()
	defer m.trackerMu.Unlock()
	m.currentTracker = nil
}

func (m *executionManager) startSnapshotTicker(ctx context.Context) {
	ticker := time.NewTicker(SnapshotInterval)
	stop := make(chan struct{})

	m.stopMu.Lock()
	m.snapshotTicker = ticker
	m.snapshotStop = stop
	m.stopOnce = sync.Once{}
	m.stopMu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				m.stopSnapshotTicker()
				return
			case <-stop:
				return
			case <-ticker.C:
				snapshot := m.GetExecutionSnapshot()
				if snapshot == nil {
					continue
				}
				if app := m.getWailsApp(); app != nil {
					app.Event.Emit(SnapshotEventName, snapshot)
				}
			}
		}
	}()
}

func (m *executionManager) stopSnapshotTicker() {
	m.stopOnce.Do(func() {
		m.stopMu.Lock()
		defer m.stopMu.Unlock()

		if m.snapshotTicker != nil {
			m.snapshotTicker.Stop()
			m.snapshotTicker = nil
		}
		if m.snapshotStop != nil {
			close(m.snapshotStop)
			m.snapshotStop = nil
		}
	})
}

func (m *executionManager) emitFinishedEvent() {
	snapshot := m.GetExecutionSnapshot()
	if snapshot != nil {
		snapshot.Progress = 100
		snapshot.IsRunning = false
		if app := m.getWailsApp(); app != nil {
			app.Event.Emit(SnapshotEventName, snapshot)
		}
	}

	if app := m.getWailsApp(); app != nil {
		app.Event.Emit(ExecutionFinishedEvent)
	}
}

func (s *managedExecution) Context() context.Context {
	return s.ctx
}

func (s *managedExecution) Tracker() *report.ProgressTracker {
	return s.tracker
}

func (s *managedExecution) TransitionTo(state engine.EngineState) error {
	if s.lifecycle == nil {
		return nil
	}
	return s.lifecycle.TransitionTo(state)
}

func (s *managedExecution) Finish() {
	s.finishLifecycle()
	s.manager.stopSnapshotTicker()
	s.manager.clearCancelFunc()
	engine.GetGlobalState().ClearActiveEngine()
	s.manager.emitFinishedEvent()
	s.manager.clearCurrentTracker()

	if s.tracker != nil {
		s.tracker.Close()
	}
}

func (s *managedExecution) finishLifecycle() {
	if s.lifecycle == nil {
		return
	}

	switch s.lifecycle.State() {
	case engine.StateClosed:
		return
	case engine.StateClosing:
		_ = s.lifecycle.TransitionTo(engine.StateClosed)
	case engine.StateIdle, engine.StateStarting, engine.StateRunning, engine.StatePaused:
		_ = s.lifecycle.TransitionTo(engine.StateClosing)
		_ = s.lifecycle.TransitionTo(engine.StateClosed)
	}
}
