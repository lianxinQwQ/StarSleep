// util 包提供 StarSleep 全局使用的工具函数。
//
// 包括日志记录、命令执行、权限检查、挂载点检测和时间戳生成等。
package util

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"starsleep/internal/i18n"
)

// logFile 全局日志文件句柄
var logFile *os.File

// InitLog 初始化日志文件
//
// 在 logDir 下创建带时间戳的日志文件（如 build-20260307-120000.log）。
//
// @param logDir 日志目录路径
func InitLog(logDir string) {
	os.MkdirAll(logDir, 0o755)
	ts := time.Now().Format("20060102-150405")
	path := fmt.Sprintf("%s/build-%s.log", logDir, ts)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		logFile = f
	}
}

// CloseLog 关闭日志文件并释放资源
func CloseLog() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// LogMsg 输出带时间戳的日志消息到 stdout 和日志文件
//
// @param format 格式化字符串
// @param args 格式化参数
func LogMsg(format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] %s", ts, fmt.Sprintf(format, args...))
	fmt.Println(msg)
	if logFile != nil {
		fmt.Fprintln(logFile, msg)
	}
}

// FatalPanicMode 为 true 时 Fatal 抛出 panic 而非 os.Exit。
// 用于 build 逐层构建时捕获失败并回滚工作快照。
var FatalPanicMode bool

// FatalError 是 Fatal 在 panic 模式下抛出的错误类型
type FatalError struct{ Msg string }

func (e FatalError) Error() string { return e.Msg }

// Fatal 输出错误消息并退出程序
//
// 在 FatalPanicMode 为 true 时抛出 FatalError panic 而非 os.Exit，
// 允许调用方捕获并执行回滚逻辑。
//
// @param msg 错误消息字符串
// @throws FatalPanicMode 为 true 时抛出 FatalError
func Fatal(msg string) {
	if FatalPanicMode {
		panic(FatalError{Msg: msg})
	}
	fmt.Fprintf(os.Stderr, i18n.T("fatal.prefix"), msg)
	os.Exit(1)
}

// Run 执行外部命令，将标准输出/错误/输入透传到当前进程
//
// @param name 命令名称
// @param args 命令参数
// @return error 命令执行失败时返回错误
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunWithEnv 执行外部命令并附加额外的环境变量
//
// 在当前进程环境变量基础上追加 extraEnv 中的 KEY=VALUE 条目。
// 用于 chroot 类 helper 传递自定义环境变量给 arch-chroot。
//
// @param extraEnv 额外的环境变量切片（KEY=VALUE 格式）
// @param name 命令名称
// @param args 命令参数
// @return error 命令执行失败时返回错误
func RunWithEnv(extraEnv []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunSilent 执行外部命令，捕获并返回标准输出
//
// 与 Run 不同，标准输出不会打印到控制台。
//
// @param name 命令名称
// @param args 命令参数
// @return 命令输出字符串（已去除首尾空白）和可能的错误
func RunSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// CheckRoot 确认以 root 权限运行
//
// 如果当前用户不是 root，调用 Fatal 退出。
func CheckRoot() {
	if os.Getuid() != 0 {
		Fatal(i18n.T("need.root"))
	}
}

// IsMountpoint 检查路径是否为挂载点
//
// 通过调用 mountpoint -q 命令判断。
//
// @param path 要检查的路径
// @return 如果是挂载点返回 true
func IsMountpoint(path string) bool {
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}

// Timestamp 返回当前时间戳字符串
//
// 格式: YYYYMMDD-HHMMSS，用于快照命名和日志文件名。
//
// @return 时间戳字符串
func Timestamp() string {
	return time.Now().Format("20060102-150405")
}
