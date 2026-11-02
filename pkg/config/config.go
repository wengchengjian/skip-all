// Package config 负责应用与游戏 YAML 配置的加载、默认值和校验。
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SchemaVersion 是当前配置结构版本。
const SchemaVersion = 1

// AppConfig 是应用级配置。
type AppConfig struct {
	SchemaVersion int           `yaml:"schema_version"`
	Language      string        `yaml:"language"`
	LogLevel      string        `yaml:"log_level"`
	Capture       CaptureConfig `yaml:"capture"`
	Input         InputConfig   `yaml:"input"`
	Hotkeys       HotkeysConfig `yaml:"hotkeys"`
	OCR           OCRConfig     `yaml:"ocr"`
	Update        UpdateConfig  `yaml:"update"`
	// ExcludedGames 是启动时跳过的游戏 ID；默认空表示全部启用。
	ExcludedGames []string `yaml:"excluded_games"`
}

// CaptureConfig 是截图参数。
type CaptureConfig struct {
	Method    string `yaml:"method"`
	TargetFPS int    `yaml:"target_fps"`
}

// InputConfig 是输入参数。
type InputConfig struct {
	PauseWhenBackground bool   `yaml:"pause_when_background"`
	Mode                string `yaml:"mode"`        // background | foreground
	CursorMode          string `yaml:"cursor_mode"` // auto | always | never
	Gamepad             bool   `yaml:"gamepad"`
}

// HotkeysConfig 是全局快捷键。
type HotkeysConfig struct {
	Toggle string `yaml:"toggle"`
	Panic  string `yaml:"panic"`
}

// UpdateConfig 是自动更新配置。
type UpdateConfig struct {
	AutoCheck bool   `yaml:"auto_check"`
	Repo      string `yaml:"repo"`
}

// OCRConfig 是 OCR 模型配置。
type OCRConfig struct {
	Model        string `yaml:"model"`
	AutoDownload bool   `yaml:"auto_download"`
	ModelDir     string `yaml:"model_dir"`
	// GPU 控制 OCR 后端：auto / dml / cuda / cpu。
	GPU string `yaml:"gpu"`
	// RuntimeDir 可放置 onnxruntime_dml.dll / onnxruntime_cuda.dll。
	RuntimeDir string `yaml:"runtime_dir"`
}

// DefaultApp 返回应用默认配置。
func DefaultApp() AppConfig {
	return AppConfig{
		SchemaVersion: SchemaVersion,
		Language:      "zh-CN",
		LogLevel:      "info",
		Capture:       CaptureConfig{Method: "auto", TargetFPS: 5},
		Input:         InputConfig{PauseWhenBackground: false, Mode: "background", CursorMode: "auto", Gamepad: false},
		Hotkeys:       HotkeysConfig{Toggle: "F9", Panic: "F10"},
		OCR:           OCRConfig{Model: "ppocrv6", AutoDownload: true, GPU: "auto"},
		Update:        UpdateConfig{AutoCheck: true, Repo: "wengchengjian/skip-all"},
		ExcludedGames: []string{},
	}
}

// LoadApp 读取应用配置；文件不存在时返回默认配置。
func LoadApp(path string) (AppConfig, error) {
	cfg := DefaultApp()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return AppConfig{}, fmt.Errorf("读取应用配置 %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("解析应用配置 %s: %w", path, err)
	}
	if err := cfg.normalizeAndValidate(); err != nil {
		return AppConfig{}, fmt.Errorf("校验应用配置 %s: %w", path, err)
	}
	return cfg, nil
}

// SaveApp 保存应用配置，先写临时文件再替换目标文件。
func SaveApp(path string, cfg AppConfig) error {
	if err := cfg.normalizeAndValidate(); err != nil {
		return fmt.Errorf("校验应用配置: %w", err)
	}
	return writeYAML(path, cfg)
}

