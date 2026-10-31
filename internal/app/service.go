// Package app 是 GUI/CLI 共用的应用服务层。
package app

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wengchengjian/skip-all/games"
	"github.com/wengchengjian/skip-all/internal/engine"
	"github.com/wengchengjian/skip-all/internal/model"
	"github.com/wengchengjian/skip-all/internal/platform"
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/config"
	"github.com/wengchengjian/skip-all/pkg/screen"
	"github.com/wengchengjian/skip-all/pkg/vision"
	"gopkg.in/natefinch/lumberjack.v2"
)

// EventSink 把服务状态推给 Wails 或 CLI。
type EventSink func(name string, data any)

// Options 是服务启动参数。
type Options struct {
	DataDir   string
	AssetsDir string
	EventSink EventSink
}

// Service 是应用服务。
type Service struct {
	mu      sync.Mutex
	dataDir string
	assets  string
	logger  *slog.Logger
	logFile *lumberjack.Logger
	sink    EventSink

	appCfg config.AppConfig
	zzzCfg config.ZZZConfig
	eng    *engine.Engine
	cancel context.CancelFunc
	ocr    vision.OCR
	gameID string

	logs []LogEntry
}

// LogEntry 是 UI 显示的一条日志。
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// GameView 是游戏列表项。
type GameView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Features []FeatureView `json:"features"`
}

// FeatureView 是功能列表项。
type FeatureView struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Defaults    map[string]any `json:"defaults"`
}

// ConfigView 是应用与游戏配置快照。
type ConfigView struct {
	App config.AppConfig `json:"app"`
	ZZZ config.ZZZConfig `json:"zzz"`
}

// StatusView 是当前运行状态。
type StatusView struct {
	State       string    `json:"state"`
	Running     bool      `json:"running"`
	Paused      bool      `json:"paused"`
	GameID      string    `json:"gameId"`
	FeatureID   string    `json:"featureId"`
	FeatureName string    `json:"featureName"`
	Message     string    `json:"message"`
	Error       string    `json:"error"`
	StartedAt   time.Time `json:"startedAt"`
}

// ActionResult 是启动/停止类操作的结果。
type ActionResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// DoctorReport 是 doctor 命令的输出。
type DoctorReport struct {
	WindowTitle   string   `json:"windowTitle"`
	ProcessName   string   `json:"processName"`
	CaptureWidth  int      `json:"captureWidth"`
	CaptureHeight int      `json:"captureHeight"`
	Scale         float64  `json:"scale"`
	FramePath     string   `json:"framePath"`
	OCRReady      bool     `json:"ocrReady"`
	OCRError      string   `json:"ocrError"`
	OCRTexts      []string `json:"ocrTexts"`
}

// NewService 创建应用服务并加载配置/日志。
func NewService(opts Options) (*Service, error) {
	dataDir := opts.DataDir
	if dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("获取配置目录: %w", err)
		}
		dataDir = filepath.Join(base, "skip-all")
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	appCfg, err := config.LoadApp(filepath.Join(dataDir, "config.yaml"))
	if err != nil {
		return nil, err
	}
	zzzCfg, err := config.LoadZZZ(filepath.Join(dataDir, "games", "zzz.yaml"))
	if err != nil {
		return nil, err
	}
	if err := config.SaveApp(filepath.Join(dataDir, "config.yaml"), appCfg); err != nil {
		return nil, err
	}
	if err := config.SaveZZZ(filepath.Join(dataDir, "games", "zzz.yaml"), zzzCfg); err != nil {
		return nil, err
	}

	logFile := &lumberjack.Logger{Filename: filepath.Join(dataDir, "logs", "skip-all.log"), MaxSize: 10, MaxBackups: 7, MaxAge: 30, Compress: true}
	level := slog.LevelInfo
	switch strings.ToLower(appCfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, logFile), &slog.HandlerOptions{Level: level}))
	assets := opts.AssetsDir
	if assets == "" {
		if exe, e := os.Executable(); e == nil {
			candidate := filepath.Join(filepath.Dir(exe), "assets")
			if _, statErr := os.Stat(candidate); statErr == nil {
				assets = candidate
			}
		}
	}
	if assets == "" {
		if wd, e := os.Getwd(); e == nil {
			assets = filepath.Join(wd, "assets")
		}
	}
	if _, statErr := os.Stat(assets); statErr != nil {
		assets = ""
	}
	svc := &Service{dataDir: dataDir, assets: assets, logger: logger, logFile: logFile, sink: opts.EventSink, appCfg: appCfg, zzzCfg: zzzCfg}
	svc.appendLog("info", "skip-all 服务已初始化")
	return svc, nil
}

// Shutdown 停止会话并释放资源。
func (s *Service) Shutdown() {
	s.Stop()
	s.mu.Lock()
	if s.ocr != nil {
		_ = s.ocr.Close()
		s.ocr = nil
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
	}
	s.mu.Unlock()
}

