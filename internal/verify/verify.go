// verify 包提供展平一致性校验功能。
//
// 将所有层目录以反序作为 lowerdir 挂载为只读 OverlayFS，
// 使用 rsync --checksum 对比合并视图与展平目录。
package verify

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// Run 执行展平一致性校验命令（解析参数并分发）
func Run(args []string) {
	util.CheckRoot()
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
			i--
		default:
			util.Fatal(i18n.T("verify.unknown.arg", args[i]))
		}
	}
	if flatDir == "" || len(layerDirs) == 0 {
		util.Fatal(i18n.T("verify.usage"))
	}
	if !RunVerify(flatDir, layerDirs) {
		os.Exit(1)
	}
}

// RunVerify 执行展平一致性校验
func RunVerify(flatDir string, layerDirs []string) bool {
	fmt.Println(i18n.T("verify.separator"))
	fmt.Println(i18n.T("verify.flat.dir", flatDir))
	fmt.Println(i18n.T("verify.layer.count", len(layerDirs)))
	fmt.Println(i18n.T("verify.separator"))

	parts := make([]string, len(layerDirs))
	for i, d := range layerDirs {
		parts[len(layerDirs)-1-i] = d
	}
	lower := strings.Join(parts, ":")

	mnt, err := os.MkdirTemp("/tmp", "starsleep-verify-mnt.")
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("verify.tmpdir.failed", err))
		return false
	}
	mounted := false
	defer func() {
		if mounted {
			syscall.Unmount(mnt, 0)
		}
		os.Remove(mnt)
	}()

	opts := "lowerdir=" + lower
	if err := syscall.Mount("overlay", mnt, "overlay", 0, opts); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("verify.mount.failed", err))
		return false
	}
	mounted = true

	fmt.Println(i18n.T("verify.mounted", mnt))
	fmt.Println(i18n.T("verify.lowerdir", lower))
	fmt.Println(i18n.T("verify.comparing"))

	cmd := exec.Command("rsync", "-anAX", "--checksum", "--delete", "--itemize-changes",
		mnt+"/", flatDir+"/")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	rsyncErr := cmd.Run()

	syscall.Unmount(mnt, 0)
	mounted = false
	os.Remove(mnt)

	if rsyncErr != nil {
		if out.Len() == 0 {
			fmt.Fprintln(os.Stderr, i18n.T("verify.rsync.failed", rsyncErr))
			return false
		}
	}

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

	fmt.Println(i18n.T("verify.separator"))
	if len(realDiffs) == 0 {
		fmt.Println(i18n.T("verify.result.ok"))
		return true
	}
	fmt.Fprintln(os.Stderr, i18n.T("verify.diff.count", len(realDiffs)))
	limit := 50
	for i, d := range realDiffs {
		if i >= limit {
			fmt.Fprintln(os.Stderr, i18n.T("verify.diff.truncated", len(realDiffs)))
			break
		}
		fmt.Fprintln(os.Stderr, "  "+d)
	}
	fmt.Fprintln(os.Stderr, i18n.T("verify.separator"))
	fmt.Fprintln(os.Stderr, i18n.T("verify.result.fail"))
	return false
}
