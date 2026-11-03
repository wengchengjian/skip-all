// Package engine 负责把游戏截图、Feature 状态机和输入执行串起来。
package engine

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/geom"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
)

// State 是会话状态。
type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StatePaused  State = "paused"
	StateStopped State = "stopped"
	StateError   State = "error"
)

// Event 是 engine 推给 UI 的一条状态/日志事件。
type Event struct {
	Time    time.Time
	Level   string
	State   State
	Message string
}

// Device 抽象游戏窗口截图与前台状态，便于测试替换。
type Device interface {
	Capture() (*image.RGBA, float64, error)
	IsActive() bool
	Activate() error
}

// FrameDevice 是可选的扩展：能返回带引用计数的帧缓冲。
// 实现它之后，异步识别 worker 可以直接读原像素，不必为每一帧深拷贝。
type FrameDevice interface {
	CaptureFrame() (assist.Frame, error)
}

// Input 抽象点击与按键，便于测试替换。
type Input interface {
	Click(geom.Point) error
	MiddleClick(geom.Point) error
	ClickHold(geom.Point, time.Duration) error
	// Press 按下左键并保持，Release 抬起。按住时长超过一拍时用这两个，
	// 调用方不必在按住期间阻塞，可以继续截图决策。
	Press(geom.Point) error
	Release() error
	ClickCurrent() error
	KeyTap(string) error
	ReleaseAll() error
}

// FeatureSpec 是 engine 要同时运行的一个功能及其设置。
type FeatureSpec struct {
	Feature  assist.Feature
	Settings assist.Settings
}

// Config 是 engine 的运行参数。
type Config struct {
	TargetFPS           int
	PauseWhenBackground bool
	ActivateWindow      bool
	// ProcessAlive 报告游戏进程是否还在运行；为空时不做进程门控。
	// 进程消失时自动暂停（避免截图连续失败把会话打成异常），进程回来时自动恢复。
	ProcessAlive func() bool
	// AsyncOCRWorkers 是异步识别 worker 数；<=1 表示同步执行。
	AsyncOCRWorkers int
}

// FeatureStatus 是单个功能的状态快照。
type FeatureStatus struct {
	ID      string
	Name    string
	State   State
	Message string
	Error   string
}

// Engine 管理一个游戏上的多个功能会话：同一帧截图喂给全部功能，动作串行执行。
type Engine struct {
	mu       sync.Mutex
	device   Device
	input    Input
	screens  *screen.Set
	vision   vision.Matcher
	logger   *slog.Logger
	features []*featureRuntime
	cfg      Config
	onEvent  func(Event)

	state       State
	lastMessage string
	lastError   string
	startedAt   time.Time
	cancel      context.CancelFunc
	fps         float64
	// 最近一拍的时间轴信息，用于观测节拍是否被截图/识别拖慢。
	tickID      uint64
	tickLag     time.Duration
	tickDropped uint64
	// clock 是固定步长节拍时钟；Run 期间非 nil。
	clock *tickClock
	// releaseTimer 管理异步按住的抬起时间。
	releaseTimer *time.Timer
	// actionMu 串行化"执行动作"与"暂停/停止时的抬起"，
	// 否则暂停的抬起可能和某一拍的 Press 交错，导致暂停后键还按着。
	actionMu sync.Mutex
	// gatePaused 表示当前暂停是"游戏进程不在"造成的自动暂停，进程回来后自动恢复。
	gatePaused bool
	// userPaused 表示用户按了暂停；此时进程回来也不自动恢复。
	userPaused bool
}

// featureRuntime 是一个功能在 engine 里的运行态。
type featureRuntime struct {
	feature  assist.Feature
	settings assist.Settings
	session  assist.Session

	state   State
	message string
	err     string

	lastState  string
	lastAction string
	lastFrame  time.Time
	// nextRun 是该功能下次最早被调用的时刻（由上一拍的 Wait 决定）。
	// 零值表示"随时可以跑"。
	nextRun time.Time
}

