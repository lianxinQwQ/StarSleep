// cmd_verify.go — starsleep verify 命令
//
// 与展平目录逐文件对比校验和，确认展平结果与 OverlayFS 合并语义完全一致。
//
// 校验流程:
//  1. 将所有层目录以反序作为 lowerdir 挂载为只读 OverlayFS
//  2. 使用 rsync --checksum 对比合并视图与展平目录
//  3. 过滤仅目录时间戳差异，报告真实差异数量
package main

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

// cmdVerify 执行展平一致性校验命令
//
// @param args 命令行参数，必须包含:
//   - --flat <展平目录>: 展平子卷路径
//   - --layers <层1> [层2...]: 层目录列表
//
// @throws 参数不完整时打印用法并退出
func cmdVerify(args []string) {
	util.CheckRoot()
	// 解析 --flat 和 --layers 参数
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
	if !runVerify(flatDir, layerDirs) {
		os.Exit(1)
	}
}

// runVerify 执行展平一致性校验
//
// 将所有层目录以反序挂载为只读 OverlayFS，然后通过 rsync
// 逾行对比合并视图和展平目录的校验和，过滤无关的目录
// 时间戳差异后报告最终结果。
//
// @param flatDir 展平子卷路径
// @param layerDirs 层目录列表（按构建顺序）
// @return 校验是否通过
func runVerify(flatDir string, layerDirs []string) bool {
	fmt.Println(i18n.T("verify.separator"))
	fmt.Println(i18n.T("verify.flat.dir", flatDir))
	fmt.Println(i18n.T("verify.layer.count", len(layerDirs)))
	fmt.Println(i18n.T("verify.separator"))
	// 将层目录倒序排列，组装为 OverlayFS 的 lowerdir 参数
	// 最后一层优先级最高，对应 OverlayFS 的语义
	parts := make([]string, len(layerDirs))
	for i, d := range layerDirs {
		parts[len(layerDirs)-1-i] = d
	}
	lower := strings.Join(parts, ":")
	// 创建临时挂载点并挂载只读 OverlayFS
	mnt, err := os.MkdirTemp("/tmp", "starsleep-verify-mnt.")
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("verify.tmpdir.failed", err))
		return false
	}
	mounted := false
	// 清理函数：确保退出时卸载并删除临时目录
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
	// 使用 rsync 逾行对比:
	// -a: 归档模式  -n: 干跑（不实际复制）  -A: ACL  -X: 扩展属性
	// --checksum: 基于校验和比较  --delete: 检测多余文件
	// --itemize-changes: 输出详细差异编码
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
	// 过滤 rsync 输出：忽略仅目录时间戳的差异（.d.. 前缀）
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
