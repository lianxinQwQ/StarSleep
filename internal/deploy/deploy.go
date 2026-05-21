// deploy 包 — starsleep deploy 命令
//
// 将 StarSleep 快照的内核和 initramfs 复制到 ESP 分区，
// 并生成 systemd-boot 引导条目。
package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

const (
	// BootDir ESP 分区上 StarSleep 启动文件存放目录
	BootDir = "/boot/starsleep"
	// EntryDir systemd-boot 引导条目配置目录
	EntryDir = "/boot/loader/entries"
	// SubvolPrefix Btrfs 子卷前缀
	SubvolPrefix = "starsleep/snapshots"
	// EntryTitle 引导条目标题
	EntryTitle = "StarSleep"
	// KernelOpts 内核启动参数
	KernelOpts = "lsm=landlock,lockdown,yama,integrity,apparmor,bpf quiet splash loglevel=3 systemd.show_status=auto rd.udev.log_level=3 rw"
)

type Options struct {
	ConfigDir       string
	WorkDir         string
	BootDir         string
	EntryDir        string
	RootUUID        string
	EntryTitle      string
	SubvolPrefix    string
	KernelOpts      string
	UseInheritStore bool
}

// Run 执行快照部署命令
func Run(args []string) {
	util.CheckRoot()
	RunWithOptions(args, Options{})
}

func RunWithOptions(args []string, opts Options) {
	configDir, remaining := config.ParseConfigFlags(config.DefaultConfigDir, args)
	if opts.ConfigDir != "" {
		configDir = opts.ConfigDir
	}

	workDir := config.DefaultWorkDir
	if opts.WorkDir != "" {
		workDir = opts.WorkDir
	}
	useInheritStore := false

	if len(remaining) > 0 {
		switch remaining[0] {
		case "--list":
			listBootEntries()
			return
		case "--remove":
			if len(remaining) < 2 {
				util.Fatal(i18n.T("flatten.remove.usage"))
			}
			removeBootEntry(remaining[1])
			return
		case "--use-inherit-store":
			useInheritStore = true
			remaining = remaining[1:]
		}
	}
	if opts.UseInheritStore {
		useInheritStore = true
	}

	var target string
	if len(remaining) == 0 {
		target = filepath.Join(workDir, "snapshots/latest")
	} else {
		target = remaining[0]
	}

	// 解析符号链接得到实际路径
	if fi, err := os.Lstat(target); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, err := os.Readlink(target)
		if err == nil {
			target = resolved
		}
	}

	// 如果 target 不是绝对路径且不存在，尝试在 snapshots 目录下查找
	if _, err := os.Stat(target); err != nil {
		full := filepath.Join(workDir, "snapshots", target)
		if _, err2 := os.Stat(full); err2 == nil {
			target = full
		} else {
			util.Fatal(i18n.T("snapshot.not.exist", target))
		}
	}

	snapName := filepath.Base(target)

	// 应用继承列表
	mc, err := config.LoadMainConfig(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}
	if useInheritStore {
		applyInheritFromStore(mc, configDir, target)
	} else {
		ApplyInheritList(mc, target)
	}

	DeploySnapshotWithOptions(target, snapName, opts)

	fmt.Println(i18n.T("deploy.separator"))
	fmt.Println(i18n.T("deploy.done"))
	fmt.Println()
	fmt.Println(i18n.T("deploy.reboot.hint"))
	entryTitle := EntryTitle
	if opts.EntryTitle != "" {
		entryTitle = opts.EntryTitle
	}
	fmt.Println(i18n.T("deploy.reboot.entry", entryTitle, snapName))
}

// DeploySnapshot 将快照的内核和 initramfs 部署到 boot 分区并生成引导条目
func DeploySnapshot(target, snapName string) {
	DeploySnapshotWithOptions(target, snapName, Options{})
}