// New 创建 engine。features 里至少要有一个功能。
func New(device Device, input Input, screens *screen.Set, matcher vision.Matcher, logger *slog.Logger, features []FeatureSpec, cfg Config, onEvent func(Event)) *Engine {
	if cfg.TargetFPS <= 0 {
		cfg.TargetFPS = 5
	}
	if logger == nil {
		logger = slog.Default()
	}
	runtimes := make([]*featureRuntime, 0, len(features))
	for _, spec := range features {
		if spec.Feature == nil {
			continue
		}
		runtimes = append(runtimes, &featureRuntime{feature: spec.Feature, settings: spec.Settings, state: StateIdle})
	}
	return &Engine{
		device:   device,
		input:    input,
		screens:  screens,
		vision:   matcher,
		logger:   logger,
		features: runtimes,
		cfg:      cfg,
		onEvent:  onEvent,
		state:    StateIdle,
	}
}

// Status 返回当前状态快照。
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	status := Status{
		State:       e.state,
		StartedAt:   e.startedAt,
		LastMessage: e.lastMessage,
		LastError:   e.lastError,
		FPS:         e.fps,
		TargetFPS:   e.cfg.TargetFPS,
		TickID:      e.tickID,
		TickLag:     e.tickLag,
		TickDropped: e.tickDropped,
		Features:    make([]FeatureStatus, 0, len(e.features)),
	}
	for _, f := range e.features {
		status.Features = append(status.Features, FeatureStatus{
			ID: f.feature.ID(), Name: f.feature.Name(),
			State: f.state, Message: f.message, Error: f.err,
		})
	}
	return status
}

// Status 是状态快照。
type Status struct {
	State       State
	StartedAt   time.Time
	LastMessage string
	LastError   string
	FPS         float64
	TargetFPS   int
	// TickID 是最近一拍的序号；TickLag 是那一拍落后计划时间的量；
	// TickDropped 是因落后过多被跳过的拍数（持续增长说明拍长不够）。
	TickID      uint64
	TickLag     time.Duration
	TickDropped uint64
	Features    []FeatureStatus
}

// FeatureIDs 返回本会话的功能 ID 列表，供服务层比对配置。
func (e *Engine) FeatureIDs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, 0, len(e.features))
	for _, f := range e.features {
		out = append(out, f.feature.ID())
	}
	return out
}

// Pause 暂停会话。
func (e *Engine) Pause() {
	e.mu.Lock()
	e.userPaused = true
	changed := e.state == StateRunning
	if changed {
		e.state = StatePaused
		e.lastMessage = "已暂停"
	}
	e.mu.Unlock()
	if changed {
		e.stopPendingRelease()
		_ = e.input.ReleaseAll()
		e.emit("info", "已暂停")
	}
}

// Resume 恢复会话。
func (e *Engine) Resume() {
	e.mu.Lock()
	e.userPaused = false
	changed := e.state == StatePaused
	if changed {
		e.state = StateRunning
		e.lastMessage = "已恢复"
	}
	e.mu.Unlock()
	if changed {
		e.emit("info", "已恢复")
	}
}

// setFPS 更新实时帧率。
func (e *Engine) setFPS(fps float64) {
	e.mu.Lock()
	e.fps = fps
	e.mu.Unlock()
}

// markTick 记录本拍的时间轴信息。
func (e *Engine) markTick(tick Tick, wait time.Duration) {
	e.mu.Lock()
	clock := e.clock
	e.tickID = tick.ID
	e.tickLag = tick.Lag
	if clock != nil {
		e.tickDropped = clock.Dropped()
	}
	e.mu.Unlock()
	if tick.Lag > 0 {
		e.logger.Debug("节拍落后", "tick", tick.ID, "lag", tick.Lag, "wait", wait)
	}
}

