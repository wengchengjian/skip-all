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

// OCRConfig 是 OCR 模型配置。
type OCRConfig struct {
	Model        string `yaml:"model"`
	AutoDownload bool   `yaml:"auto_download"`
	ModelDir     string `yaml:"model_dir"`
}

// DefaultApp 返回应用默认配置。
func DefaultApp() AppConfig {
	return AppConfig{
		SchemaVersion: SchemaVersion,
		Language:      "zh-CN",
		LogLevel:      "info",
		Capture:       CaptureConfig{Method: "auto", TargetFPS: 10},
		Input:         InputConfig{PauseWhenBackground: false, Mode: "background", CursorMode: "auto", Gamepad: false},
		Hotkeys:       HotkeysConfig{Toggle: "F9", Panic: "F10"},
		OCR:           OCRConfig{Model: "ppocrv6", AutoDownload: true},
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
	if c.OCR.Model == "" {
		c.OCR.Model = "ppocrv6"
	}
	return nil
}

// StorySkipConfig 是剧情跳过功能的配置。Settings 原样传给 Feature。
type StorySkipConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Settings map[string]any `yaml:"settings"`
}

// ZZZConfig 是绝区零游戏配置。
type ZZZConfig struct {
	SchemaVersion int                        `yaml:"schema_version"`
	Features      map[string]StorySkipConfig `yaml:"features"`
}

// DefaultZZZ 返回绝区零默认配置。
func DefaultZZZ() ZZZConfig {
	return ZZZConfig{
		SchemaVersion: SchemaVersion,
		Features: map[string]StorySkipConfig{
			"story_skip": {Enabled: true, Settings: map[string]any{}},
		},
	}
}

// LoadZZZ 读取绝区零配置；文件不存在时返回默认配置。
func LoadZZZ(path string) (ZZZConfig, error) {
	cfg := DefaultZZZ()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return ZZZConfig{}, fmt.Errorf("读取游戏配置 %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ZZZConfig{}, fmt.Errorf("解析游戏配置 %s: %w", path, err)
	}
	if err := cfg.normalizeAndValidate(); err != nil {
		return ZZZConfig{}, fmt.Errorf("校验游戏配置 %s: %w", path, err)
	}
	return cfg, nil
}

// SaveZZZ 保存绝区零配置。
func SaveZZZ(path string, cfg ZZZConfig) error {
	if err := cfg.normalizeAndValidate(); err != nil {
		return fmt.Errorf("校验游戏配置: %w", err)
	}
	return writeYAML(path, cfg)
}

func (c *ZZZConfig) normalizeAndValidate() error {
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
