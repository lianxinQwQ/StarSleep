// files.go — 文件对比逻辑
//
// 基于最近一次构建的 Btrfs 快照创建临时副本，应用 inherit.list 继承列表后，
// 使用 rsync 与指定目标目录进行文件级差异对比。不需要重新构建系统。
//
// 流程:
//  1. 找到 latest 快照
//  2. 创建临时 Btrfs 快照
//  3. 应用 inherit.list（从宿主机复制继承文件到快照）
//  4. rsync 对比快照中对应子目录与目标目录
//  5. 清理临时快照
package compare

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// InheritApplier 继承列表应用回调函数类型
//
// 将 config.yaml 中 inherit 段定义的文件从宿主机 reflink 复制到快照中。
// 由调用方（main 包的 applyInheritList）提供具体实现。
type InheritApplier func(snapshotDir string)

// Files 执行文件对比模式
//
// 基于最近一次构建的快照（latest）创建临时 Btrfs 快照，
// 应用 inherit.list 继承列表后，与指定目标目录进行文件级 rsync 对比。
// 不需要重新构建系统，直接复用已有快照，快速高效。
//
// @param targetDir 要对比的目标目录路径
// @param workDir 工作目录（存放 snapshots 等子目录）
// @param applyInherit 继承列表应用函数
func Files(targetDir, workDir string, applyInherit InheritApplier) {
	// 验证目标目录存在
	fi, err := os.Stat(targetDir)
	if err != nil || !fi.IsDir() {
		util.Fatal(i18n.T("compare.target.not.dir", targetDir))
	}

	// ── 定位 latest 快照 ──
	latestLink := filepath.Join(workDir, "snapshots/latest")
	latestTarget, err := os.Readlink(latestLink)
	if err != nil {
		util.Fatal(i18n.T("compare.no.snapshot", err))
	}
	if _, err := os.Stat(latestTarget); err != nil {
		util.Fatal(i18n.T("compare.no.snapshot", err))
	}

	fmt.Println(i18n.T("compare.separator"))
	fmt.Println(i18n.T("compare.file.title"))
	fmt.Println(i18n.T("compare.file.target", targetDir))
	fmt.Println(i18n.T("compare.file.snapshot", latestTarget))
	fmt.Println(i18n.T("compare.separator"))

	// ── 创建临时快照 ──
	tmpSnapName := "compare-tmp-" + util.Timestamp()
	tmpSnapDir := filepath.Join(workDir, "snapshots", tmpSnapName)
	fmt.Println(i18n.T("compare.file.creating.snap", tmpSnapName))

	if err := util.Run("btrfs", "subvolume", "snapshot", latestTarget, tmpSnapDir); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	// 确保退出时清理临时快照
	defer snapshotCleanup(tmpSnapDir)

	// ── 应用 inherit.list ──
	fmt.Println(i18n.T("compare.file.applying.inherit"))
	applyInherit(tmpSnapDir)

	// ── rsync 对比快照与目标目录 ──
	// 如果目标是绝对路径（如 /etc/），对比快照中对应的子目录
	compareSrc := tmpSnapDir
	if filepath.IsAbs(targetDir) {
		compareSrc = filepath.Join(tmpSnapDir, targetDir)
	}

	// 确保快照中存在对应目录
	if fi, err := os.Stat(compareSrc); err != nil || !fi.IsDir() {
		util.Fatal(i18n.T("compare.src.not.dir", compareSrc))
	}

	fmt.Println(i18n.T("compare.separator"))
	fmt.Println(i18n.T("compare.file.comparing", compareSrc, targetDir))

	diffLines := byRsync(compareSrc, targetDir)

	// ── 输出结果 ──
	fmt.Println(i18n.T("compare.separator"))
	if len(diffLines) == 0 {
		fmt.Println(i18n.T("compare.file.match"))
	} else {
		fmt.Println(i18n.T("compare.file.diff.count", len(diffLines)))
		// 限制输出数量，避免终端过载
		limit := 100
		for idx, line := range diffLines {
			if idx >= limit {
				fmt.Println(i18n.T("compare.file.diff.truncated", len(diffLines)))
				break
			}
			fmt.Println("  " + line)
		}
	}
	fmt.Println(i18n.T("compare.separator"))
}

// byRsync 使用 rsync 对比两个目录的文件级差异
//
// 以 rsync 干跑模式（-n）对比源目录和目标目录，
// 通过 --itemize-changes 输出详细差异编码。
// 过滤仅目录时间戳差异（.d.. 前缀）的无关条目。
//
// @param srcDir 源目录（构建结果）
// @param dstDir 目标目录（对比基准）
// @return 真实差异行切片
func byRsync(srcDir, dstDir string) []string {
	// -a: 归档模式  -n: 干跑  -A: ACL  -X: 扩展属性
	// --checksum: 基于校验和  --delete: 检测多余文件
	// --itemize-changes: 输出详细变更编码
	cmd := exec.Command("rsync", "-anAX", "--checksum", "--delete", "--itemize-changes",
		srcDir+"/", dstDir+"/")
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	rsyncErr := cmd.Run()

	if rsyncErr != nil && out.Len() == 0 {
		fmt.Fprintln(os.Stderr, i18n.T("compare.rsync.failed", rsyncErr))
		return nil
	}

	// 过滤无关差异：仅目录时间戳的变化（.d.. 前缀）
	var realDiffs []string
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// .d.. 表示仅目录属性（通常是时间戳）变化，非实质差异
		if strings.HasPrefix(line, ".d..") {
			continue
		}
		realDiffs = append(realDiffs, line)
	}
	return realDiffs
}

// snapshotCleanup 清理对比模式使用的临时快照
//
// 优先通过 btrfs subvolume delete 删除 Btrfs 子卷，
// 若非子卷则回退到普通目录删除。
//
// @param snapDir 临时快照路径
func snapshotCleanup(snapDir string) {
	// 检查是否为 Btrfs 子卷（等价于 isBtrfsSubvolume 但内联以避免跨包依赖）
	if util.Run("btrfs", "subvolume", "show", snapDir) == nil {
		util.Run("btrfs", "subvolume", "delete", snapDir)
	} else {
		os.RemoveAll(snapDir)
	}
	fmt.Println(i18n.T("compare.cleanup.done"))
}
