package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultApp(t *testing.T) {
	cfg := DefaultApp()
	if cfg.SchemaVersion != 1 || cfg.Capture.Method != "auto" || cfg.Capture.TargetFPS != 10 {
		t.Fatalf("默认应用配置错误: %+v", cfg)
	}
	if !cfg.OCR.AutoDownload || cfg.OCR.Model != "ppocrv6" {
		t.Fatalf("默认 OCR 配置错误: %+v", cfg.OCR)
	}
	if cfg.Input.Mode != "background" || cfg.Input.PauseWhenBackground {
		t.Fatalf("默认输入配置错误: %+v", cfg.Input)
	}
}

func TestLoadAppMissingFile(t *testing.T) {
	cfg, err := LoadApp(filepath.Join(t.TempDir(), "not-exist.yml"))
	if err != nil {
		t.Fatalf("文件不存在应返回默认配置: %v", err)
	}
	if cfg.Capture.TargetFPS != 10 {
		t.Errorf("默认帧率错误: %d", cfg.Capture.TargetFPS)
	}
}

func TestLoadAppPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := `
language: en-US
log_level: debug
capture:
  target_fps: 5
ocr:
  auto_download: false
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadApp(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "en-US" || cfg.LogLevel != "debug" || cfg.Capture.TargetFPS != 5 {
		t.Errorf("配置覆盖失败: %+v", cfg)
	}
	if cfg.Capture.Method != "auto" {
		t.Errorf("缺省字段应保留默认值: %+v", cfg.Capture)
	}
	if cfg.OCR.AutoDownload {
		t.Error("显式 false 应覆盖默认 true")
	}
}

func TestLoadAppInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(path, []byte("log_level: verbose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApp(path); err == nil {
		t.Fatal("非法 log_level 应返回错误")
	}
}

func TestSaveAppRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "app.yml")
	cfg := DefaultApp()
	cfg.Capture.TargetFPS = 20
	cfg.OCR.ModelDir = "D:/models"
	if err := SaveApp(path, cfg); err != nil {
		t.Fatalf("SaveApp 失败: %v", err)
	}
	loaded, err := LoadApp(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Capture.TargetFPS != 20 || loaded.OCR.ModelDir != "D:/models" {
		t.Errorf("保存后读取不一致: %+v", loaded)
	}
}

func TestZZZConfig(t *testing.T) {
	def := DefaultZZZ()
	if !def.Features["story_skip"].Enabled {
		t.Fatal("默认应启用 story_skip")
	}

	path := filepath.Join(t.TempDir(), "zzz.yml")
	data := `
schema_version: 1
features:
  story_skip:
    enabled: false
    settings:
      mode: auto
      option: last
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadZZZ(path)
	if err != nil {
		t.Fatal(err)
	}
	feature := cfg.Features["story_skip"]
	if feature.Enabled {
		t.Error("enabled 应为 false")
	}
	if feature.Settings["mode"] != "auto" || feature.Settings["option"] != "last" {
		t.Errorf("settings 解析错误: %+v", feature.Settings)
	}
}

func TestLoadZZZInvalidFeatureName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zzz.yml")
	data := "schema_version: 1\nfeatures:\n  \" \":\n    enabled: true\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadZZZ(path); err == nil {
		t.Fatal("空 feature 名称应报错")
	}
}
