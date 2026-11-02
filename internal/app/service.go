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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wengchengjian/skip-all/games"
	"github.com/wengchengjian/skip-all/internal/engine"
	"github.com/wengchengjian/skip-all/internal/model"
	"github.com/wengchengjian/skip-all/internal/platform"
	"github.com/wengchengjian/skip-all/internal/update"
	"github.com/wengchengjian/skip-all/pkg/assist"
	"github.com/wengchengjian/skip-all/pkg/assist/storyskip"
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
	Quit      func()
}

// Service 是应用服务。
type Service struct {
	mu      sync.Mutex
	dataDir string
	assets  string
	logger  *slog.Logger
	logFile *lumberjack.Logger
	sink    EventSink

	appCfg     config.AppConfig
	gameCfg    map[string]config.GameConfig
	ocr        vision.OCR
	ocrBackend string

	updateChecker    *update.Checker
	updateInfo       update.Info
	updateDownloadMu sync.Mutex
	quit             func()

	sessions  map[string]*gameSession
	watchStop chan struct{}
	waiting   map[string]bool
	autoStop  bool

	logs []LogEntry
}

// gameSession 是一个游戏当前的 engine 会话；一个游戏可同时运行多个功能。
type gameSession struct {
	gameID     string
	featureIDs []string
	eng        *engine.Engine
	cancel     context.CancelFunc
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
	App   config.AppConfig             `json:"app"`
	Games map[string]config.GameConfig `json:"games"`
}

// GameStateView 是托盘里一个游戏的进程与会话状态。
type GameStateView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ProcessRunning bool   `json:"processRunning"`
	State          string `json:"state"`
	Message        string `json:"message"`
}

// FeatureToggleView 是一个功能在全部游戏上的启用状态；全部启用才算启用。
type FeatureToggleView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// TrayStateView 是托盘菜单与提示需要的一次性快照。
type TrayStateView struct {
	Games    []GameStateView     `json:"games"`
	Features []FeatureToggleView `json:"features"`
	Status   StatusView          `json:"status"`
	CPU      float64             `json:"cpu"`
	MemoryMB float64             `json:"memoryMb"`
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
	FPS         float64   `json:"fps"`
	TargetFPS   int       `json:"targetFps"`
	// TickID/TickLagMs/TickDropped 是节拍观测：ID 单调递增，
	// Lag 是最近一拍落后计划时间的毫秒数，Dropped 是跳过的拍数。
	TickID      uint64             `json:"tickId"`
	TickLagMs   float64            `json:"tickLagMs"`
	TickDropped uint64             `json:"tickDropped"`
	OCRBackend  string             `json:"ocrBackend"`
	Features    []FeatureStateView `json:"features"`
}

