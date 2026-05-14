// logger/logger.go
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var L zerolog.Logger

// Init 初始化日志
// logDir: 日志目录，如 "./logs"
func Init(logDir string) error {
	// 确保目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir failed: %w", err)
	}

	// 生成文件名：2026-03-16_log.log
	filename := time.Now().Format("2006-01-02") + "_log.log"
	filepath := filepath.Join(logDir, filename)

	// 打开文件（追加模式）
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("open log file failed: %w", err)
	}

	// 同时输出到文件和控制台（可选）
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05",
	}

	// 多输出：文件 + 控制台
	multi := zerolog.MultiLevelWriter(file, consoleWriter)

	// 初始化全局 logger
	L = zerolog.New(multi).
		With().
		Timestamp().
		Caller().
		Logger()

	// 设置 zerolog 全局 log 变量（可选，兼容 log.Info() 写法）
	log.Logger = L

	return nil
}

// 快捷方法封装
func Debug() *zerolog.Event { return L.Debug() }
func Info() *zerolog.Event  { return L.Info() }
func Warn() *zerolog.Event  { return L.Warn() }
func Error() *zerolog.Event { return L.Error() }
func Fatal() *zerolog.Event { return L.Fatal() }

// 带消息的快捷方法
func Debugf(msg string) { L.Debug().Msg(msg) }
func Infof(msg string)  { L.Info().Msg(msg) }
func Warnf(msg string)  { L.Warn().Msg(msg) }
func Errorf(msg string) { L.Error().Msg(msg) }
func Fatalf(msg string) { L.Fatal().Msg(msg) }