// markFrame 用帧间隔的指数滑动平均更新实时帧率，避免只看第一秒的瞬间值。
func (e *Engine) markFrame(dt time.Duration) {
	if dt <= 0 {
		return
	}
	instant := 1 / dt.Seconds()
	e.mu.Lock()
	if e.fps <= 0 {
		e.fps = instant
	} else {
		e.fps = e.fps*0.8 + instant*0.2
	}
	e.mu.Unlock()
}

// Stop 请求停止会话。
func (e *Engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Run 启动会话，阻塞到 ctx 取消或全部功能失败。
// 功能会话里的 panic 也要转成 StateError 返回，不能让整个 GUI 进程退出。
func (e *Engine) Run(ctx context.Context) (runErr error) {
	defer func() {
		if r := recover(); r != nil {
			runErr = e.fail(fmt.Errorf("会话异常退出: %v\n%s", r, debug.Stack()))
		}
	}()
	e.mu.Lock()
	if e.state == StateRunning || e.state == StatePaused {
		e.mu.Unlock()
		return errors.New("会话已在运行")
	}
	runCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.state = StateRunning
	e.startedAt = time.Now()
	e.lastError = ""
	e.lastMessage = "会话已启动"
	e.userPaused = false
	e.gatePaused = false
	e.mu.Unlock()
	defer func() {
		cancel()
		e.setFPS(0)
		e.mu.Lock()
		e.cancel = nil
		for _, f := range e.features {
			if f.session != nil {
				_ = f.session.Close()
				f.session = nil
			}
		}
		if e.state != StateError {
			e.state = StateStopped
		}
		e.mu.Unlock()
		e.stopPendingRelease()
		_ = e.input.ReleaseAll()
		if closer, ok := e.input.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		if closer, ok := e.device.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	var rt assist.Runtime = &runtime{screens: e.screens, vision: e.vision, logger: e.logger}
	var asyncRT *assist.AsyncRuntime
	if e.cfg.AsyncOCRWorkers > 1 {
		asyncRT = assist.NewAsyncRuntime(rt, e.cfg.AsyncOCRWorkers)
		rt = asyncRT
	}
	defer func() {
		if asyncRT != nil {
			asyncRT.Close()
		}
	}()
	started := e.createSessions(rt)
	if started == 0 {
		return e.fail(errors.New("没有可运行的功能会话"))
	}

	interval := time.Second / time.Duration(e.cfg.TargetFPS)
	clock := newTickClock(interval)
	e.mu.Lock()
	e.clock = clock
	e.mu.Unlock()
	failures := 0
	lastFrameAt := time.Time{}
	for {
		if runCtx.Err() != nil {
			e.emit("info", "会话已停止")
			return nil
		}
		if !e.gateProcess() {
			// 游戏进程不在：保持暂停，等它回来。
			if !sleep(runCtx, 500*time.Millisecond) {
				return nil
			}
			continue
		}
		if e.isPaused() {
			if !sleep(runCtx, 100*time.Millisecond) {
				return nil
			}
			continue
		}
		if !e.device.IsActive() {
			if e.cfg.PauseWhenBackground {
				if !sleep(runCtx, 200*time.Millisecond) {
					return nil
				}
				continue
			}
			if e.cfg.ActivateWindow {
				if err := e.device.Activate(); err != nil {
					e.emit("warn", "激活游戏窗口失败: "+err.Error())
					if !sleep(runCtx, 500*time.Millisecond) {
						return nil
					}
					continue
				}
			}
		}
		// 本拍起点取"开始截图之前"。截图的 10~15ms 必须算进这一拍里，
		// 否则实际周期 = 截图耗时 + 间隔，比 TargetFPS 慢一截。
		tickStart := time.Now()
		img, scale, frameRetain, frameRelease, err := e.captureFrame()
		if err != nil {
			// 进程刚好退出时截图会失败，交给进程门控处理，不累计失败次数。
			if e.cfg.ProcessAlive != nil && !e.cfg.ProcessAlive() {
				failures = 0
				if !sleep(runCtx, 300*time.Millisecond) {
					return nil
				}
				continue
			}
			failures++
			e.emit("warn", fmt.Sprintf("截图失败(%d/3): %v", failures, err))
			if failures >= 3 {
				return e.fail(fmt.Errorf("连续截图失败: %w", err))
			}
			if !sleep(runCtx, 300*time.Millisecond) {
				return nil
			}
			continue
		}
		failures = 0
		// 一拍从截图前的时间起算，截图与决策的耗时都被吸收进这一拍，
		// 而不是像"干完活再睡 wait"那样累加到周期上。
		tick := clock.Begin(tickStart)
		frame := assist.Frame{Image: img, At: tick.Time, Scale: scale}
		// 帧缓冲来自池时，本拍用完释放一次；异步 worker 会各自 Retain/Release。
		frame.Retain = frameRetain
		frame.Release = frameRelease
		earliest, allWaiting := e.stepFeatures(runCtx, tick, frame)
		if runCtx.Err() != nil {
			return nil
		}
		if e.allFailed() {
			return e.fail(errors.New("全部功能已停止: " + e.failedSummary()))
		}
		// 本拍的责任已了：释放帧引用（异步 worker 自己持有的那些不受影响）。
		frame.Drop()
		now := time.Now()
		if !lastFrameAt.IsZero() {
			e.markFrame(now.Sub(lastFrameAt))
		}
		lastFrameAt = now
		// 节拍周期只由 TargetFPS 决定；功能给的 Wait 不再改周期。
		clock.Schedule(tick.Time, interval)
		next := clock.Next()
		// 空闲回退：只有"所有功能都要等到更晚"时才睡到那个时刻，
		// 否则会饿死每拍都该跑的 Wait=0 功能。
		if allWaiting && !earliest.IsZero() && earliest.After(next) {
			next = earliest
		}
		e.markTick(tick, next.Sub(tick.Time))
		if !sleepUntil(runCtx, next) {
			return nil
		}
	}
}

// createSessions 为每个功能创建会话，返回成功创建的个数。
func (e *Engine) createSessions(rt assist.Runtime) int {
	started := 0
	for _, f := range e.features {
		session, err := f.feature.NewSession(rt, f.settings)
		if err != nil {
			e.mu.Lock()
			f.state = StateError
			f.err = err.Error()
			f.message = err.Error()
			e.mu.Unlock()
			e.emit("error", fmt.Sprintf("%s 创建会话失败: %v", f.feature.Name(), err))
			continue
		}
		e.mu.Lock()
		f.session = session
		f.state = StateRunning
		f.message = "会话已启动"
		e.mu.Unlock()
		started++
		e.emit("info", "开始执行 "+f.feature.Name())
	}
	return started
}

// stepFeatures 用同一帧驱动到期功能。
//
// 语义：
//   - 节拍周期由 TargetFPS 决定，功能不能把它拉快或拉慢。
//   - 功能返回的 Wait 只表示"这个功能下次最早什么时候再叫我"，记在 nextRun 上。
//   - 返回 earliest = 所有在等的最小到期时刻；allWaiting = 是否**所有**运行中的
//     功能都还在等。只有 allWaiting 为真才能做空闲回退，否则会饿死
//     Wait=0 的功能（它每拍都该跑）。
func (e *Engine) stepFeatures(ctx context.Context, tick Tick, frame assist.Frame) (earliest time.Time, allWaiting bool) {
	allWaiting = true
	anyRunning := false
	for _, f := range e.features {
		if ctx.Err() != nil {
			return time.Time{}, false
		}
		e.mu.Lock()
		session, state, due := f.session, f.state, f.nextRun
		e.mu.Unlock()
		if session == nil || state != StateRunning {
			continue
		}
		anyRunning = true
		if !due.IsZero() && tick.Time.Before(due) {
			if earliest.IsZero() || due.Before(earliest) {
				earliest = due
			}
			continue
		}
		result, actions, err := session.Step(ctx, frame)
		if err != nil {
			e.mu.Lock()
			f.state = StateError
			f.err = err.Error()
			f.message = err.Error()
			e.mu.Unlock()
			e.emit("error", fmt.Sprintf("%s 执行失败: %v", f.feature.Name(), err))
			allWaiting = false
			continue
		}
		if result.Message != "" {
			e.mu.Lock()
			f.message = result.Message
			e.mu.Unlock()
			e.emit("info", result.Message)
		}
		e.logFeatureStep(f, tick, result.State, actions)
		if err := e.executeActions(ctx, actions); err != nil {
			e.mu.Lock()
			f.state = StateError
			f.err = err.Error()
			e.mu.Unlock()
			e.emit("error", fmt.Sprintf("%s 动作执行失败: %v", f.feature.Name(), err))
			allWaiting = false
			continue
		}
		next := time.Time{}
		if result.Wait > 0 {
			next = tick.Time.Add(result.Wait)
			if earliest.IsZero() || next.Before(earliest) {
				earliest = next
			}
		} else {
			// Wait<=0 表示这个功能下一拍就要再跑，不能进入空闲回退。
			allWaiting = false
		}
		e.mu.Lock()
		f.nextRun = next
		e.mu.Unlock()
	}
	if !anyRunning {
		allWaiting = false
	}
	return earliest, allWaiting
}

// allFailed 报告是否全部功能都因错误退出。
func (e *Engine) allFailed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.features) == 0 {
		return false
	}
	for _, f := range e.features {
		if f.state != StateError {
			return false
		}
	}
	return true
}