// FeatureStateView 是单个功能在某个游戏上的运行状态。
type FeatureStateView struct {
	GameID    string `json:"gameId"`
	FeatureID string `json:"featureId"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Message   string `json:"message"`
}

// ActionResult 是启动/停止类操作的结果。
type ActionResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// UpdateView 是 UI 可见的更新检查结果。
type UpdateView struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	Available   bool   `json:"available"`
	Notes       string `json:"notes"`
	URL         string `json:"url"`
	AssetName   string `json:"assetName"`
	AssetURL    string `json:"assetUrl"`
	AssetDigest string `json:"assetDigest"`
	AssetSize   int64  `json:"assetSize"`
	ChecksumURL string `json:"checksumUrl"`
	PublishedAt string `json:"publishedAt"`
	Downloaded  bool   `json:"downloaded"`
}

func toUpdateView(info update.Info) UpdateView {
	return UpdateView{
		Current: info.Current, Latest: info.Latest, Available: info.Available,
		Notes: info.Notes, URL: info.URL, AssetName: info.AssetName,
		AssetURL: info.AssetURL, AssetDigest: info.AssetDigest, AssetSize: info.AssetSize,
		ChecksumURL: info.ChecksumURL, PublishedAt: info.PublishedAt,
	}
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
	gameCfg := make(map[string]config.GameConfig)
	for _, game := range games.All() {
		cfg, err := config.LoadGame(gameConfigFile(dataDir, game.ID()))
		if err != nil {
			return nil, err
		}
		gameCfg[game.ID()] = cfg
	}
	if err := config.SaveApp(filepath.Join(dataDir, "config.yaml"), appCfg); err != nil {
		return nil, err
	}
	for id, cfg := range gameCfg {
		if err := config.SaveGame(gameConfigFile(dataDir, id), cfg); err != nil {
			return nil, err
		}
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
	// 启动时清掉历史诊断截图，避免 debug 目录长期堆积。
	pruneDebugFrames(filepath.Join(dataDir, "debug"), debugFramesKeep)
	svc := &Service{dataDir: dataDir, assets: assets, logger: logger, logFile: logFile, sink: opts.EventSink, appCfg: appCfg, gameCfg: gameCfg, updateChecker: update.NewChecker(appCfg.Update.Repo), quit: opts.Quit, sessions: make(map[string]*gameSession), waiting: make(map[string]bool)}
	svc.appendLog("info", "skip-all 服务已初始化")
	return svc, nil
}

// Shutdown 停止会话并释放资源。
func (s *Service) Shutdown() {
	s.StopAll()
	s.mu.Lock()
	if s.watchStop != nil {
		close(s.watchStop)
		s.watchStop = nil
	}
	if s.ocr != nil {
		_ = s.ocr.Close()
		s.ocr = nil
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
	}
	s.mu.Unlock()
}

// ReleaseIdleResources 在没有会话运行时释放 OCR 模型，供轻量模式回收内存。
// 下一次需要 OCR 时会重新加载，代价是首帧慢一点。
func (s *Service) ReleaseIdleResources() {
	if len(s.runningSessions()) > 0 {
		return
	}
	s.mu.Lock()
	ocr := s.ocr
	s.ocr = nil
	s.ocrBackend = "未加载"
	s.mu.Unlock()
	if ocr == nil {
		return
	}
	if err := ocr.Close(); err != nil {
		s.appendLog("warn", "释放 OCR 资源失败: "+err.Error())
		return
	}
	s.appendLog("info", "已释放 OCR 资源，进入轻量模式")
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

// gameConfigFile 返回单个游戏的配置文件路径。
func gameConfigFile(dataDir, gameID string) string {
	return filepath.Join(dataDir, "games", gameID+".yaml")
}

// gameConfigLocked 返回指定游戏的配置；缺失时回落到默认配置。调用方需持有 s.mu。
func (s *Service) gameConfigLocked(gameID string) config.GameConfig {
	if cfg, ok := s.gameCfg[gameID]; ok {
		return cfg
	}
	return config.DefaultGame()
}

// GetConfig 返回当前配置。
func (s *Service) GetConfig() ConfigView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ConfigView{App: s.appCfg, Games: s.gameCfg}
}

// SaveConfig 保存配置并更新内存快照；运行中的会话会用新配置自动重启。
func (s *Service) SaveConfig(view ConfigView) error {
	if err := config.SaveApp(filepath.Join(s.dataDir, "config.yaml"), view.App); err != nil {
		return err
	}
	// 逐个游戏保存；前端传来的未知 ID 直接忽略。
	saved := make(map[string]config.GameConfig, len(view.Games))
	for id, cfg := range view.Games {
		if _, ok := games.Get(id); !ok {
			continue
		}
		if err := config.SaveGame(gameConfigFile(s.dataDir, id), cfg); err != nil {
			return err
		}
		saved[id] = cfg
	}
	s.mu.Lock()
	s.appCfg = view.App
	for id, cfg := range saved {
		s.gameCfg[id] = cfg
	}
	s.mu.Unlock()
	s.restartRunning()
	return nil
}

// restartRunning 在配置保存后让所有运行中的会话立即使用新配置。
func (s *Service) restartRunning() {
	items := s.runningSessions()
	if len(items) == 0 {
		return
	}
	s.StopAll()
	s.mu.Lock()
	s.autoStop = false
	s.mu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.runningSessions()) == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, item := range items {
		if s.isExcluded(item.gameID) {
			s.appendLog("info", "["+item.gameID+"] 已加入排除列表，停止")
			continue
		}
		// 按当前配置重新算启用功能：新开的功能会跟着起来，被停用的不再重启。
		if res := s.StartGame(item.gameID); !res.OK {
			s.appendLog("warn", "应用新配置后重启失败: "+res.Error)
		}
	}
}

type sessionRef struct{ gameID string }

func (s *Service) runningSessions() []sessionRef {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sessionRef
	for _, sess := range s.sessions {
		st := sess.eng.Status()
		if st.State == engine.StateRunning || st.State == engine.StatePaused {
			out = append(out, sessionRef{gameID: sess.gameID})
		}
	}
	return out
}

// Start 启动指定游戏的单个功能；功能已禁用或未知时返回错误。
func (s *Service) Start(gameID, featureID string) ActionResult {
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
	s.mu.Lock()
	enabled := s.featureEnabledLocked(gameID, featureID)
	s.mu.Unlock()
	if !enabled {
		return ActionResult{OK: false, Error: "功能已在配置中禁用"}
	}
	return s.startGame(game, []assist.Feature{feature})
}

// StartGame 启动一个游戏里全部启用的功能（剧情跳过 + 自动战斗并行）。
func (s *Service) StartGame(gameID string) ActionResult {
	game, ok := games.Get(gameID)
	if !ok {
		return ActionResult{OK: false, Error: "未知游戏: " + gameID}
	}
	return s.startGame(game, s.enabledFeatures(game))
}

// startGame 为一个游戏创建 engine：同一帧截图喂给多个功能，动作串行执行。
func (s *Service) startGame(game assist.Game, features []assist.Feature) ActionResult {
	if len(features) == 0 {
		return ActionResult{OK: false, Error: "没有启用的功能"}
	}
	gameID := game.ID()
	s.mu.Lock()
	if sess, ok := s.sessions[gameID]; ok {
		st := sess.eng.Status()
		if st.State == engine.StateRunning || st.State == engine.StatePaused {
			s.mu.Unlock()
			return ActionResult{OK: false, Error: "该游戏已有会话在运行"}
		}
	}
	appCfg := s.appCfg
	s.mu.Unlock()

	win, err := platform.Find(game.WindowTitles(), game.ProcessNames())
	if err != nil {
		return ActionResult{OK: false, Error: "找不到游戏窗口: " + err.Error()}
	}
	capturer := platform.NewCapturer(win)
	background := strings.ToLower(appCfg.Input.Mode) != "foreground"
	if game.RequiresForeground() && background {
		background = false
		s.appendLog("info", "["+game.Name()+"] 该游戏需要前台真实输入，已覆盖输入模式")
	}
	input := platform.NewInputWithOptions(win, platform.InputOptions{Background: background, Gamepad: appCfg.Input.Gamepad, GamepadDLL: s.gamepadDLL(), CursorMode: appCfg.Input.CursorMode})
	if appCfg.Input.Gamepad && !input.GamepadAvailable() {
		s.appendLog("warn", "["+game.Name()+"] 虚拟手柄不可用: "+fmt.Sprint(input.GamepadError()))
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
	targetFPS := appCfg.Capture.TargetFPS
	specs := make([]engine.FeatureSpec, 0, len(features))
	names := make([]string, 0, len(features))
	ids := make([]string, 0, len(features))
	for _, feature := range features {
		settings := feature.DefaultSettings()
		s.mu.Lock()
		if fc, ok := s.gameConfigLocked(gameID).Features[feature.ID()]; ok && fc.Settings != nil {
			settings = fc.Settings
		}
		s.mu.Unlock()
		if settings == nil {
			settings = assist.Settings{}
		}
		if feature.ID() == "story_skip" {
			settings[storyskip.SettingGamepad] = appCfg.Input.Gamepad && input.GamepadAvailable()
		}
		if err := feature.Validate(settings); err != nil {
			s.appendLog("warn", "["+game.Name()+"] "+feature.Name()+" 配置无效，跳过: "+err.Error())
			continue
		}
		specs = append(specs, engine.FeatureSpec{Feature: feature, Settings: assist.Settings(settings)})
		names = append(names, feature.Name())
		ids = append(ids, feature.ID())
		if needs, ok := feature.(assist.FrameRateFeature); ok && needs.MinFPS() > targetFPS {
			targetFPS = needs.MinFPS()
		}
	}
	if len(specs) == 0 {
		return ActionResult{OK: false, Error: "没有可运行的功能"}
	}
	alive := s.processGate(game.ProcessNames())
	eng := engine.New(capturer, input, screens, matcher, s.logger, specs, engine.Config{
		TargetFPS:           targetFPS,
		PauseWhenBackground: appCfg.Input.PauseWhenBackground,
		ActivateWindow:      !background,
		ProcessAlive:        alive,
	}, func(ev engine.Event) { s.onEngineEvent(gameID, ev) })
	ctx, cancel := context.WithCancel(context.Background())
	sess := &gameSession{gameID: gameID, featureIDs: ids, eng: eng, cancel: cancel}
	s.mu.Lock()
	s.sessions[gameID] = sess
	s.mu.Unlock()
	s.appendLog("info", "["+game.Name()+"] 启动 "+strings.Join(names, " + "))
	go func() {
		err := eng.Run(ctx)
		if err != nil {
			s.appendLog("error", "["+game.Name()+"] "+err.Error())
		}
		s.mu.Lock()
		if cur, ok := s.sessions[gameID]; ok && cur == sess {
			delete(s.sessions, gameID)
		}
		s.mu.Unlock()
		s.emit("skipall:status", s.GetStatus())
	}()
	return ActionResult{OK: true}
}

// processGate 返回带缓存的进程存活判定：每 1 秒最多枚举一次进程表。
func (s *Service) processGate(processNames []string) func() bool {
	var mu sync.Mutex
	var at time.Time
	alive := true
	return func() bool {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(at) < time.Second {
			return alive
		}
		at = time.Now()
		alive = platform.ProcessAlive(processNames)
		return alive
	}
}

// featureEnabledLocked 报告功能是否启用；配置缺失按启用处理。调用方需持有 s.mu。
func (s *Service) featureEnabledLocked(gameID, featureID string) bool {
	if fc, ok := s.gameConfigLocked(gameID).Features[featureID]; ok {
		return fc.Enabled
	}
	return true
}

// enabledFeatures 返回游戏配置里启用的功能，保持 Features() 的声明顺序。
func (s *Service) enabledFeatures(game assist.Game) []assist.Feature {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []assist.Feature
	for _, f := range game.Features() {
		if s.featureEnabledLocked(game.ID(), f.ID()) {
			out = append(out, f)
		}
	}
	return out
}

// StartAll 启动所有未排除游戏的默认功能；游戏未启动时由后台扫描等待。
func (s *Service) StartAll() ActionResult {
	s.mu.Lock()
	s.autoStop = false
	s.mu.Unlock()
	s.startWatcher()
	started := 0
	var failures []string
	for _, game := range games.All() {
		if s.isExcluded(game.ID()) {
			s.appendLog("info", "["+game.Name()+"] 已在排除列表，跳过")
			continue
		}
		if len(s.enabledFeatures(game)) == 0 {
			continue
		}
		s.mu.Lock()
		sess, exists := s.sessions[game.ID()]
		s.mu.Unlock()
		if exists {
			st := sess.eng.Status()
			if st.State == engine.StateRunning || st.State == engine.StatePaused {
				continue
			}
		}
		res := s.StartGame(game.ID())
		if res.OK {
			started++
		} else {
			failures = append(failures, game.Name()+": "+res.Error)
			s.appendLog("info", "["+game.Name()+"] 等待启动: "+res.Error)
		}
	}
	if started == 0 && len(failures) > 0 {
		return ActionResult{OK: false, Error: strings.Join(failures, "；")}
	}
	return ActionResult{OK: true}
}

// StopAll 停止所有游戏会话。
func (s *Service) StopAll() {
	s.mu.Lock()
	s.autoStop = true
	sessions := make([]*gameSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.Unlock()
	for _, sess := range sessions {
		sess.cancel()
		sess.eng.Stop()
	}
}

// Stop 兼容旧调用，停止全部会话。
func (s *Service) Stop() { s.StopAll() }

// PauseAll 暂停所有运行中的会话。
func (s *Service) PauseAll() {
	s.mu.Lock()
	sessions := make([]*gameSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.Unlock()
	for _, sess := range sessions {
		sess.eng.Pause()
	}
	s.emit("skipall:status", s.GetStatus())
}

// Pause 兼容旧调用。
func (s *Service) Pause() { s.PauseAll() }

// ResumeAll 恢复所有暂停的会话。
func (s *Service) ResumeAll() {
	s.mu.Lock()
	sessions := make([]*gameSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.Unlock()
	for _, sess := range sessions {
		sess.eng.Resume()
	}
	s.emit("skipall:status", s.GetStatus())
}

// Resume 兼容旧调用。
func (s *Service) Resume() { s.ResumeAll() }

// GetStatus 返回所有会话的聚合状态。
func (s *Service) GetStatus() StatusView {
	s.mu.Lock()
	sessions := make([]*gameSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	backend := s.ocrBackend
	s.mu.Unlock()
	if len(sessions) == 0 {
		return StatusView{State: string(engine.StateIdle), OCRBackend: backend, Features: []FeatureStateView{}}
	}
	status := StatusView{State: string(engine.StateStopped), OCRBackend: backend, Features: []FeatureStateView{}}
	var gameIDs, featureIDs, featureNames []string
	for _, sess := range sessions {
		st := sess.eng.Status()
		gameIDs = append(gameIDs, sess.gameID)
		status.TargetFPS = st.TargetFPS
		status.TickID = st.TickID
		status.TickLagMs = float64(st.TickLag.Microseconds()) / 1000
		status.TickDropped += st.TickDropped
		if st.LastMessage != "" {
			status.Message = st.LastMessage
		}
		if st.LastError != "" {
			status.Error = st.LastError
		}
		status.FPS += st.FPS
		if status.StartedAt.IsZero() || (!st.StartedAt.IsZero() && st.StartedAt.Before(status.StartedAt)) {
			status.StartedAt = st.StartedAt
		}
		switch st.State {
		case engine.StateRunning:
			status.State = string(engine.StateRunning)
			status.Running = true
		case engine.StatePaused:
			if !status.Running {
				status.State = string(engine.StatePaused)
				status.Paused = true
			}
		case engine.StateError:
			if !status.Running && !status.Paused {
				status.State = string(engine.StateError)
			}
		}
		for _, fs := range st.Features {
			featureIDs = append(featureIDs, fs.ID)
			featureNames = append(featureNames, fs.Name)
			status.Features = append(status.Features, FeatureStateView{
				GameID: sess.gameID, FeatureID: fs.ID, Name: fs.Name,
				State: string(fs.State), Message: fs.Message,
			})
		}
	}
	if len(sessions) > 0 {
		status.FPS /= float64(len(sessions))
	}
	if status.Running {
		status.Paused = false
	}
	status.GameID = strings.Join(gameIDs, ",")
	status.FeatureID = strings.Join(featureIDs, ",")
	status.FeatureName = strings.Join(featureNames, " + ")
	if len(sessions) > 1 {
		status.FeatureName = "全部游戏"
	}
	return status
}

// GetTrayState 汇总托盘要显示的内容：游戏列表与状态、功能开关、性能占用。
func (s *Service) GetTrayState() TrayStateView {
	all := games.All()
	s.mu.Lock()
	states := make([]GameStateView, 0, len(all))
	for _, game := range all {
		item := GameStateView{ID: game.ID(), Name: game.Name(), State: string(engine.StateIdle)}
		if sess, ok := s.sessions[game.ID()]; ok {
			st := sess.eng.Status()
			item.State = string(st.State)
			item.Message = st.LastMessage
		}
		states = append(states, item)
	}
	features := s.featureTogglesLocked(all)
	s.mu.Unlock()

	for i, game := range all {
		states[i].ProcessRunning = platform.ProcessAlive(game.ProcessNames())
	}
	cpu, mem := platform.ProcessUsage()
	return TrayStateView{Games: states, Features: features, Status: s.GetStatus(), CPU: cpu, MemoryMB: mem}
}

// featureTogglesLocked 汇总功能在全部游戏上的启用状态；全部游戏都启用才算启用。调用方需持有 s.mu。
func (s *Service) featureTogglesLocked(all []assist.Game) []FeatureToggleView {
	var out []FeatureToggleView
	index := make(map[string]int)
	for _, game := range all {
		for _, f := range game.Features() {
			i, ok := index[f.ID()]
			if !ok {
				out = append(out, FeatureToggleView{ID: f.ID(), Name: f.Name(), Enabled: true})
				i = len(out) - 1
				index[f.ID()] = i
			}
			if !s.featureEnabledLocked(game.ID(), f.ID()) {
				out[i].Enabled = false
			}
		}
	}
	return out
}

// ToggleFeatureEverywhere 翻转功能开关并返回新状态，供全局快捷键使用。
func (s *Service) ToggleFeatureEverywhere(featureID string) (bool, error) {
	s.mu.Lock()
	enabled := true
	for _, game := range games.All() {
		if !s.featureEnabledLocked(game.ID(), featureID) {
			enabled = false
			break
		}
	}
	s.mu.Unlock()
	return !enabled, s.SetFeatureEnabledEverywhere(featureID, !enabled)
}

// SetFeatureEnabledEverywhere 在全部游戏上启用/停用某个功能；运行中的会话按新配置重启。
// 玩家同一时间只玩一个游戏，托盘里的功能开关就没必要再按游戏区分。
func (s *Service) SetFeatureEnabledEverywhere(featureID string, enabled bool) error {
	all := games.All()
	s.mu.Lock()
	saved := make(map[string]config.GameConfig, len(all))
	for _, game := range all {
		cfg := s.gameConfigLocked(game.ID())
		if cfg.Features == nil {
			cfg.Features = make(map[string]config.StorySkipConfig, 2)
		}
		fc := cfg.Features[featureID]
		fc.Enabled = enabled
		cfg.Features[featureID] = fc
		s.gameCfg[game.ID()] = cfg
		saved[game.ID()] = cfg
	}
	s.mu.Unlock()
	for id, cfg := range saved {
		if err := config.SaveGame(gameConfigFile(s.dataDir, id), cfg); err != nil {
			return err
		}
	}
	s.appendLog("info", featureLabel(all, featureID)+" "+enabledText(enabled)+"（全部游戏）")
	s.restartRunning()
	return nil
}

// featureLabel 找功能的中文名，找不到就用 ID。
func featureLabel(all []assist.Game, featureID string) string {
	for _, game := range all {
		for _, f := range game.Features() {
			if f.ID() == featureID {
				return f.Name()
			}
		}
	}
	return featureID
}

// enabledText 把开关状态转成日志用词。
func enabledText(enabled bool) string {
	if enabled {
		return "已启用"
	}
	return "已停用"
}

func (s *Service) isExcluded(gameID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.appCfg.ExcludedGames {
		if id == gameID {
			return true
		}
	}
	return false
}

// startWatcher 启动后台扫描，等待游戏窗口出现后自动开始。
func (s *Service) startWatcher() {
	s.mu.Lock()
	if s.watchStop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.watchStop = stop
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.scanGames()
			}
		}
	}()
}

func (s *Service) scanGames() {
	s.mu.Lock()
	stopped := s.autoStop
	s.mu.Unlock()
	if stopped {
		return
	}
	for _, game := range games.All() {
		if s.isExcluded(game.ID()) {
			continue
		}
		if len(s.enabledFeatures(game)) == 0 {
			continue
		}
		s.mu.Lock()
		sess, exists := s.sessions[game.ID()]
		s.mu.Unlock()
		if exists {
			st := sess.eng.Status()
			if st.State == engine.StateRunning || st.State == engine.StatePaused {
				continue
			}
		}
		res := s.StartGame(game.ID())
		if res.OK {
			s.mu.Lock()
			delete(s.waiting, game.ID())
			s.mu.Unlock()
			s.appendLog("info", "["+game.Name()+"] 检测到游戏窗口，已自动启动")
			continue
		}
		s.mu.Lock()
		already := s.waiting[game.ID()]
		if !already {
			s.waiting[game.ID()] = true
		}
		s.mu.Unlock()
		if !already {
			s.appendLog("info", "["+game.Name()+"] 暂未找到游戏窗口，后台等待")
		}
	}
}
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
	pruneDebugFrames(frameDir, debugFramesKeep)
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

// debugFramesKeep 是 debug 目录保留的诊断截图数量。
const debugFramesKeep = 5

// pruneDebugFrames 只保留最近的几张诊断截图，避免 debug 目录无限增长。
func pruneDebugFrames(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type frame struct {
		path string
		mod  time.Time
	}
	var frames []frame
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "doctor-") || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		frames = append(frames, frame{path: filepath.Join(dir, e.Name()), mod: info.ModTime()})
	}
	if len(frames) <= keep {
		return
	}
	sort.Slice(frames, func(i, j int) bool { return frames[i].mod.After(frames[j].mod) })
	for _, f := range frames[keep:] {
		_ = os.Remove(f.path)
	}
}

func (s *Service) ensureOCR() (vision.OCR, error) {
	s.mu.Lock()
	if s.ocr != nil {
		defer s.mu.Unlock()
		return s.ocr, nil
	}
	appCfg := s.appCfg
	dir, err := s.modelDirLocked()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	mode := appCfg.OCR.GPU
	if mode == "" {
		mode = "auto"
	}
	dml := s.findOCRRuntime("onnxruntime_dml.dll", appCfg.OCR.RuntimeDir)
	cuda := s.findOCRRuntime("onnxruntime_cuda.dll", appCfg.OCR.RuntimeDir)
	var candidates []ocrCandidate
	dmlDevice := 0
	if (mode == "auto" || mode == "dml") && dml != "" {
		if index, name := platform.DiscreteGPUIndex(); name != "" {
			dmlDevice = index
			s.appendLog("info", fmt.Sprintf("DirectML 选择独立显卡 [%d] %s", index, name))
		}
		candidates = append(candidates, ocrCandidate{name: "DirectML", runtime: dml, directML: true, dmlDevice: dmlDevice})
	}
	if (mode == "auto" || mode == "cuda") && cuda != "" {
		candidates = append(candidates, ocrCandidate{name: "CUDA", runtime: cuda, cuda: true})
	}
	// 如果 GPU provider 初始化失败，优先用同一个 ONNX Runtime DLL 建 CPU session；
	// 该 DLL 同时包含 CPU provider，不需要重新加载一个 ORT 引擎。
	cpuRuntime := ""
	if len(candidates) > 0 {
		cpuRuntime = candidates[0].runtime
	}
	candidates = append(candidates, ocrCandidate{name: "CPU", runtime: cpuRuntime})
	lazy := vision.NewLazyOCR(func() (vision.OCR, error) {
		var lastErr error
		for _, candidate := range candidates {
			ocr, err := vision.NewPaddleOCRWithOptions(dir, vision.PaddleOptions{
				UseDirectML: candidate.directML,
				DMLDeviceID: candidate.dmlDevice,
				UseCUDA:     candidate.cuda,
				RuntimeLib:  candidate.runtime,
			})
			if err == nil {
				s.mu.Lock()
				s.ocrBackend = candidate.name
				s.mu.Unlock()
				s.appendLog("info", "OCR 后端: "+candidate.name+" ("+candidate.runtime+")")
				return ocr, nil
			}
			lastErr = err
			s.appendLog("warn", "OCR "+candidate.name+" 初始化失败，尝试下一个后端: "+err.Error())
		}
		return nil, fmt.Errorf("OCR 初始化失败: %w", lastErr)
	})
	s.mu.Lock()
	s.ocr = lazy
	s.ocrBackend = "未加载"
	s.mu.Unlock()
	s.appendLog("info", "OCR 延迟加载：首次需要 OCR 时再初始化模型")
	return lazy, nil
}

type ocrCandidate struct {
	name      string
	runtime   string
	directML  bool
	dmlDevice int
	cuda      bool
}

// findOCRRuntime 依次在自定义目录、assets/runtime、exe/runtime 找 GPU 版 ORT DLL。
func (s *Service) findOCRRuntime(name, customDir string) string {
	var dirs []string
	if customDir != "" {
		dirs = append(dirs, customDir)
	}
	if s.assets != "" {
		dirs = append(dirs, filepath.Join(s.assets, "runtime"))
	}
	if s.dataDir != "" {
		dirs = append(dirs, filepath.Join(s.dataDir, "runtime"))
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "runtime"))
	}
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
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

func (s *Service) onEngineEvent(gameID string, ev engine.Event) {
	name := gameID
	if game, ok := games.Get(gameID); ok {
		name = game.Name()
	}
	message := "[" + name + "] " + ev.Message
	s.appendLog(ev.Level, message)
	s.emit("skipall:log", LogEntry{Time: ev.Time, Level: ev.Level, Message: message})
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