// ListGames 返回内置游戏与功能。
func (s *Service) ListGames() []GameView {
	var out []GameView
	for _, g := range games.All() {
		gv := GameView{ID: g.ID(), Name: g.Name()}
		for _, f := range g.Features() {
			gv.Features = append(gv.Features, FeatureView{ID: f.ID(), Name: f.Name(), Description: f.Description(), Defaults: map[string]any(f.DefaultSettings())})
		}
		out = append(out, gv)
	}
	return out
}

// GetConfig 返回当前配置。
func (s *Service) GetConfig() ConfigView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ConfigView{App: s.appCfg, ZZZ: s.zzzCfg}
}

// SaveConfig 保存配置并更新内存快照（下一次启动生效）。
func (s *Service) SaveConfig(view ConfigView) error {
	if err := config.SaveApp(filepath.Join(s.dataDir, "config.yaml"), view.App); err != nil {
		return err
	}
	if err := config.SaveZZZ(filepath.Join(s.dataDir, "games", "zzz.yaml"), view.ZZZ); err != nil {
		return err
	}
	s.mu.Lock()
	s.appCfg = view.App
	s.zzzCfg = view.ZZZ
	s.mu.Unlock()
	return nil
}

// Start 启动指定游戏的功能。
func (s *Service) Start(gameID, featureID string) ActionResult {
	s.mu.Lock()
	if s.eng != nil {
		st := s.eng.Status()
		if st.State == engine.StateRunning || st.State == engine.StatePaused {
			s.mu.Unlock()
			return ActionResult{OK: false, Error: "已有会话在运行"}
		}
	}
	s.mu.Unlock()

	game, ok := games.Get(gameID)
	if !ok {
		return ActionResult{OK: false, Error: "未知游戏: " + gameID}
	}
	var feature assist.Feature
	for _, f := range game.Features() {
		if f.ID() == featureID {
			feature = f
			break
		}
	}
	if feature == nil {
		return ActionResult{OK: false, Error: "未知功能: " + featureID}
	}

	settings := feature.DefaultSettings()
	s.mu.Lock()
	if fc, ok := s.zzzCfg.Features[featureID]; ok {
		if !fc.Enabled {
			s.mu.Unlock()
			return ActionResult{OK: false, Error: "功能已在配置中禁用"}
		}
		if fc.Settings != nil {
			settings = fc.Settings
		}
	}
	appCfg := s.appCfg
	s.mu.Unlock()
	if err := feature.Validate(settings); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}

	win, err := platform.Find(game.WindowTitles(), game.ProcessNames())
	if err != nil {
		return ActionResult{OK: false, Error: "找不到游戏窗口: " + err.Error()}
	}
	capturer := platform.NewCapturer(win)
	background := strings.ToLower(appCfg.Input.Mode) != "foreground"
	input := platform.NewInputWithOptions(win, platform.InputOptions{Background: background, Gamepad: appCfg.Input.Gamepad, GamepadDLL: s.gamepadDLL()})
	if appCfg.Input.Gamepad && !input.GamepadAvailable() {
		s.appendLog("warn", "虚拟手柄不可用: "+fmt.Sprint(input.GamepadError()))
	}
	screens, err := screen.LoadFS(game.Assets(), "screens")
	if err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	ocr, err := s.ensureOCR()
	if err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	matcher := vision.NewMatcher(ocr, vision.NewTemplateStoreFS(game.Assets(), "templates"))
	eng := engine.New(capturer, input, screens, matcher, s.logger, feature, assist.Settings(settings), engine.Config{TargetFPS: appCfg.Capture.TargetFPS, PauseWhenBackground: appCfg.Input.PauseWhenBackground, ActivateWindow: !background}, func(ev engine.Event) { s.onEngineEvent(ev) })
	s.mu.Lock()
	s.eng = eng
	s.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.gameID = gameID
	s.mu.Unlock()
	go func() {
		err := eng.Run(ctx)
		if err != nil {
			s.appendLog("error", err.Error())
		}
		s.emit("skipall:status", s.GetStatus())
	}()
	return ActionResult{OK: true}
}

// Stop 停止当前会话。
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if s.eng != nil {
		s.eng.Stop()
	}
}

// Pause 暂停当前会话。
func (s *Service) Pause() {
	if s.eng != nil {
		s.eng.Pause()
		s.emit("skipall:status", s.GetStatus())
	}
}

// Resume 恢复当前会话。
func (s *Service) Resume() {
	if s.eng != nil {
		s.eng.Resume()
		s.emit("skipall:status", s.GetStatus())
	}
}

// GetStatus 返回当前状态。
func (s *Service) GetStatus() StatusView {
	s.mu.Lock()
	eng := s.eng
	gameID := s.gameID
	s.mu.Unlock()
	if eng == nil {
		return StatusView{State: string(engine.StateIdle), GameID: gameID}
	}
	st := eng.Status()
	return StatusView{State: string(st.State), Running: st.State == engine.StateRunning, Paused: st.State == engine.StatePaused, GameID: gameID, FeatureID: st.FeatureID, FeatureName: st.FeatureName, Message: st.LastMessage, Error: st.LastError, StartedAt: st.StartedAt}
}

