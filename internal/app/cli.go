package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// Version 是当前版本号；发布构建可用 -ldflags "-X ...Version=x.y.z" 覆盖。
var Version = "0.1.0"

// RunCLI 执行命令行入口，返回进程退出码。
func RunCLI(args []string) int {
	root := &cobra.Command{
		Use:     "skip-all",
		Short:   "跳过游戏剧情、CG 和对话",
		Version: Version,
	}
	var dataDir string
	root.PersistentFlags().StringVar(&dataDir, "data-dir", "", "数据目录（默认 %APPDATA%/skip-all）")
	root.AddCommand(newListCommand(&dataDir), newRunCommand(&dataDir), newDoctorCommand(&dataDir), newModelsCommand(&dataDir))
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		return 1
	}
	return 0
}

func newService(dataDir string) (*Service, error) {
	return NewService(Options{
		DataDir: dataDir,
		EventSink: func(name string, data any) {
			if name == "skipall:log" {
				if entry, ok := data.(LogEntry); ok {
					fmt.Fprintf(os.Stderr, "[%s] %s\n", entry.Level, entry.Message)
				}
			}
			if name == "skipall:model" {
				fmt.Fprintf(os.Stderr, "模型下载: %v\n", data)
			}
		},
	})
}

func newListCommand(dataDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出内置游戏与功能",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newService(*dataDir)
			if err != nil {
				return err
			}
			defer svc.Shutdown()
			data, err := json.MarshalIndent(svc.ListGames(), "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
}

func newRunCommand(dataDir *string) *cobra.Command {
	var gameID, featureID string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "运行一个跳过会话",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newService(*dataDir)
			if err != nil {
				return err
			}
			defer svc.Shutdown()
			var result ActionResult
			if strings.EqualFold(gameID, "all") {
				result = svc.StartAll()
			} else {
				result = svc.Start(gameID, featureID)
			}
			if !result.OK {
				return fmt.Errorf("%s", result.Error)
			}
			fmt.Fprintln(os.Stderr, "会话已启动，按 Ctrl+C 停止")
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			<-ctx.Done()
			svc.Stop()
			time.Sleep(300 * time.Millisecond)
			return nil
		},
	}
	cmd.Flags().StringVar(&gameID, "game", "all", "游戏 ID；all 表示所有未排除游戏")
	cmd.Flags().StringVar(&featureID, "feature", "story_skip", "功能 ID")
	cmd.Flags().Bool("headless", true, "无界面运行（CLI 模式）")
	return cmd
}

func newDoctorCommand(dataDir *string) *cobra.Command {
	var gameID string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "检查窗口、截图与 OCR",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newService(*dataDir)
			if err != nil {
				return err
			}
			defer svc.Shutdown()
			report, err := svc.Doctor(gameID)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&gameID, "game", "zzz", "游戏 ID")
	return cmd
}

func newModelsCommand(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{Use: "models", Short: "OCR 模型管理"}
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "检查 OCR 模型是否完整",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newService(*dataDir)
			if err != nil {
				return err
			}
			defer svc.Shutdown()
			if err := svc.EnsureModels(); err != nil {
				return err
			}
			fmt.Println("OCR 模型完整")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "下载并安装 OCR 模型",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newService(*dataDir)
			if err != nil {
				return err
			}
			defer svc.Shutdown()
			if err := svc.InstallModels(); err != nil {
				return err
			}
			fmt.Println("OCR 模型安装完成")
			return nil
		},
	})
	return cmd
}

// IsCLIInvocation 判断是否应按 CLI 模式启动。
func IsCLIInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	first := args[0]
	return !strings.HasPrefix(first, "-") || first == "--help" || first == "-h" || first == "--version"
}
