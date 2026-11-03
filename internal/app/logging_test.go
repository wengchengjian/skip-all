package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// failingWriter 模拟 GUI 构建下的 os.Stdout：没有控制台，每次写都失败。
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("no console")
}

// TestLogWriterSurvivesConsoleFailure 验证控制台写入失败不会挡掉文件日志。
//
// 回归用例：原来用 io.MultiWriter(os.Stdout, logFile)，它遇到第一个错误就中断。
// GUI 构建（-H windowsgui）没有控制台，stdout 必然失败，导致日志文件一条都写不出来。
func TestLogWriterSurvivesConsoleFailure(t *testing.T) {
	var file bytes.Buffer
	w := logWriter{file: &file, console: failingWriter{}}

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write 返回错误: %v", err)
	}
	if got := file.String(); got != "hello\n" {
		t.Fatalf("文件没拿到日志: %q", got)
	}
}

// TestLogWriterWritesBoth 验证有控制台时两边都写。
func TestLogWriterWritesBoth(t *testing.T) {
	var file, console bytes.Buffer
	w := logWriter{file: &file, console: &console}
	if _, err := w.Write([]byte("hi\n")); err != nil {
		t.Fatal(err)
	}
	if file.String() != "hi\n" || console.String() != "hi\n" {
		t.Fatalf("file=%q console=%q", file.String(), console.String())
	}
}

// TestParseLogLevel 验证配置里的级别字符串都能解析。
func TestParseLogLevel(t *testing.T) {
	cases := map[string]string{
		"debug": "DEBUG", "info": "INFO", "warn": "WARN", "warning": "WARN",
		"error": "ERROR", "": "INFO", "unknown": "INFO",
	}
	for in, want := range cases {
		if got := parseLogLevel(in).String(); got != want {
			t.Errorf("parseLogLevel(%q)=%s，期望 %s", in, got, want)
		}
	}
	if !strings.EqualFold(parseLogLevel("warn").String(), "WARN") {
		t.Error("warn 级别解析错误")
	}
}