// GetLogs 返回最近日志。
func (s *Service) GetLogs() []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]LogEntry, len(s.logs))
	copy(out, s.logs)
	return out
}

// EnsureModels 检查并可选安装 OCR 模型。
func (s *Service) EnsureModels() error {
	dir, err := s.modelDir()
	if err != nil {
		return err
	}
	return model.CheckOCR(dir)
}

// InstallModels 下载并安装 OCR 模型。
func (s *Service) InstallModels() error {
	dir, err := s.modelDir()
	if err != nil {
		return err
	}
	return model.InstallOCR(context.Background(), dir, func(percent float64, message string) {
		s.emit("skipall:model", map[string]any{"percent": percent, "message": message})
	})
}

// Doctor 检查窗口、截图与 OCR 状态，并保存一帧调试图。
func (s *Service) Doctor(gameID string) (DoctorReport, error) {
	game, ok := games.Get(gameID)
	if !ok {
		return DoctorReport{}, fmt.Errorf("未知游戏: %s", gameID)
	}
	win, err := platform.Find(game.WindowTitles(), game.ProcessNames())
	if err != nil {
		return DoctorReport{}, fmt.Errorf("找不到游戏窗口: %w", err)
	}
	capturer := platform.NewCapturer(win)
	defer capturer.Close()
	img, scale, err := capturer.Capture()
	if err != nil {
		return DoctorReport{}, fmt.Errorf("截图失败: %w", err)
	}
	frameDir := filepath.Join(s.dataDir, "debug")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		return DoctorReport{}, err
	}
	framePath := filepath.Join(frameDir, "doctor-"+time.Now().Format("20060102-150405")+".png")
	f, err := os.Create(framePath)
	if err != nil {
		return DoctorReport{}, err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return DoctorReport{}, err
	}
	_ = f.Close()
	report := DoctorReport{WindowTitle: win.Title(), ProcessName: win.ProcessName(), CaptureWidth: img.Bounds().Dx(), CaptureHeight: img.Bounds().Dy(), Scale: scale, FramePath: framePath}
	screens, err := screen.LoadFS(game.Assets(), "screens")
	if err != nil {
		report.OCRError = err.Error()
		return report, nil
	}
	ocr, err := s.ensureOCR()
	if err != nil {
		report.OCRError = err.Error()
		return report, nil
	}
	matcher := vision.NewMatcher(ocr, vision.NewTemplateStoreFS(game.Assets(), "templates"))
	if sc, ok := screens.Get("委托助手"); ok {
		if area, ok := sc.Area("文本-剧情右上角"); ok {
			blocks, err := matcher.OCR(img, *area, vision.OCROptions{})
			if err != nil {
				report.OCRError = err.Error()
			} else {
				report.OCRReady = true
				for _, b := range blocks {
					report.OCRTexts = append(report.OCRTexts, b.Text)
				}
			}
		}
	}
	return report, nil
}

func (s *Service) ensureOCR() (vision.OCR, error) {
	s.mu.Lock()
	if s.ocr != nil {
		defer s.mu.Unlock()
		return s.ocr, nil
	}
	dir, err := s.modelDirLocked()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	ocr, err := vision.NewPaddleOCR(dir)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ocr = ocr
	s.mu.Unlock()
	return ocr, nil
}

func (s *Service) gamepadDLL() string {
	var candidates []string
	if s.assets != "" {
		candidates = append(candidates, filepath.Join(s.assets, "runtime", "ViGEmClient.dll"))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "runtime", "ViGEmClient.dll"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (s *Service) modelDir() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.modelDirLocked()
}

func (s *Service) modelDirLocked() (string, error) {
	if s.appCfg.OCR.ModelDir != "" {
		return s.appCfg.OCR.ModelDir, nil
	}
	var candidates []string
	if s.assets != "" {
		candidates = append(candidates, filepath.Join(s.assets, "models", "ppocrv6"))
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(base, "models", "ppocrv6"), filepath.Join(base, "assets", "models", "ppocrv6"))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "det.onnx")); err == nil {
			return candidate, nil
		}
	}
	return model.DefaultDir()
}

func (s *Service) onEngineEvent(ev engine.Event) {
	s.appendLog(ev.Level, ev.Message)
	s.emit("skipall:log", LogEntry{Time: ev.Time, Level: ev.Level, Message: ev.Message})
	s.emit("skipall:status", s.GetStatus())
}

func (s *Service) appendLog(level, message string) {
	entry := LogEntry{Time: time.Now(), Level: level, Message: message}
	s.mu.Lock()
	s.logs = append(s.logs, entry)
	if len(s.logs) > 500 {
		s.logs = s.logs[len(s.logs)-500:]
	}
	s.mu.Unlock()
	if s.logger != nil {
		s.logger.Info(message, "level", level)
	}
}

func (s *Service) emit(name string, data any) {
	if s.sink != nil {
		s.sink(name, data)
	}
}

// GameIDs 返回内置游戏 ID，供 CLI 参数校验。
func GameIDs() []string {
	all := games.All()
	out := make([]string, 0, len(all))
	for _, g := range all {
		out = append(out, g.ID())
	}
	return out
}
