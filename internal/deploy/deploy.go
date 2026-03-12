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

// Run 执行快照部署命令
func Run(args []string) {
	util.CheckRoot()

	workDir := config.DefaultWorkDir

	if len(args) > 0 {
		switch args[0] {
		case "--list":
			listBootEntries()
			return
		case "--remove":
			if len(args) < 2 {
				util.Fatal(i18n.T("flatten.remove.usage"))
			}
			removeBootEntry(args[1])
			return
		}
	}

	var target string
	if len(args) == 0 {
		target = filepath.Join(workDir, "snapshots/latest")
	} else {
		target = args[0]
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
	DeploySnapshot(target, snapName)

	fmt.Println(i18n.T("deploy.separator"))
	fmt.Println(i18n.T("deploy.done"))
	fmt.Println()
	fmt.Println(i18n.T("deploy.reboot.hint"))
	fmt.Println(i18n.T("deploy.reboot.entry", EntryTitle, snapName))
}

// DeploySnapshot 将快照的内核和 initramfs 部署到 boot 分区并生成引导条目
func DeploySnapshot(target, snapName string) {
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

	bootDest := filepath.Join(BootDir, snapName)
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
	confPath := filepath.Join(EntryDir, confName)

	os.MkdirAll(EntryDir, 0o755)
	rootUUID := detectRootUUID()
	entry := fmt.Sprintf(`title %s - %s
linux /starsleep/%s/vmlinuz
%s
options root="UUID=%s" rootflags=subvol=/%s/%s %s
`,
		EntryTitle, snapName,
		snapName,
		strings.Join(initrdLines, "\n"),
		rootUUID, SubvolPrefix, snapName, KernelOpts)

	if err := os.WriteFile(confPath, []byte(entry), 0o644); err != nil {
		util.Fatal(i18n.T("write.entry.failed", err))
	}

	fmt.Println(i18n.T("deploy.entry.generated", confPath))
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
