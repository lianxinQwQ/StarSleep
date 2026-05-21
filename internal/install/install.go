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
	"starsleep/internal/util"

	"gopkg.in/yaml.v3"
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
	fetchProfile(profile, repoURL, branch)
	writeConfigMeta()

	// ── 阶段 E: 初始化工作目录 + 构建系统 ──
	runBuild()

	// ── 阶段 F: 复制构建产物到目标根子卷 ──
	copyProductToTarget(rootUUID)

	// ── 阶段 G: 生成 fstab ──
	generateFstab(rootUUID, bootPart)

	// ── 阶段 H: 初始化 systemd-boot ──
	mc, err := config.LoadMainConfig(TargetConfigDir)
	snapshotName := "snapshot-initial"
	if err == nil {
		_ = mc
	}
	initBootloader(TargetBootMount, rootUUID, snapshotName, entryName)

	// ── 阶段 I: 创建共享目录 ──
	initSharedDirs()

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
	// 预创建 starsleep 目录结构
	dirs := []string{
		filepath.Join(TargetStarsleepMount, "shared"),
		filepath.Join(TargetStarsleepMount, "shared/home"),
		filepath.Join(TargetStarsleepMount, "shared/pacman-cache"),
		filepath.Join(TargetStarsleepMount, "shared/paru-cache"),
		filepath.Join(TargetStarsleepMount, "shared/root"),
		filepath.Join(TargetStarsleepMount, "var"),
		filepath.Join(TargetStarsleepMount, "var/log"),
		filepath.Join(TargetStarsleepMount, "var/cache"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}
}

// writeConfigMeta 将目标路径写入 config.yaml 的 meta 段，让 build 使用目标路径
func writeConfigMeta() {
	configPath := filepath.Join(TargetConfigDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		util.Fatal("无法读取配置: " + err.Error())
	}

	updated, err := injectConfigMeta(data, installMetaConfig())
	if err != nil {
		util.Fatal("更新配置 meta 段失败: " + err.Error())
	}

	if err := os.WriteFile(configPath, updated, 0o644); err != nil {
		util.Fatal("写入配置 meta 段失败: " + err.Error())
	}
}

func installMetaConfig() config.MetaConfig {
	return config.MetaConfig{
		WorkDir:     TargetStarsleepMount,
		SnapshotDir: TargetStarsleepMount + "/snapshots",
		PkgCache:    TargetStarsleepMount + "/shared/pacman-cache",
		DBPath:      "var/lib/pacman",
	}
}

func injectConfigMeta(data []byte, meta config.MetaConfig) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config.yaml top level must be a mapping")
	}

	content := make([]*yaml.Node, 0, len(root.Content))
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "meta" {
			continue
		}
		content = append(content, root.Content[i], root.Content[i+1])
	}
	root.Content = append([]*yaml.Node{
		scalarNode("meta"),
		metaConfigNode(meta),
	}, content...)

	return yaml.Marshal(&doc)
}

func metaConfigNode(meta config.MetaConfig) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			scalarNode("work_dir"), scalarNode(meta.WorkDir),
			scalarNode("snapshot_dir"), scalarNode(meta.SnapshotDir),
			scalarNode("pkg_cache"), scalarNode(meta.PkgCache),
			scalarNode("db_path"), scalarNode(meta.DBPath),
		},
	}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// copyProductToTarget 将 flatDir 的内容复制到目标 @ 子卷
func copyProductToTarget(rootUUID string) {
	fmt.Println(i18n.T("install.copy.product"))

	flatDir := filepath.Join(TargetStarsleepMount, "work/flat")

	// 挂载 @ 子卷进行复制
	tmpMount := filepath.Join(TargetMount, ".tmp-root-@")
	os.MkdirAll(tmpMount, 0o755)
	if err := util.Run("mount", "-o", "subvol=@,compress=zstd", "UUID="+rootUUID, tmpMount); err != nil {
		util.Fatal(fmt.Sprintf("挂载 @ 子卷失败: %v", err))
	}

	if err := util.Run("rsync", "-aAX", "--delete", flatDir+"/", tmpMount+"/"); err != nil {
		util.Fatal(fmt.Sprintf("复制构建产物失败: %v", err))
	}

	util.Run("umount", tmpMount)
	os.Remove(tmpMount)
	fmt.Println(i18n.T("install.copy.done"))
}

// runBuild 初始化工作目录并运行构建（在目标 starsleep 子卷上）
func runBuild() {
	fmt.Println(i18n.T("install.init.workdir"))

	// 创建必要的目录结构
	dirs := []string{
		filepath.Join(TargetStarsleepMount, "layers"),
		filepath.Join(TargetStarsleepMount, "snapshots"),
		filepath.Join(TargetStarsleepMount, "work/merged"),
		filepath.Join(TargetStarsleepMount, "work/ovl_work"),
		filepath.Join(TargetStarsleepMount, "logs"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}

	// 创建 Btrfs 子卷（pacman-cache, paru-cache）
	for _, subvol := range []string{
		filepath.Join(TargetStarsleepMount, "shared/pacman-cache"),
		filepath.Join(TargetStarsleepMount, "shared/paru-cache"),
	} {
		if _, err := os.Stat(subvol); os.IsNotExist(err) {
			if err := util.Run("btrfs", "subvolume", "create", subvol); err != nil {
				util.Fatal(fmt.Sprintf("创建子卷 %s 失败: %v", subvol, err))
			}
		}
	}

	// 检查必要的外部工具
	fmt.Println(i18n.T("install.build.start"))
	build.Run([]string{"-c", TargetConfigDir})
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

// initSharedDirs 在目标系统 /starsleep/shared/ 下创建共享目录结构
func initSharedDirs() {
	fmt.Println(i18n.T("install.init.shared"))

	shared := filepath.Join(TargetStarsleepMount, "shared")
	dirs := []string{
		filepath.Join(shared, "home"),
		filepath.Join(shared, "root"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}
	os.Chmod(filepath.Join(shared, "root"), 0o700)

	fmt.Println(i18n.T("install.shared.done"))
}
