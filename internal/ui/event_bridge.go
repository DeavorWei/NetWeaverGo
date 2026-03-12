package ui

import (
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventBridge 事件桥接器 - 统一处理事件转发到前端
// 解决多个服务中重复的事件监听器同步代码问题
type EventBridge struct {
	wailsApp *application.App
	ready    chan struct{}
	active   chan struct{}
}

// NewEventBridge 创建事件桥接器实例
func NewEventBridge() *EventBridge {
	return &EventBridge{
		ready:  make(chan struct{}),
		active: make(chan struct{}),
	}
}

// SetWailsApp 设置 Wails 应用实例
func (b *EventBridge) SetWailsApp(app *application.App) {
	b.wailsApp = app
}

// StartAndWait 启动事件监听并等待就绪
// 此方法会阻塞直到事件监听器完全就绪
// 返回一个 done channel，当事件循环结束时关闭
func (b *EventBridge) StartAndWait(frontendBus chan report.ExecutorEvent) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		// 等待 Wails 应用实例就绪（最多等待 1 秒）
		for i := 0; i < 100; i++ {
			if b.wailsApp != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// 通知 App 已就绪
		close(b.ready)

		// 等待启动信号
		<-b.active

		// 开始消费 FrontendBus
		for ev := range frontendBus {
			if b.wailsApp != nil {
				b.wailsApp.Event.Emit("device:event", ev)
			}
		}
	}()

	// 等待 Wails App 就绪
	<-b.ready

	// 发送启动信号，让事件循环开始
	close(b.active)

	return done
}

// StartWithTracker 启动事件监听并设置 Tracker（带进度追踪）
// 此方法包含快照推送功能，用于 EngineService
func (b *EventBridge) StartWithTracker(
	frontendBus chan report.ExecutorEvent,
	tracker *report.ProgressTracker,
	snapshotInterval time.Duration,
	ctx <-chan struct{},
) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		// 等待 Wails 应用实例就绪
		for i := 0; i < 100; i++ {
			if b.wailsApp != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// 通知 App 已就绪
		close(b.ready)

		// 等待启动信号
		<-b.active

		// 启动快照定时推送（如果提供了 tracker）
		var snapshotTicker *time.Ticker
		var snapshotStop chan struct{}
		if tracker != nil && snapshotInterval > 0 {
			snapshotTicker = time.NewTicker(snapshotInterval)
			snapshotStop = make(chan struct{})
			go b.runSnapshotPusher(tracker, snapshotTicker, snapshotStop, ctx)
		}

		// 开始消费 FrontendBus
		for ev := range frontendBus {
			if b.wailsApp != nil {
				b.wailsApp.Event.Emit("device:event", ev)
			}
		}

		// 停止快照推送
		if snapshotTicker != nil {
			snapshotTicker.Stop()
			close(snapshotStop)
		}
	}()

	// 等待就绪
	<-b.ready

	// 发送启动信号
	close(b.active)

	return done
}

// runSnapshotPusher 运行快照推送器
func (b *EventBridge) runSnapshotPusher(
	tracker *report.ProgressTracker,
	ticker *time.Ticker,
	stop <-chan struct{},
	ctx <-chan struct{},
) {
	for {
		select {
		case <-ticker.C:
			snapshot := tracker.GetSnapshot()
			if snapshot != nil && b.wailsApp != nil {
				b.wailsApp.Event.Emit(SnapshotEventName, snapshot)
			}
		case <-stop:
			return
		case <-ctx:
			return
		}
	}
}

// EmitFinished 发送执行完成事件
func (b *EventBridge) EmitFinished(tracker *report.ProgressTracker) {
	if b.wailsApp == nil {
		return
	}

	// 发送最终快照（100% 进度）
	if tracker != nil {
		snapshot := tracker.GetSnapshot()
		if snapshot != nil {
			snapshot.Progress = 100
			snapshot.IsRunning = false
			b.wailsApp.Event.Emit(SnapshotEventName, snapshot)
		}
	}

	// 发送完成事件
	b.wailsApp.Event.Emit(ExecutionFinishedEvent)
}

// EmitEvent 通用事件发送方法
func (b *EventBridge) EmitEvent(eventName string, data interface{}) {
	if b.wailsApp != nil {
		b.wailsApp.Event.Emit(eventName, data)
	}
}

// QuickBridge 快速桥接方法（简化版，用于不需要同步等待的场景）
// 返回 done channel
func QuickBridge(app *application.App, frontendBus chan report.ExecutorEvent) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)
		for ev := range frontendBus {
			if app != nil {
				app.Event.Emit("device:event", ev)
			}
		}
	}()

	return done
}

// EnsureAppReady 确保 Wails App 实例已就绪
// 超时返回 false
func EnsureAppReady(app **application.App, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if *app != nil {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	logger.Warn("EventBridge", "-", "等待 Wails App 实例超时")
	return false
}
