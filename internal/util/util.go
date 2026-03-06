package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"starsleep/internal/i18n"
)

var logFile *os.File

// InitLog 初始化日志文件
func InitLog(logDir string) {
	os.MkdirAll(logDir, 0o755)
	ts := time.Now().Format("20060102-150405")
	path := fmt.Sprintf("%s/build-%s.log", logDir, ts)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		logFile = f
	}
}

// CloseLog 关闭日志文件
func CloseLog() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// LogMsg 输出日志到 stdout 和日志文件
func LogMsg(format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] %s", ts, fmt.Sprintf(format, args...))
	fmt.Println(msg)
	if logFile != nil {
		fmt.Fprintln(logFile, msg)
	}
}

// FatalPanicMode 为 true 时 Fatal 抛出 panic 而非 os.Exit，
// 用于 build 逐层构建时捕获失败并回滚工作快照。
var FatalPanicMode bool

// FatalError 是 Fatal 在 panic 模式下抛出的错误类型
type FatalError struct{ Msg string }

func (e FatalError) Error() string { return e.Msg }

// Fatal 输出错误并退出（或在 PanicMode 下抛出 panic）
func Fatal(msg string) {
	if FatalPanicMode {
		panic(FatalError{Msg: msg})
	}
	fmt.Fprintf(os.Stderr, i18n.T("fatal.prefix"), msg)
	os.Exit(1)
}

// Run 执行命令，将输出透传到当前进程
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunSilent 执行命令，捕获输出
func RunSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// CheckRoot 确认以 root 权限运行
func CheckRoot() {
	if os.Getuid() != 0 {
		Fatal(i18n.T("need.root"))
	}
}

// IsMountpoint 检查路径是否为挂载点
func IsMountpoint(path string) bool {
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}

// Timestamp 返回当前时间戳
func Timestamp() string {
	return time.Now().Format("20060102-150405")
}