func DeploySnapshotWithOptions(target, snapName string, opts Options) {
	snapBoot := filepath.Join(target, "boot")

	if _, err := os.Stat(snapBoot); err != nil {
		util.Fatal(i18n.T("boot.not.found", snapBoot))
	}

	vmlinuz := findFile(snapBoot, "vmlinuz-*")
	initramfs := findFile(snapBoot, "initramfs-*.img")

	if vmlinuz == "" {
		util.Fatal(i18n.T("kernel.not.found", snapBoot))
	}
	if initramfs == "" {
		util.Fatal(i18n.T("initramfs.not.found", snapBoot))
	}

	fmt.Println(i18n.T("deploy.separator"))
	fmt.Println(i18n.T("deploy.snapshot", snapName))
	fmt.Println(i18n.T("deploy.kernel", filepath.Base(vmlinuz)))
	fmt.Println(i18n.T("deploy.initramfs", filepath.Base(initramfs)))
	fmt.Println(i18n.T("deploy.separator"))

	bootDir := BootDir
	if opts.BootDir != "" {
		bootDir = opts.BootDir
	}
	entryDir := EntryDir
	if opts.EntryDir != "" {
		entryDir = opts.EntryDir
	}
	entryTitle := EntryTitle
	if opts.EntryTitle != "" {
		entryTitle = opts.EntryTitle
	}
	subvolPrefix := SubvolPrefix
	if opts.SubvolPrefix != "" {
		subvolPrefix = opts.SubvolPrefix
	}
	kernelOpts := KernelOpts
	if opts.KernelOpts != "" {
		kernelOpts = opts.KernelOpts
	}

	bootDest := filepath.Join(bootDir, snapName)
	os.MkdirAll(bootDest, 0o755)

	copyFile(vmlinuz, filepath.Join(bootDest, "vmlinuz"))
	copyFile(initramfs, filepath.Join(bootDest, "initramfs-linux.img"))
	fmt.Println(i18n.T("deploy.boot.copied", bootDest))

	// 检测并添加 CPU 微码更新（AMD/Intel）到 initrd 行
	var initrdLines []string
	for _, ucode := range []string{"/boot/amd-ucode.img", "/boot/intel-ucode.img"} {
		if _, err := os.Stat(ucode); err == nil {
			initrdLines = append(initrdLines, fmt.Sprintf("initrd /%s", filepath.Base(ucode)))
		}
	}
	initrdLines = append(initrdLines,
		fmt.Sprintf("initrd /starsleep/%s/initramfs-linux.img", snapName))

	confName := fmt.Sprintf("starsleep-%s.conf", snapName)
	confPath := filepath.Join(entryDir, confName)

	os.MkdirAll(entryDir, 0o755)
	rootUUID := opts.RootUUID
	if rootUUID == "" {
		rootUUID = detectRootUUID()
	}
	entry := bootEntryContent(Options{
		EntryTitle:   entryTitle,
		RootUUID:     rootUUID,
		SubvolPrefix: subvolPrefix,
		KernelOpts:   kernelOpts,
	}, snapName, strings.Join(initrdLines, "\n"))

	if err := os.WriteFile(confPath, []byte(entry), 0o644); err != nil {
		util.Fatal(i18n.T("write.entry.failed", err))
	}

	fmt.Println(i18n.T("deploy.entry.generated", confPath))
}

func bootEntryContent(opts Options, snapName, initrdLines string) string {
	return fmt.Sprintf(`title %s - %s
linux /starsleep/%s/vmlinuz
%s
options root="UUID=%s" rootflags=subvol=/%s/%s %s
`,
		opts.EntryTitle, snapName,
		snapName,
		initrdLines,
		opts.RootUUID, opts.SubvolPrefix, snapName, opts.KernelOpts)
}

func listBootEntries() {
	fmt.Println(i18n.T("deploy.list.header"))
	found := false
	entries, _ := filepath.Glob(filepath.Join(EntryDir, "starsleep-*.conf"))
	for _, conf := range entries {
		found = true
		name := filepath.Base(conf)
		name = strings.TrimSuffix(name, ".conf")
		name = strings.TrimPrefix(name, "starsleep-")

		title := ""
		data, err := os.ReadFile(conf)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "title ") {
					title = strings.TrimPrefix(line, "title ")
					break
				}
			}
		}
		fmt.Printf("  %s  (%s)\n", name, title)
	}
	if !found {
		fmt.Println(i18n.T("deploy.list.empty"))
	}
}