func (e *Engine) failedSummary() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var parts []string
	for _, f := range e.features {
		if f.err != "" {
			parts = append(parts, f.feature.Name()+": "+f.err)
		}
	}
	return strings.Join(parts, "；")
}

// captureFrame 截取一帧。实现了 FrameDevice 的设备会返回带引用计数的帧缓冲，
// 这样异步识别 worker 可以直接读原像素，不必为每帧深拷贝约 8MB。
// 返回值里的 retain/release 为 nil 表示帧不会被复用，无需引用计数。
func (e *Engine) captureFrame() (img *image.RGBA, scale float64, retain, release func(), err error) {
	if fd, ok := e.device.(FrameDevice); ok {
		frame, err := fd.CaptureFrame()
		if err != nil {
			return nil, 0, nil, nil, err
		}
		return frame.Image, frame.Scale, frame.Retain, frame.Release, nil
	}
	img, scale, err = e.device.Capture()
	return img, scale, nil, nil, err
}

// gateProcess 处理游戏进程门控：进程不在就自动暂停，进程回来再自动恢复。
// 返回 false 表示当前应当暂停；用户自己按的暂停不会因为进程恢复而被取消。
func (e *Engine) gateProcess() bool {
	if e.cfg.ProcessAlive == nil {
		return true
	}
	alive := e.cfg.ProcessAlive()
	var paused, resumed bool
	e.mu.Lock()
	switch {
	case !alive && !e.gatePaused:
		e.gatePaused = true
		if e.state == StateRunning {
			e.state = StatePaused
			e.lastMessage = "游戏进程已退出，已暂停"
			paused = true
		}
	case alive && e.gatePaused:
		e.gatePaused = false
		if !e.userPaused {
			e.state = StateRunning
			e.lastMessage = "游戏进程已恢复，已继续"
			resumed = true
		}
	}
	e.mu.Unlock()
	if paused {
		e.stopPendingRelease()
		_ = e.input.ReleaseAll()
		e.emit("info", "游戏进程已退出，已暂停")
	}
	if resumed {
		e.emit("info", "游戏进程已恢复，已继续")
	}
	return alive
}

