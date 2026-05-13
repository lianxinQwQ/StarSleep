// install 包提供 starsleep install 命令。
//
// 负责从分区格式化到首个可引导系统快照的完整安装流程。
package install

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/build"
	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/init_env"
	"starsleep/internal/util"
)

const (
	// DefaultRepoURL 是预设配置模板的 Git 仓库地址（GitHub Archive API 格式）
	DefaultRepoURL = "https://github.com/lianxin/starsleep-go"
	// TargetMount 是目标根分区的临时挂载点
	TargetMount = "/mnt/starsleep-target"
	// TargetBootMount 是目标 EFI 分区的临时挂载点
	TargetBootMount = "/mnt/starsleep-target/boot"
)

// Run 执行系统安装命令
func Run(args []string) {
	util.CheckRoot()

	var bootPart, rootPart, disk, profile, repoURL, entryName string
	force := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--boot":
			i++
			if i < len(args) {
				bootPart = args[i]
			}
		case "--root":
			i++
			if i < len(args) {
				rootPart = args[i]
			}
		case "--disk":
			i++
			if i < len(args) {
				disk = args[i]
			}
		case "--profile":
			i++
			if i < len(args) {
				profile = args[i]
			}
		case "--name":
			i++
			if i < len(args) {
				entryName = args[i]
			}
		case "--force":
			force = true
		case "--repo":
			i++
			if i < len(args) {
				repoURL = args[i]
			}
		default:
			util.Fatal(i18n.T("install.usage"))
		}
	}

	if profile == "" {
		profile = "dev"
	}
	if repoURL == "" {
		repoURL = DefaultRepoURL
	}

	// 整盘模式：交互式分区（如果提供了 --disk）
	if disk != "" {
		bootP, rootP := interactivePartition(disk)
		bootPart = bootP
		rootPart = rootP
	}

	if bootPart == "" {
		util.Fatal(i18n.T("install.missing.boot"))
	}
	if rootPart == "" {
		util.Fatal(i18n.T("install.missing.root"))
	}

	fmt.Println(i18n.T("install.separator"))
	fmt.Println(i18n.T("install.title"))
	fmt.Println(i18n.T("install.separator"))
	fmt.Println(i18n.T("install.profile", profile))
	fmt.Println(i18n.T("install.boot.partition", bootPart))
	fmt.Println(i18n.T("install.root.partition", rootPart))
	if force {
		fmt.Println(i18n.T("install.force.mode"))
	}
	fmt.Println(i18n.T("install.separator"))

	// ── 交互：询问 UEFI 固件启动菜单显示名称 ──
	if entryName == "" {
		entryName = askEntryName()
	}

	// ── 阶段 A: 格式化分区 ──
	formatPartitions(bootPart, rootPart, force)

	// ── 阶段 B: 挂载目标 ──
	mountTarget(bootPart, rootPart)

	// ── 阶段 C: 获取 UUID 并创建 Btrfs 子卷布局 ──
	rootUUID := createSubvolLayout(rootPart)

	// ── 阶段 D: 拉取预设配置 ──
	fetchProfile(profile, repoURL)

	// ── 阶段 E: 初始化工作目录 + 构建系统 ──
	runBuild()

	// ── 阶段 F: 复制构建产物到目标根子卷 ──
	copyProductToTarget(rootUUID)

	// ── 阶段 G: 生成 fstab ──
	generateFstab(rootUUID, bootPart)

	// ── 阶段 H: 初始化 systemd-boot ──
	mc, err := config.LoadMainConfig(config.DefaultConfigDir)
	snapshotName := "snapshot-initial"
	if err == nil {
		_ = mc
	}
	initBootloader(TargetBootMount, rootUUID, snapshotName, entryName)

	// ── 阶段 I: 创建共享目录 ──
	initSharedDirs()

	// ── 卸载并清理 ──
	util.Run("umount", "-R", TargetMount)

	// ── 打印安装摘要 ──
	printSummary(profile, bootPart, rootPart, rootUUID, snapshotName)
}

