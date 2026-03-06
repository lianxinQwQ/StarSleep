// verify.go — starsleep verify 命令
//
// 展平一致性校验：将所有层通过 OverlayFS 叠加为只读合并视图，
// 与展平目录逐文件对比校验和，确认展平结果与 OverlayFS 合并语义完全一致。
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func cmdVerify(args []string) {
	checkRoot()

	flatDir := ""
	var layerDirs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--flat":
			i++
			if i < len(args) {
				flatDir = args[i]
			}
		case "--layers":
			i++
			for i < len(args) && !strings.HasPrefix(args[i], "--") {
				layerDirs = append(layerDirs, args[i])
				i++
			}
			i-- // for loop will increment
		default:
			fatal(T("verify.unknown.arg", args[i]))
		}
	}

	if flatDir == "" || len(layerDirs) == 0 {
		fatal(T("verify.usage"))
	}

	if !runVerify(flatDir, layerDirs) {
		os.Exit(1)
	}
}

// runVerify 执行展平一致性校验，返回是否通过
func runVerify(flatDir string, layerDirs []string) bool {
	fmt.Println(T("verify.separator"))
	fmt.Println(T("verify.flat.dir", flatDir))
	fmt.Println(T("verify.layer.count", len(layerDirs)))
	fmt.Println(T("verify.separator"))

	// 构建 lowerdir：最上层（最后的层）在左边
	parts := make([]string, len(layerDirs))
	for i, d := range layerDirs {
		parts[len(layerDirs)-1-i] = d
	}
	lower := strings.Join(parts, ":")

	// 创建临时挂载点
	mnt, err := os.MkdirTemp("/tmp", "starsleep-verify-mnt.")
	if err != nil {
		fmt.Fprintln(os.Stderr, T("verify.tmpdir.failed", err))
		return false
	}
	mounted := false
	defer func() {
		if mounted {
			syscall.Unmount(mnt, 0)
		}
		os.Remove(mnt)
	}()

	// 挂载只读 OverlayFS
	opts := "lowerdir=" + lower
	if err := syscall.Mount("overlay", mnt, "overlay", 0, opts); err != nil {
		fmt.Fprintln(os.Stderr, T("verify.mount.failed", err))
		return false
	}
	mounted = true

	fmt.Println(T("verify.mounted", mnt))
	fmt.Println(T("verify.lowerdir", lower))

	// 使用 rsync --dry-run --checksum 对比
	fmt.Println(T("verify.comparing"))

	cmd := exec.Command("rsync", "-anAX", "--checksum", "--delete", "--itemize-changes",
		mnt+"/", flatDir+"/")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	rsyncErr := cmd.Run()

	// 卸载
	syscall.Unmount(mnt, 0)
	mounted = false
	os.Remove(mnt)

	if rsyncErr != nil {
		// rsync 非零退出可能仅代表有差异，也可能是真错误
		// 检查是否有输出来判断
		if out.Len() == 0 {
			fmt.Fprintln(os.Stderr, T("verify.rsync.failed", rsyncErr))
			return false
		}
	}

	// 过滤有意义的差异（跳过纯目录时间戳变化）
	var realDiffs []string
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ".d..") {
			continue
		}
		realDiffs = append(realDiffs, line)
	}

	fmt.Println(T("verify.separator"))
	if len(realDiffs) == 0 {
		fmt.Println(T("verify.result.ok"))
		return true
	}

	fmt.Fprintln(os.Stderr, T("verify.diff.count", len(realDiffs)))
	limit := 50
	for i, d := range realDiffs {
		if i >= limit {
			fmt.Fprintln(os.Stderr, T("verify.diff.truncated", len(realDiffs)))
			break
		}
		fmt.Fprintln(os.Stderr, "  "+d)
	}
	fmt.Fprintln(os.Stderr, T("verify.separator"))
	fmt.Fprintln(os.Stderr, T("verify.result.fail"))
	return false
}