// logFeatureStep 记录功能的状态切换和动作变化；重复的同一动作只写文件日志，避免刷屏。
func (e *Engine) logFeatureStep(f *featureRuntime, tick Tick, state string, actions []assist.Action) {
	action := actionSummary(actions)
	e.mu.Lock()
	changed := state != f.lastState || action != f.lastAction
	f.lastState, f.lastAction = state, action
	e.mu.Unlock()
	if !changed {
		if action != "" {
			// 带 tick：定位"第几拍出了这个动作"，重复动作刷屏时也能对上时间轴。
			e.logger.Debug("重复动作", "tick", tick.ID, "feature", f.feature.ID(), "state", state, "action", action)
		}
		return
	}
	e.logger.Debug("状态切换", "tick", tick.ID, "lag", tick.Lag, "feature", f.feature.ID(), "state", state, "action", action)
	// 前缀用功能自己的名字。原来固定写"剧情流程"，自动战斗的状态也被打成
	// "剧情流程: 战斗前切治疗"，日志根本没法读。
	message := f.feature.Name() + ": " + state
	if action != "" {
		message += " | " + action
	}
	e.emit("info", message)
}

func actionSummary(actions []assist.Action) string {
	if len(actions) == 0 {
		return ""
	}
	var parts []string
	for _, action := range actions {
		switch action.Kind {
		case assist.ActionClick:
			parts = append(parts, fmt.Sprintf("点击(%d,%d)", action.Point.X, action.Point.Y))
		case assist.ActionMiddleClick:
			parts = append(parts, fmt.Sprintf("中键(%d,%d)", action.Point.X, action.Point.Y))
		case assist.ActionClickCurrent:
			parts = append(parts, "点击当前光标")
		case assist.ActionClickHold:
			parts = append(parts, fmt.Sprintf("按住(%d,%d) %s", action.Point.X, action.Point.Y, action.Duration))
		case assist.ActionHold:
			parts = append(parts, fmt.Sprintf("按住(%d,%d)", action.Point.X, action.Point.Y))
		case assist.ActionRelease:
			parts = append(parts, "抬起")
		case assist.ActionKeyTap:
			parts = append(parts, "按键 "+action.Key)
		case assist.ActionWait:
			parts = append(parts, fmt.Sprintf("等待 %s", action.Duration))
		}
	}
	return strings.Join(parts, " + ")
}

