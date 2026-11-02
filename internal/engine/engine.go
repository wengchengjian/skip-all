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

// Input 抽象点击与按键，便于测试替换。
type Input interface {
	Click(geom.Point) error
	MiddleClick(geom.Point) error
	ClickHold(geom.Point, time.Duration) error
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
	tickID  uint64
	tickLag time.Duration
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
	// TickID 是最近一拍的序号；TickLag 是那一拍落后计划时间的量。
	TickID   uint64
	TickLag  time.Duration
	Features []FeatureStatus
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
	e.tickID = tick.ID
	e.tickLag = tick.Lag
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
		_ = e.input.ReleaseAll()
		if closer, ok := e.input.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		if closer, ok := e.device.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	rt := &runtime{screens: e.screens, vision: e.vision, logger: e.logger}
	started := e.createSessions(rt)
	if started == 0 {
		return e.fail(errors.New("没有可运行的功能会话"))
	}

	interval := time.Second / time.Duration(e.cfg.TargetFPS)
	clock := newTickClock(interval)
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
		img, scale, err := e.device.Capture()
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
		// 一拍从本拍开始时间起算，截图与决策的耗时被吸收进这一拍，
		// 而不是像"干完活再睡 wait"那样累加到周期上。
		tick := clock.Begin(time.Now())
		frame := assist.Frame{Image: img, At: tick.Time, Scale: scale}
		wait := e.stepFeatures(runCtx, tick, frame)
		if runCtx.Err() != nil {
			return nil
		}
		if e.allFailed() {
			return e.fail(errors.New("全部功能已停止: " + e.failedSummary()))
		}
		now := time.Now()
		if !lastFrameAt.IsZero() {
			e.markFrame(now.Sub(lastFrameAt))
		}
		lastFrameAt = now
		clock.Schedule(tick.Time, wait)
		e.markTick(tick, wait)
		if !sleepUntil(runCtx, clock.next) {
			return nil
		}
	}
}

// createSessions 为每个功能创建会话，返回成功创建的个数。
func (e *Engine) createSessions(rt *runtime) int {
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

// stepFeatures 用同一帧驱动全部功能，返回本帧建议的等待时长（取各功能的最小值）。
func (e *Engine) stepFeatures(ctx context.Context, tick Tick, frame assist.Frame) time.Duration {
	wait := time.Duration(0)
	for _, f := range e.features {
		if ctx.Err() != nil {
			return 0
		}
		e.mu.Lock()
		session, state := f.session, f.state
		e.mu.Unlock()
		if session == nil || state != StateRunning {
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
			continue
		}
		if result.Wait > 0 && (wait <= 0 || result.Wait < wait) {
			wait = result.Wait
		}
	}
	return wait
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
			e.logger.Debug("重复动作", "feature", f.feature.ID(), "state", state, "action", action)
		}
		return
	}
	message := "剧情流程: " + state
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
		case assist.ActionKeyTap:
			parts = append(parts, "按键 "+action.Key)
		case assist.ActionWait:
			parts = append(parts, fmt.Sprintf("等待 %s", action.Duration))
		}
	}
	return strings.Join(parts, " + ")
}

func (e *Engine) executeActions(ctx context.Context, actions []assist.Action) error {
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
			if err := e.input.ClickHold(action.Point, action.Duration); err != nil {
				return fmt.Errorf("按住点击失败: %w", err)
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