func (c *AppConfig) normalizeAndValidate() error {
	if c.SchemaVersion == 0 {
		c.SchemaVersion = SchemaVersion
	}
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("不支持的 schema_version: %d", c.SchemaVersion)
	}
	if c.Language == "" {
		c.Language = "zh-CN"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("未知 log_level: %s", c.LogLevel)
	}
	if c.Capture.Method == "" {
		c.Capture.Method = "auto"
	}
	switch strings.ToLower(c.Capture.Method) {
	case "auto", "printwindow", "bitblt":
	default:
		return fmt.Errorf("未知 capture.method: %s", c.Capture.Method)
	}
	if c.Capture.TargetFPS == 0 {
		c.Capture.TargetFPS = 10
	}
	if c.Capture.TargetFPS < 1 || c.Capture.TargetFPS > 120 {
		return fmt.Errorf("capture.target_fps 超出范围: %d", c.Capture.TargetFPS)
	}
	if c.Input.Mode == "" {
		c.Input.Mode = "background"
	}
	switch strings.ToLower(c.Input.Mode) {
	case "background", "foreground":
		c.Input.Mode = strings.ToLower(c.Input.Mode)
	default:
		return fmt.Errorf("未知 input.mode: %s", c.Input.Mode)
	}
	if c.Input.CursorMode == "" {
		c.Input.CursorMode = "auto"
	}
	switch strings.ToLower(c.Input.CursorMode) {
	case "auto", "always", "never":
		c.Input.CursorMode = strings.ToLower(c.Input.CursorMode)
	default:
		return fmt.Errorf("未知 input.cursor_mode: %s", c.Input.CursorMode)
	}
	if c.Hotkeys.Toggle == "" {
		c.Hotkeys.Toggle = "F9"
	}
	if c.Hotkeys.Panic == "" {
		c.Hotkeys.Panic = "F10"
	}
	if c.Update.Repo == "" {
		c.Update.Repo = "wengchengjian/skip-all"
	}
	if c.OCR.Model == "" {
		c.OCR.Model = "ppocrv6"
	}
	if c.OCR.GPU == "" {
		c.OCR.GPU = "auto"
	}
	switch strings.ToLower(c.OCR.GPU) {
	case "auto", "dml", "cuda", "cpu":
		c.OCR.GPU = strings.ToLower(c.OCR.GPU)
	default:
		return fmt.Errorf("未知 ocr.gpu: %s", c.OCR.GPU)
	}
	seen := make(map[string]bool, len(c.ExcludedGames))
	out := c.ExcludedGames[:0]
	for _, id := range c.ExcludedGames {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	c.ExcludedGames = out
	return nil
}

// StorySkipConfig 是剧情跳过功能的配置。Settings 原样传给 Feature。
type StorySkipConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Settings map[string]any `yaml:"settings"`
}

// GameConfig 是单个游戏的配置。
type GameConfig struct {
	SchemaVersion int                        `yaml:"schema_version"`
	Features      map[string]StorySkipConfig `yaml:"features"`
}

// DefaultGame 返回单个游戏的默认配置。
func DefaultGame() GameConfig {
	return GameConfig{
		SchemaVersion: SchemaVersion,
		Features: map[string]StorySkipConfig{
			"story_skip":  {Enabled: true, Settings: map[string]any{}},
			"auto_combat": {Enabled: true, Settings: map[string]any{}},
		},
	}
}

// LoadGame 读取单个游戏的配置；文件不存在时返回默认配置。
func LoadGame(path string) (GameConfig, error) {
	cfg := DefaultGame()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return GameConfig{}, fmt.Errorf("读取游戏配置 %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return GameConfig{}, fmt.Errorf("解析游戏配置 %s: %w", path, err)
	}
	if err := cfg.normalizeAndValidate(); err != nil {
		return GameConfig{}, fmt.Errorf("校验游戏配置 %s: %w", path, err)
	}
	return cfg, nil
}

// SaveGame 保存单个游戏的配置。
func SaveGame(path string, cfg GameConfig) error {
	if err := cfg.normalizeAndValidate(); err != nil {
		return fmt.Errorf("校验游戏配置: %w", err)
	}
	return writeYAML(path, cfg)
}

func (c *GameConfig) normalizeAndValidate() error {
	if c.SchemaVersion == 0 {
		c.SchemaVersion = SchemaVersion
	}
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("不支持的 schema_version: %d", c.SchemaVersion)
	}
	if c.Features == nil {
		c.Features = map[string]StorySkipConfig{}
	}
	for name, feature := range c.Features {
		if strings.TrimSpace(name) == "" {
			return errors.New("feature 名称不能为空")
		}
		if feature.Settings == nil {
			feature.Settings = map[string]any{}
			c.Features[name] = feature
		}
	}
	return nil
}

func writeYAML(path string, value any) error {
	if path == "" {
		return errors.New("配置路径为空")
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("生成 YAML: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建配置目录 %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时配置: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时配置: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时配置: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换配置 %s: %w", path, err)
	}
	return nil
}