// isHolding 报告当前是否处于异步按住状态。
func (e *Engine) isHolding() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.releaseTimer != nil
}

// scheduleRelease 在 hold 之后抬起左键，不阻塞调用方。
func (e *Engine) scheduleRelease(hold time.Duration) {
	e.mu.Lock()
	if e.releaseTimer != nil {
		e.releaseTimer.Stop()
	}
	e.releaseTimer = time.AfterFunc(hold, func() {
		e.mu.Lock()
		e.releaseTimer = nil
		e.mu.Unlock()
		e.actionMu.Lock()
		defer e.actionMu.Unlock()
		if err := e.input.Release(); err != nil {
			e.emit("warn", "抬起左键失败: "+err.Error())
		}
	})
	e.mu.Unlock()
}

// stopPendingRelease 取消尚未触发的抬起并立即抬起，供停止/暂停使用。
// 与 executeActions 互斥：否则暂停的抬起可能和某一拍的 Press 交错。
func (e *Engine) stopPendingRelease() {
	e.actionMu.Lock()
	defer e.actionMu.Unlock()
	e.mu.Lock()
	timer := e.releaseTimer
	e.releaseTimer = nil
	e.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	// 连招里每拍都会重新按住，抬起定时器一直被推后；
	// 停止/暂停时必须主动抬起，否则键会一直按着。
	if err := e.input.Release(); err != nil {
		e.emit("warn", "抬起左键失败: "+err.Error())
	}
}

