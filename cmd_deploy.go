// cmd_deploy.go — starsleep flatten 命令
//
// 将 StarSleep 快照的内核和 initramfs 复制到 ESP 分区，
// 并生成 systemd-boot 引导条目。
//
// 支持的操作:
//   - 部署快照到引导分区（默认或指定快照）
//   - --list: 列出所有已部署的引导条目
//   - --remove <名称>: 删除指定引导条目
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

const (
	// bootDir ESP 分区上 StarSleep 启动文件存放目录
	bootDir = "/boot/starsleep"
	// entryDir systemd-boot 引导条目配置目录
	entryDir = "/boot/loader/entries"
	// subvolPrefix Btrfs 子卷前缀，用于引导参数中的 rootflags
	subvolPrefix = "starsleep/snapshots"
	// entryTitle 引导条目标题
	entryTitle = "StarSleep"
	// rootLabel 根分区卷标
	rootLabel = "arkane_root"
	// kernelOpts 内核启动参数
	kernelOpts = "lsm=landlock,lockdown,yama,integrity,apparmor,bpf quiet splash loglevel=3 systemd.show_status=auto rd.udev.log_level=3 rw"
)

// cmdFlatten 执行快照部署命令
//
// 将指定快照（或 latest）的内核和 initramfs 部署到 ESP 分区，
// 并生成 systemd-boot 引导条目。
//
// @param args 命令行参数列表，支持:
//   - --list: 列出已部署的引导条目
//   - --remove <名称>: 删除指定引导条目
//   - <快照路径>: 要部署的快照（默认: snapshots/latest）
func cmdFlatten(args []string) {
	util.CheckRoot()

	workDir := defaultWorkDir

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
		// 未指定快照时使用 latest 符号链接
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
	deploySnapshot(target, snapName)

	fmt.Println(i18n.T("deploy.separator"))
	fmt.Println(i18n.T("deploy.done"))
	fmt.Println()
	fmt.Println(i18n.T("deploy.reboot.hint"))
	fmt.Println(i18n.T("deploy.reboot.entry", entryTitle, snapName))
}

// deploySnapshot 将快照的内核和 initramfs 部署到 boot 分区并生成引导条目
//
// 操作步骤:
//  1. 查找快照内的 vmlinuz-* 和 initramfs-*.img
//  2. 复制到 ESP 分区的快照专属目录
//  3. 生成 systemd-boot 引导条目配置文件
//
// @param target 快照目录的绝对路径
// @param snapName 快照名称（用于引导条目命名）
// @throws 找不到内核或 initramfs 时调用 Fatal 退出
func deploySnapshot(target, snapName string) {
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

	// 生成 systemd-boot 引导条目配置文件
	confName := fmt.Sprintf("starsleep-%s.conf", snapName)
	confPath := filepath.Join(entryDir, confName)

	os.MkdirAll(entryDir, 0o755)
	entry := fmt.Sprintf(`title %s - %s
linux /starsleep/%s/vmlinuz
%s
options root="LABEL=%s" rootflags=subvol=/%s/%s %s
`,
		entryTitle, snapName,
		snapName,
		strings.Join(initrdLines, "\n"),
		rootLabel, subvolPrefix, snapName, kernelOpts)

	if err := os.WriteFile(confPath, []byte(entry), 0o644); err != nil {
		util.Fatal(i18n.T("write.entry.failed", err))
	}

	fmt.Println(i18n.T("deploy.entry.generated", confPath))
}

// listBootEntries 列出所有已部署的 StarSleep 引导条目
//
// 扫描 entryDir 中的 starsleep-*.conf 文件，解析每个条目的 title 并打印。
func listBootEntries() {
	fmt.Println(i18n.T("deploy.list.header"))
	found := false
	entries, _ := filepath.Glob(filepath.Join(entryDir, "starsleep-*.conf"))
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

// removeBootEntry 删除指定快照的引导条目和启动文件
//
// @param snapName 快照名称
func removeBootEntry(snapName string) {
	conf := filepath.Join(entryDir, fmt.Sprintf("starsleep-%s.conf", snapName))
	bootSnap := filepath.Join(bootDir, snapName)

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

// findFile 在指定目录下按 glob 模式查找第一个匹配的文件
//
// @param dir 搜索目录
// @param pattern glob 匹配模式（如 "vmlinuz-*"）
// @return 匹配的文件路径，未找到则返回空字符串
func findFile(dir, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// copyFile 复制文件内容
//
// 将源文件完整读入内存后写入目标路径。
//
// @param src 源文件路径
// @param dst 目标文件路径
// @throws 读写失败时调用 Fatal 退出
func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		util.Fatal(i18n.T("read.file.failed", src, err))
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		util.Fatal(i18n.T("write.file.failed", dst, err))
	}
}
