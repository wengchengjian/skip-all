package storyskip

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// 设置项名称。
const (
	SettingMode                = "mode"
	SettingDialogOption        = "dialog_option"
	SettingDialogClickInterval = "dialog_click_interval"
	SettingOptionClickInterval = "option_click_interval"
	SettingSleepAfterEmpty     = "sleep_after_empty"
	SettingGamepad             = "gamepad"
)

// Feature 是剧情跳过功能。
type Feature struct {
	spec Spec
}

// StorySkipFeature 是剧情跳过 Feature 的别名，便于按功能名引用。
type StorySkipFeature = Feature

var _ assist.Feature = (*Feature)(nil)

// NewFeature 创建剧情跳过功能。
func NewFeature(spec Spec) *Feature {
	return &Feature{spec: spec.withDefaults()}
}

// ID 返回功能标识。
func (f *Feature) ID() string { return "story_skip" }

// Name 返回功能名称。
func (f *Feature) Name() string { return "剧情跳过" }

// Description 返回功能说明。
func (f *Feature) Description() string {
	return "识别剧情画面并自动点击跳过、确认框或对话选项。"
}

// DefaultSettings 返回默认设置。
func (f *Feature) DefaultSettings() assist.Settings {
	return assist.Settings{
		SettingMode:                string(ModeSkip),
		SettingDialogOption:        string(OptionLast),
		SettingDialogClickInterval: 0.5,
		SettingOptionClickInterval: 0.1,
		SettingSleepAfterEmpty:     0.5,
		SettingGamepad:             false,
	}
}

// Validate 校验功能设置。
func (f *Feature) Validate(settings assist.Settings) error {
	_, err := parseSettings(settings)
	return err
}

// NewSession 创建剧情跳过会话。
func (f *Feature) NewSession(rt assist.Runtime, settings assist.Settings) (assist.Session, error) {
	if rt == nil {
		return nil, errors.New("剧情跳过缺少运行时")
	}
	if err := f.spec.validate(); err != nil {
		return nil, err
	}
	cfg, err := parseSettings(settings)
	if err != nil {
		return nil, err
	}
	return newSession(rt, f.spec, cfg), nil
}

type sessionSettings struct {
	mode                Mode
	dialogOption        DialogOption
	dialogClickInterval time.Duration
	optionClickInterval time.Duration
	sleepAfterEmpty     time.Duration
	gamepad             bool
}

func defaultSessionSettings() sessionSettings {
	return sessionSettings{
		mode:                ModeSkip,
		dialogOption:        OptionLast,
		dialogClickInterval: 500 * time.Millisecond,
		optionClickInterval: 100 * time.Millisecond,
		sleepAfterEmpty:     500 * time.Millisecond,
	}
}

func parseSettings(settings assist.Settings) (sessionSettings, error) {
	cfg := defaultSessionSettings()
	if settings == nil {
		return cfg, nil
	}

	if value, ok := stringSetting(settings, SettingMode); ok {
		switch Mode(strings.ToLower(value)) {
		case ModeSkip, ModeAuto, ModeClick:
			cfg.mode = Mode(strings.ToLower(value))
		default:
			return cfg, fmt.Errorf("设置 %s 不支持 %q，可选 skip/auto/click", SettingMode, value)
		}
	}
	if value, ok := stringSetting(settings, SettingDialogOption); ok {
		switch DialogOption(strings.ToLower(value)) {
		case OptionFirst, OptionLast:
			cfg.dialogOption = DialogOption(strings.ToLower(value))
		default:
			return cfg, fmt.Errorf("设置 %s 不支持 %q，可选 first/last", SettingDialogOption, value)
		}
	}

	var err error
	if cfg.dialogClickInterval, err = durationSetting(settings, SettingDialogClickInterval, cfg.dialogClickInterval); err != nil {
		return cfg, err
	}
	if cfg.dialogClickInterval <= 0 {
		return cfg, fmt.Errorf("设置 %s 必须大于 0", SettingDialogClickInterval)
	}
	if cfg.optionClickInterval, err = durationSetting(settings, SettingOptionClickInterval, cfg.optionClickInterval); err != nil {
		return cfg, err
	}
	if cfg.optionClickInterval < 0 {
		return cfg, fmt.Errorf("设置 %s 不能为负数", SettingOptionClickInterval)
	}
	if cfg.sleepAfterEmpty, err = durationSetting(settings, SettingSleepAfterEmpty, cfg.sleepAfterEmpty); err != nil {
		return cfg, err
	}
	if cfg.sleepAfterEmpty < 0 {
		return cfg, fmt.Errorf("设置 %s 不能为负数", SettingSleepAfterEmpty)
	}
	if value, ok := boolSetting(settings, SettingGamepad); ok {
		cfg.gamepad = value
	}
	return cfg, nil
}

func boolSetting(settings assist.Settings, key string) (bool, bool) {
	value, ok := settings[key]
	if !ok || value == nil {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return parsed, err == nil
	default:
		return false, false
	}
}

func stringSetting(settings assist.Settings, key string) (string, bool) {
	value, ok := settings[key]
	if !ok || value == nil {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text, text != ""
}

func durationSetting(settings assist.Settings, key string, fallback time.Duration) (time.Duration, error) {
	value, ok := settings[key]
	if !ok || value == nil {
		return fallback, nil
	}
	var seconds float64
	switch v := value.(type) {
	case float64:
		seconds = v
	case float32:
		seconds = float64(v)
	case int:
		seconds = float64(v)
	case int64:
		seconds = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return fallback, fmt.Errorf("设置 %s 不是有效数字: %w", key, err)
		}
		seconds = parsed
	default:
		return fallback, fmt.Errorf("设置 %s 不是有效数字", key)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