func (e *Engine) executeActions(ctx context.Context, actions []assist.Action) error {
	// 与 Pause/Stop 的抬起互斥：拿到锁后再确认一次没被暂停，
	// 避免"暂停已经抬起、这一拍又把键按下去"。
	e.actionMu.Lock()
	defer e.actionMu.Unlock()
	if e.isPaused() {
		return nil
	}
	for _, action := range actions {
		if ctx.Err() != nil {
			return nil
		}
		switch action.Kind {
		case assist.ActionClick:
			if err := e.input.Click(action.Point); err != nil {
				return fmt.Errorf("点击失败: %w", err)
			}
		case assist.ActionMiddleClick:
			if err := e.input.MiddleClick(action.Point); err != nil {
				return fmt.Errorf("中键点击失败: %w", err)
			}
		case assist.ActionClickCurrent:
			if err := e.input.ClickCurrent(); err != nil {
				return fmt.Errorf("当前光标点击失败: %w", err)
			}
		case assist.ActionClickHold:
			// 按住时长超过一拍就拆成 Press/Release：600ms 的重击如果同步阻塞，
			// 引擎在按住期间不截图、不决策，等于失明半秒（实测 2.8 秒只推进 5 帧）。
			if action.Duration > maxBlockingHold {
				// 已经按住了就不要再按：连招会连着几拍都要求重击，
				// 每次都重新按下会把抬起时间无限推后，键就一直按着。
				// 只在空闲时按下，抬起由第一次按下时定的定时器负责。
				if !e.isHolding() {
					if err := e.input.Press(action.Point); err != nil {
						return fmt.Errorf("按住失败: %w", err)
					}
					e.scheduleRelease(action.Duration)
				}
				continue
			}
			if err := e.input.ClickHold(action.Point, action.Duration); err != nil {
				return fmt.Errorf("按住点击失败: %w", err)
			}
		case assist.ActionHold:
			if err := e.input.Press(action.Point); err != nil {
				return fmt.Errorf("按住失败: %w", err)
			}
		case assist.ActionRelease:
			if err := e.input.Release(); err != nil {
				return fmt.Errorf("抬起失败: %w", err)
			}
		case assist.ActionKeyTap:
			if err := e.input.KeyTap(action.Key); err != nil {
				return fmt.Errorf("按键失败: %w", err)
			}
		case assist.ActionWait:
			if !sleep(ctx, action.Duration) {
				return nil
			}
		}
	}
	return nil
}

func (e *Engine) isPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state == StatePaused
}

func (e *Engine) fail(err error) error {
	e.mu.Lock()
	e.state = StateError
	e.lastError = err.Error()
	e.lastMessage = err.Error()
	e.mu.Unlock()
	e.emit("error", err.Error())
	return err
}

func (e *Engine) emit(level, message string) {
	event := Event{Time: time.Now(), Level: level, State: e.Status().State, Message: message}
	if e.onEvent != nil {
		e.onEvent(event)
	}
	if level == "error" {
		e.logger.Error(message)
	} else if level == "warn" {
		e.logger.Warn(message)
	} else {
		e.logger.Info(message)
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type runtime struct {
	screens *screen.Set
	vision  vision.Matcher
	logger  *slog.Logger
}

func (r *runtime) Screen(name string) (*screen.Screen, error) {
	sc, ok := r.screens.Get(name)
	if !ok {
		return nil, fmt.Errorf("未找到画面 %q", name)
	}
	return sc, nil
}

func (r *runtime) OCR(frame assist.Frame, area screen.Area, opts vision.OCROptions) ([]vision.TextBlock, error) {
	if frame.Image == nil {
		return nil, errors.New("空帧")
	}
	return r.vision.OCR(frame.Image, area, opts)
}

func (r *runtime) MatchTemplate(frame assist.Frame, area screen.Area, ref vision.TemplateRef, threshold float64) ([]vision.Match, error) {
	if frame.Image == nil {
		return nil, errors.New("空帧")
	}
	return r.vision.Template(frame.Image, area, ref, threshold)
}

func (r *runtime) ColorStats(frame assist.Frame, area screen.Area) (vision.ColorStats, error) {
	if frame.Image == nil {
		return vision.ColorStats{}, errors.New("空帧")
	}
	return r.vision.Color(frame.Image, area)
}

func (r *runtime) Log() *slog.Logger { return r.logger }
