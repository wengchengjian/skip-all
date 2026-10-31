// Package engine 负责把游戏截图、Feature 状态机和输入执行串起来。
package engine

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
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
	ClickCurrent() error
	KeyTap(string) error
	ReleaseAll() error
}

// Config 是 engine 的运行参数。
type Config struct {
	TargetFPS           int
	PauseWhenBackground bool
	ActivateWindow      bool
}

// Engine 管理一次 Feature 会话。
type Engine struct {
	mu       sync.Mutex
	device   Device
	input    Input
	screens  *screen.Set
	vision   vision.Matcher
	logger   *slog.Logger
	feature  assist.Feature
	settings assist.Settings
	cfg      Config
	onEvent  func(Event)

	state       State
	lastMessage string
	lastError   string
	startedAt   time.Time
	cancel      context.CancelFunc
}

// New 创建 engine。
func New(device Device, input Input, screens *screen.Set, matcher vision.Matcher, logger *slog.Logger, feature assist.Feature, settings assist.Settings, cfg Config, onEvent func(Event)) *Engine {
	if cfg.TargetFPS <= 0 {
		cfg.TargetFPS = 5
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		device:   device,
		input:    input,
		screens:  screens,
		vision:   matcher,
		logger:   logger,
		feature:  feature,
		settings: settings,
		cfg:      cfg,
		onEvent:  onEvent,
		state:    StateIdle,
	}
}

// Status 返回当前状态快照。
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{
		State:       e.state,
		FeatureID:   e.feature.ID(),
		FeatureName: e.feature.Name(),
		StartedAt:   e.startedAt,
		LastMessage: e.lastMessage,
		LastError:   e.lastError,
	}
}

// Status 是状态快照。
type Status struct {
	State       State
	FeatureID   string
	FeatureName string
	StartedAt   time.Time
	LastMessage string
	LastError   string
}

// Pause 暂停会话。
func (e *Engine) Pause() {
	e.mu.Lock()
	if e.state == StateRunning {
		e.state = StatePaused
		e.lastMessage = "已暂停"
	}
	e.mu.Unlock()
	e.emit("info", "已暂停")
}

// Resume 恢复会话。
func (e *Engine) Resume() {
	e.mu.Lock()
	if e.state == StatePaused {
		e.state = StateRunning
		e.lastMessage = "已恢复"
	}
	e.mu.Unlock()
	e.emit("info", "已恢复")
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

// Run 启动会话，阻塞到 ctx 取消或发生错误。
func (e *Engine) Run(ctx context.Context) error {
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
	e.mu.Unlock()
	defer func() {
		cancel()
		e.mu.Lock()
		e.cancel = nil
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

	session, err := e.feature.NewSession(&runtime{
		screens: e.screens,
		vision:  e.vision,
		logger:  e.logger,
	}, e.settings)
	if err != nil {
		return e.fail(fmt.Errorf("创建功能会话: %w", err))
	}
	defer session.Close()
	e.emit("info", "开始执行 "+e.feature.Name())

	interval := time.Second / time.Duration(e.cfg.TargetFPS)
	failures := 0
	lastState := ""
	lastAction := ""
	for {
		if runCtx.Err() != nil {
			e.emit("info", "会话已停止")
			return nil
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
		frame := assist.Frame{Image: img, At: time.Now(), Scale: scale}
		result, actions, err := session.Step(runCtx, frame)
		if err != nil {
			return e.fail(fmt.Errorf("执行功能节点: %w", err))
		}
		if result.Message != "" {
			e.setMessage(result.Message)
		}
		lastState, lastAction = e.logStep(lastState, lastAction, result.State, actions)
		if err := e.executeActions(runCtx, actions); err != nil {
			return e.fail(err)
		}
		wait := result.Wait
		if wait <= 0 {
			wait = interval
		}
		if !sleep(runCtx, wait) {
			return nil
		}
	}
}

// logStep 记录状态切换和动作变化；重复的同一动作只写文件日志，避免刷屏。
func (e *Engine) logStep(lastState, lastAction, state string, actions []assist.Action) (string, string) {
	action := actionSummary(actions)
	if state != lastState || action != lastAction {
		message := "剧情流程: " + state
		if action != "" {
			message += " | " + action
		}
		e.emit("info", message)
		return state, action
	}
	if action != "" {
		e.logger.Debug("重复动作", "state", state, "action", action)
	}
	return lastState, lastAction
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
		case assist.ActionClickCurrent:
			parts = append(parts, "点击当前光标")
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
		case assist.ActionClickCurrent:
			if err := e.input.ClickCurrent(); err != nil {
				return fmt.Errorf("当前光标点击失败: %w", err)
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

func (e *Engine) setMessage(msg string) {
	e.mu.Lock()
	e.lastMessage = msg
	e.mu.Unlock()
	e.emit("info", msg)
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
