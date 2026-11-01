package combat

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wengchengjian/skip-all/pkg/assist"
)

// 设置项名称。
const (
	SettingUseLiberation = "use_liberation"
	SettingUseEcho       = "use_echo"
	SettingUseDodge      = "use_dodge"
	SettingSwitchHealer  = "switch_healer"
	SettingUseConcerto   = "use_concerto_switch"
	// SettingExtPrefix 是游戏专属能力开关的前缀，后面接 Extension.Name()。
	SettingExtPrefix = "ext_"
	SettingInterval  = "interval"
	SettingHold      = "hold"
)

// Feature 是自动战斗功能。
type Feature struct {
	spec Spec
}

var (
	_ assist.Feature          = (*Feature)(nil)
	_ assist.FrameRateFeature = (*Feature)(nil)
)

// NewFeature 创建自动战斗功能。
func NewFeature(spec Spec) *Feature {
	return &Feature{spec: spec.withDefaults()}
}

// ID 返回功能标识。
func (f *Feature) ID() string { return "auto_combat" }

// Name 返回功能名称。
func (f *Feature) Name() string { return "自动战斗" }

// Description 返回功能说明。
func (f *Feature) Description() string {
	return "识别进入战斗后按连招循环出招，脱离战斗自动停下。"
}

// MinFPS 返回自动战斗需要的最低帧率。上游 ok-ww 的触发间隔是 0.1 秒，
// 5 FPS 的截图节奏会把连招拖到每 200 毫秒才出一次手。
func (f *Feature) MinFPS() int { return 10 }

// DefaultSettings 返回默认设置。
func (f *Feature) DefaultSettings() assist.Settings {
	settings := assist.Settings{
		SettingUseLiberation: true,
		SettingUseEcho:       true,
		SettingUseDodge:      true,
		SettingSwitchHealer:  f.spec.HealerKey != "",
		SettingUseConcerto:   f.spec.Concerto != nil,
		SettingInterval:      f.spec.Interval.Seconds(),
		SettingHold:          f.spec.Hold.Seconds(),
	}
	for _, ext := range f.spec.Extensions {
		settings[SettingExtPrefix+ext.Name()] = true
	}
	return settings
}

// Validate 校验功能设置。
func (f *Feature) Validate(settings assist.Settings) error {
	_, err := parseSettings(settings, f.spec)
	return err
}

// NewSession 创建自动战斗会话。
func (f *Feature) NewSession(rt assist.Runtime, settings assist.Settings) (assist.Session, error) {
	if rt == nil {
		return nil, errors.New("自动战斗缺少运行时")
	}
	if err := f.spec.validate(); err != nil {
		return nil, err
	}
	cfg, err := parseSettings(settings, f.spec)
	if err != nil {
		return nil, err
	}
	return newSession(rt, f.spec, cfg), nil
}

// sessionSettings 是解析后的会话设置。
type sessionSettings struct {
	noLiberation bool
	noEcho       bool
	noDodge      bool
	noHealer     bool
	noConcerto   bool
	extOff       map[string]bool
}

// disabledExt 报告某个游戏专属能力是否被设置关掉。
func (c sessionSettings) disabledExt(name string) bool {
	return c.extOff[name]
}

func parseSettings(settings assist.Settings, spec Spec) (sessionSettings, error) {
	cfg := sessionSettings{}
	if settings == nil {
		return cfg, nil
	}
	if value, ok := boolSetting(settings, SettingUseLiberation); ok {
		cfg.noLiberation = !value
	}
	if value, ok := boolSetting(settings, SettingUseEcho); ok {
		cfg.noEcho = !value
	}
	if value, ok := boolSetting(settings, SettingUseDodge); ok {
		cfg.noDodge = !value
	}
	if value, ok := boolSetting(settings, SettingSwitchHealer); ok {
		cfg.noHealer = !value
	}
	if value, ok := boolSetting(settings, SettingUseConcerto); ok {
		cfg.noConcerto = !value
	}
	if spec.Extensions != nil {
		cfg.extOff = make(map[string]bool, len(spec.Extensions))
		for _, ext := range spec.Extensions {
			if value, ok := boolSetting(settings, SettingExtPrefix+ext.Name()); ok && !value {
				cfg.extOff[ext.Name()] = true
			}
		}
	}
	if value, ok := floatSetting(settings, SettingInterval); ok {
		if value <= 0 {
			return cfg, fmt.Errorf("设置 %s 必须大于 0", SettingInterval)
		}
	}
	if value, ok := floatSetting(settings, SettingHold); ok {
		if value < 0 {
			return cfg, fmt.Errorf("设置 %s 不能为负数", SettingHold)
		}
	}
	return cfg, nil
}

func boolSetting(settings assist.Settings, key string) (bool, bool) {
	value, ok := settings[key]
	if !ok {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "on", "yes":
			return true, true
		case "false", "0", "off", "no":
			return false, true
		}
	}
	return false, false
}

func floatSetting(settings assist.Settings, key string) (float64, bool) {
	value, ok := settings[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case time.Duration:
		return v.Seconds(), true
	case string:
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d.Seconds(), true
		}
	}
	return 0, false
}
