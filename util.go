package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var logFile *os.File

// initLog 初始化日志文件
func initLog(logDir string) {
	os.MkdirAll(logDir, 0o755)
	ts := time.Now().Format("20060102-150405")
	path := fmt.Sprintf("%s/build-%s.log", logDir, ts)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		logFile = f
	}
}

// logMsg 输出日志到 stdout 和日志文件
func logMsg(format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] %s", ts, fmt.Sprintf(format, args...))
	fmt.Println(msg)
	if logFile != nil {
		fmt.Fprintln(logFile, msg)
	}
}

// fatalPanicMode 为 true 时 fatal 抛出 panic 而非 os.Exit，
// 用于 build 逐层构建时捕获失败并回滚工作快照。
var fatalPanicMode bool

// fatalError 是 fatal 在 panic 模式下抛出的错误类型
type fatalError struct{ msg string }

func (e fatalError) Error() string { return e.msg }

// fatal 输出错误并退出（或在 panicMode 下抛出 panic）
func fatal(msg string) {
	if fatalPanicMode {
		panic(fatalError{msg: msg})
	}
	fmt.Fprintf(os.Stderr, "[StarSleep] 错误: %s\n", msg)
	os.Exit(1)
}

// run 执行命令，将输出透传到当前进程
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// runSilent 执行命令，捕获输出
func runSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// checkRoot 确认以 root 权限运行
func checkRoot() {
	if os.Getuid() != 0 {
		fatal("需要 root 权限，请使用 sudo 运行")
	}
}

// isMountpoint 检查路径是否为挂载点
func isMountpoint(path string) bool {
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}

// timestamp 返回当前时间戳
func timestamp() string {
	return time.Now().Format("20060102-150405")
}