func removeBootEntry(snapName string) {
	conf := filepath.Join(EntryDir, fmt.Sprintf("starsleep-%s.conf", snapName))
	bootSnap := filepath.Join(BootDir, snapName)

	if _, err := os.Stat(conf); err == nil {
		os.Remove(conf)
		fmt.Println(i18n.T("deploy.removed.entry", conf))
	} else {
		fmt.Println(i18n.T("deploy.entry.not.exist", conf))
	}

	if _, err := os.Stat(bootSnap); err == nil {
		os.RemoveAll(bootSnap)
		fmt.Println(i18n.T("deploy.removed.boot", bootSnap))
	}
}

func findFile(dir, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		util.Fatal(i18n.T("read.file.failed", src, err))
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		util.Fatal(i18n.T("write.file.failed", dst, err))
	}
}

func detectRootUUID() string {
	if uuid, err := util.RunSilent("findmnt", "-n", "-o", "UUID", "/"); err == nil && uuid != "" {
		return uuid
	}

	source, err := util.RunSilent("findmnt", "-n", "-o", "SOURCE", "/")
	if err == nil && source != "" {
		uuid, blkidErr := util.RunSilent("blkid", "-s", "UUID", "-o", "value", source)
		if blkidErr == nil && uuid != "" {
			return uuid
		}
	}

	util.Fatal(i18n.T("root.uuid.not.found"))
	return ""
}

// ApplyInheritList 从当前宿主机复制继承路径到快照
func ApplyInheritList(mc *config.MainConfig, snapshotDir string) {
	paths := config.LoadInheritList(mc)
	if len(paths) == 0 {
		return
	}

	util.LogMsg(i18n.T("apply.inherit"), len(paths))
	for _, entry := range paths {
		fi, err := os.Stat(entry)
		if err != nil {
			util.LogMsg(i18n.T("inherit.path.missing"), entry)
			continue
		}

		dst := filepath.Join(snapshotDir, entry)
		os.MkdirAll(filepath.Dir(dst), 0o755)

		if fi.IsDir() {
			if err := util.Run("cp", "-ax", "--reflink=auto", entry, filepath.Dir(dst)+"/"); err != nil {
				util.LogMsg(i18n.T("copy.dir.failed"), entry, err)
			} else {
				util.LogMsg(i18n.T("inherit.dir"), entry)
			}
		} else {
			if err := util.Run("cp", "-a", "--reflink=auto", entry, dst); err != nil {
				util.LogMsg(i18n.T("copy.file.failed"), entry, err)
			} else {
				util.LogMsg(i18n.T("inherit.file"), entry)
			}
		}
	}
}

// applyInheritFromStore 从 configDir/inherit/ 目录复制继承文件到快照
func applyInheritFromStore(mc *config.MainConfig, configDir string, snapshotDir string) {
	storeDir := filepath.Join(configDir, config.InheritDir)
	if _, err := os.Stat(storeDir); err != nil {
		util.Fatal(i18n.T("inherit.store.not.found", storeDir))
	}

	paths := config.LoadInheritList(mc)
	if len(paths) == 0 {
		return
	}

	util.LogMsg(i18n.T("apply.inherit.store"), len(paths))
	for _, entry := range paths {
		storeSrc := filepath.Join(storeDir, entry)
		fi, err := os.Stat(storeSrc)
		if err != nil {
			util.LogMsg(i18n.T("inherit.store.missing"), entry)
			continue
		}

		dst := filepath.Join(snapshotDir, entry)
		os.MkdirAll(filepath.Dir(dst), 0o755)

		if fi.IsDir() {
			if err := util.Run("cp", "-ax", "--reflink=auto", storeSrc, filepath.Dir(dst)+"/"); err != nil {
				util.LogMsg(i18n.T("copy.dir.failed"), storeSrc, err)
			} else {
				util.LogMsg(i18n.T("inherit.dir"), entry)
			}
		} else {
			if err := util.Run("cp", "-a", "--reflink=auto", storeSrc, dst); err != nil {
				util.LogMsg(i18n.T("copy.file.failed"), storeSrc, err)
			} else {
				util.LogMsg(i18n.T("inherit.file"), entry)
			}
		}
	}
}