// mountTarget 挂载目标分区
func mountTarget(bootPart, rootPart string) {
	fmt.Println(i18n.T("install.mount.target", TargetMount))
	os.MkdirAll(TargetMount, 0o755)
	if err := util.Run("mount", rootPart, TargetMount); err != nil {
		util.Fatal(fmt.Sprintf("挂载根分区失败: %v", err))
	}

	fmt.Println(i18n.T("install.mount.boot", bootPart, TargetBootMount))
	os.MkdirAll(TargetBootMount, 0o755)
	if err := util.Run("mount", bootPart, TargetBootMount); err != nil {
		util.Fatal(fmt.Sprintf("挂载 EFI 分区失败: %v", err))
	}
}

// copyProductToTarget 将 flatDir 的内容复制到目标根子卷
func copyProductToTarget(rootUUID string) {
	fmt.Println(i18n.T("install.copy.product"))

	workDir := config.DefaultWorkDir
	flatDir := filepath.Join(workDir, "work/flat")

	// 先卸载当前 @ 子卷（如果有的话，重新挂载）
	subvolMount := filepath.Join(TargetMount, "@")
	// 确保子卷存在
	util.Run("btrfs", "subvolume", "create", subvolMount)

	// 挂载 @ 子卷到临时位置进行复制
	tmpMount := filepath.Join(TargetMount, ".tmp-root")
	os.MkdirAll(tmpMount, 0o755)
	util.Run("mount", "-o", fmt.Sprintf("subvol=@,compress=zstd"), "UUID="+rootUUID, tmpMount)

	// rsync 复制
	if err := util.Run("rsync", "-aAX", "--delete", flatDir+"/", tmpMount+"/"); err != nil {
		util.Fatal(fmt.Sprintf("复制构建产物失败: %v", err))
	}

	util.Run("umount", tmpMount)
	os.Remove(tmpMount)
	fmt.Println(i18n.T("install.copy.done"))
}

// runBuild 初始化工作目录并运行构建
func runBuild() {
	fmt.Println(i18n.T("install.init.workdir"))
	init_env.Run([]string{})

	fmt.Println(i18n.T("install.build.start"))
	build.Run([]string{})
	fmt.Println(i18n.T("install.build.done"))
}

// printSummary 打印安装摘要
func printSummary(profile, bootPart, rootPart, rootUUID, snapshotName string) {
	fmt.Println()
	fmt.Println(i18n.T("install.summary.title"))
	fmt.Println(i18n.T("install.summary.profile", profile))
	fmt.Println(i18n.T("install.summary.boot", bootPart))
	fmt.Println(i18n.T("install.summary.root", rootPart))
	fmt.Println(i18n.T("install.summary.uuid", rootUUID))
	fmt.Println(i18n.T("install.summary.snapshot", snapshotName))
	fmt.Println(i18n.T("install.summary.boot.entry", snapshotName))
	fmt.Println(i18n.T("install.summary.bottom"))
	fmt.Println()
	fmt.Println(i18n.T("install.done"))
	fmt.Println()
	fmt.Println(i18n.T("install.reboot.hint"))
	fmt.Println(i18n.T("install.reboot.cmd"))
}

// askEntryName 询问用户在 UEFI 固件启动菜单中的显示名称
func askEntryName() string {
	fmt.Println()
	fmt.Println(i18n.T("install.entry.name.prompt"))
	fmt.Print(i18n.T("install.entry.name.input"))
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		text = "StarSleep"
	}
	fmt.Printf(i18n.T("install.entry.name.confirm"), text)
	fmt.Println()
	return text
}

// confirmPrompt 等待用户输入 y/N 确认
func confirmPrompt(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(strings.ToLower(text))
	return text == "y" || text == "yes"
}

// initSharedDirs 在目标系统创建共享目录结构
func initSharedDirs() {
	fmt.Println(i18n.T("install.init.shared"))

	dirs := []string{
		filepath.Join(TargetMount, "home"),
		filepath.Join(TargetMount, "root"),
		filepath.Join(TargetMount, "var/log"),
		filepath.Join(TargetMount, "var/cache"),
		filepath.Join(TargetMount, "var/tmp"),
	}

	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}

	// 设置 /root 权限
	os.Chmod(filepath.Join(TargetMount, "root"), 0o700)
	// 设置 /var/tmp sticky bit
	os.Chmod(filepath.Join(TargetMount, "var/tmp"), 0o1777)

	fmt.Println(i18n.T("install.shared.done"))
}
