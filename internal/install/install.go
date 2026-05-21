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
	"starsleep/internal/deploy"
	"starsleep/internal/i18n"
	"starsleep/internal/init_env"
	"starsleep/internal/util"
)

const (
	// DefaultRepoURL 是预设配置模板的 Git 仓库地址（GitHub Archive API 格式）
	DefaultRepoURL = "https://github.com/lianxinQwQ/StarSleep"
	// TargetMount 是目标根分区的临时挂载点
	TargetMount = "/mnt/starsleep-target"
	// TargetBootMount 是目标 EFI 分区的临时挂载点
	TargetBootMount = "/mnt/starsleep-target/boot"
	// TargetStarsleepMount 是目标 starsleep 子卷的临时挂载点
	TargetStarsleepMount = "/mnt/starsleep-target/starsleep"
	// TargetConfigDir 是目标 starsleep 子卷下的配置目录
	TargetConfigDir = "/mnt/starsleep-target/starsleep/config"
)

// Run 执行系统安装命令
func Run(args []string) {
	util.CheckRoot()

	var bootPart, rootPart, disk, profile, repoURL, entryName, branch string
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
		case "--branch":
			i++
			if i < len(args) {
				branch = args[i]
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

	if repoURL == "" {
		repoURL = DefaultRepoURL
	}

	// ── 交互式参数补全 ──
	bootPart, rootPart, disk, profile, entryName = askMissingOptions(bootPart, rootPart, disk, profile, entryName)

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

	// ── 挂载 starsleep 子卷 ──
	mountStarsleepSubvol(rootUUID)

	// ── 阶段 D: 拉取预设配置 ──
	if err := init_env.InitializeWorkspace(init_env.Options{WorkDir: TargetStarsleepMount}); err != nil {
		util.Fatal(err.Error())
	}
	fetchProfile(profile, repoURL, branch)

	// ── 阶段 E: 初始化工作目录 + 构建系统 ──
	runBuild()
	snapshotPath, snapshotName := latestSnapshot(TargetStarsleepMount)

	initBootloader(TargetBootMount, rootUUID, snapshotName, entryName)

	deploy.RunWithOptions([]string{snapshotPath}, deploy.Options{
		ConfigDir:    TargetConfigDir,
		WorkDir:      TargetStarsleepMount,
		BootDir:      filepath.Join(TargetBootMount, "starsleep"),
		EntryDir:     filepath.Join(TargetBootMount, "loader", "entries"),
		RootUUID:     rootUUID,
		EntryTitle:   entryName,
		SubvolPrefix: "starsleep/snapshots",
	})

	// ── 阶段 F: 生成 fstab ──
	generateFstab(snapshotPath, rootUUID, bootPart)

	// ── 卸载并清理 ──
	util.Run("umount", TargetStarsleepMount)
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

// mountStarsleepSubvol 挂载目标的 starsleep 子卷供构建使用
func mountStarsleepSubvol(rootUUID string) {
	fmt.Println(i18n.T("install.mount.starsleep"))
	os.MkdirAll(TargetStarsleepMount, 0o755)
	if err := util.Run("mount", "-o", "subvol=starsleep,compress=zstd", "UUID="+rootUUID, TargetStarsleepMount); err != nil {
		util.Fatal(fmt.Sprintf("挂载 starsleep 子卷失败: %v", err))
	}
}

// runBuild 初始化工作目录并运行构建（在目标 starsleep 子卷上）
func runBuild() {
	fmt.Println(i18n.T("install.init.workdir"))

	// 检查必要的外部工具
	fmt.Println(i18n.T("install.build.start"))
	build.RunWithOptions(nil, build.Options{
		ConfigDir:   TargetConfigDir,
		WorkDir:     TargetStarsleepMount,
		SnapshotDir: filepath.Join(TargetStarsleepMount, "snapshots"),
		PkgCache:    filepath.Join(TargetStarsleepMount, "shared/pacman-cache"),
		ParuCache:   filepath.Join(TargetStarsleepMount, "shared/paru-cache"),
	})
	fmt.Println(i18n.T("install.build.done"))
}

func latestSnapshot(workDir string) (string, string) {
	latestLink := filepath.Join(workDir, "snapshots", "latest")
	target, err := os.Readlink(latestLink)
	if err != nil {
		util.Fatal(fmt.Sprintf("读取最新快照失败: %v", err))
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(latestLink), target)
	}
	return target, filepath.Base(target)
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

// askMissingOptions 对未通过命令行指定的选项进行交互式询问
func askMissingOptions(bootPart, rootPart, disk, profile, entryName string) (string, string, string, string, string) {
	// 如果都给了，直接返回
	if bootPart != "" && rootPart != "" && profile != "" && entryName != "" {
		return bootPart, rootPart, disk, profile, entryName
	}

	fmt.Println(i18n.T("install.interactive.header"))
	fmt.Println(i18n.T("install.separator"))

	// ── 分区选择 ──
	if bootPart == "" || rootPart == "" {
		bootPart, rootPart = askPartition(disk)
	}

	// ── profile 选择 ──
	if profile == "" {
		profile = askProfile()
	}

	// ── 名称 ──
	if entryName == "" {
		entryName = askEntryName()
	}

	fmt.Println(i18n.T("install.separator"))
	return bootPart, rootPart, disk, profile, entryName
}

// askPartition 交互式询问分区选择方式，返回 boot 和 root 分区路径
func askPartition(disk string) (string, string) {
	if disk != "" {
		return interactivePartition(disk)
	}

	// 先问选择方式
	fmt.Println(i18n.T("install.partition.method"))
	fmt.Println("  1) " + i18n.T("install.partition.method.disk"))
	fmt.Println("  2) " + i18n.T("install.partition.method.manual"))
	fmt.Print(i18n.T("install.partition.method.select"))

	var method int
	for {
		fmt.Scanf("%d", &method)
		if method == 1 || method == 2 {
			break
		}
		fmt.Print(i18n.T("install.partition.method.retry"))
		// 清空 stdin 残余
		reader := bufio.NewReader(os.Stdin)
		reader.ReadString('\n')
	}

	switch method {
	case 1:
		disks, err := detectDisks()
		if err != nil {
			util.Fatal(err.Error())
		}
		if len(disks) == 0 {
			util.Fatal(i18n.T("install.no.disk"))
		}
		fmt.Println(i18n.T("install.disk.select"))
		for i, d := range disks {
			fmt.Printf("  %d) %s (%s)\n", i+1, "/dev/"+d.Name, d.Size)
		}
		fmt.Print(i18n.T("install.disk.select.prompt"))
		var idx int
		fmt.Scanf("%d", &idx)
		if idx < 1 || idx > len(disks) {
			util.Fatal("无效的磁盘选择")
		}
		return interactivePartition("/dev/" + disks[idx-1].Name)
	case 2:
		fmt.Print(i18n.T("install.partition.enter.boot"))
		reader := bufio.NewReader(os.Stdin)
		boot, _ := reader.ReadString('\n')
		boot = strings.TrimSpace(boot)

		fmt.Print(i18n.T("install.partition.enter.root"))
		root, _ := reader.ReadString('\n')
		root = strings.TrimSpace(root)

		if boot == "" || root == "" {
			util.Fatal(i18n.T("install.missing.partition"))
		}
		return boot, root
	}
	return "", ""
}

// askProfile 交互式询问预设配置
func askProfile() string {
	fmt.Println(i18n.T("install.profile.select"))
	fmt.Println("  1) minimal  - " + i18n.T("install.profile.minimal.desc"))
	fmt.Println("  2) gnome    - " + i18n.T("install.profile.gnome.desc"))
	fmt.Println("  3) dev      - " + i18n.T("install.profile.dev.desc"))
	fmt.Print(i18n.T("install.profile.select.prompt"))

	var choice int
	for {
		fmt.Scanf("%d", &choice)
		if choice >= 1 && choice <= 3 {
			break
		}
		fmt.Print(i18n.T("install.profile.select.retry"))
		reader := bufio.NewReader(os.Stdin)
		reader.ReadString('\n')
	}

	switch choice {
	case 1:
		return "minimal"
	case 2:
		return "gnome"
	default:
		return "dev"
	}
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
