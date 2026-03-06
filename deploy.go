// deploy.go — starsleep flatten 命令
//
// 将 StarSleep 快照的内核和 initramfs 复制到 ESP 分区，
// 并生成 systemd-boot 引导条目。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	bootDir      = "/boot/starsleep"
	entryDir     = "/boot/loader/entries"
	subvolPrefix = "starsleep/snapshots"
	entryTitle   = "StarSleep"
	rootLabel    = "arkane_root"
	kernelOpts   = "lsm=landlock,lockdown,yama,integrity,apparmor,bpf quiet splash loglevel=3 systemd.show_status=auto rd.udev.log_level=3 rw"
)

func cmdFlatten(args []string) {
	checkRoot()

	workDir := defaultWorkDir

	// 参数解析
	if len(args) > 0 {
		switch args[0] {
		case "--list":
			listBootEntries()
			return
		case "--remove":
			if len(args) < 2 {
				fatal(T("flatten.remove.usage"))
			}
			removeBootEntry(args[1])
			return
		}
	}

	// 确定要部署的快照
	var target string
	if len(args) == 0 {
		target = filepath.Join(workDir, "snapshots/latest")
	} else {
		target = args[0]
	}

	// 解析软链接
	if fi, err := os.Lstat(target); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, err := os.Readlink(target)
		if err == nil {
			target = resolved
		}
	}

	// 尝试补全路径
	if _, err := os.Stat(target); err != nil {
		full := filepath.Join(workDir, "snapshots", target)
		if _, err2 := os.Stat(full); err2 == nil {
			target = full
		} else {
			fatal(T("snapshot.not.exist", target))
		}
	}

	snapName := filepath.Base(target)
	snapBoot := filepath.Join(target, "boot")

	if _, err := os.Stat(snapBoot); err != nil {
		fatal(T("boot.not.found", snapBoot))
	}

	// 查找内核和 initramfs
	vmlinuz := findFile(snapBoot, "vmlinuz-*")
	initramfs := findFile(snapBoot, "initramfs-*.img")

	if vmlinuz == "" {
		fatal(T("kernel.not.found", snapBoot))
	}
	if initramfs == "" {
		fatal(T("initramfs.not.found", snapBoot))
	}

	fmt.Println(T("deploy.separator"))
	fmt.Println(T("deploy.snapshot", snapName))
	fmt.Println(T("deploy.kernel", filepath.Base(vmlinuz)))
	fmt.Println(T("deploy.initramfs", filepath.Base(initramfs)))
	fmt.Println(T("deploy.separator"))

	// 复制 boot 文件到 ESP
	bootDest := filepath.Join(bootDir, snapName)
	os.MkdirAll(bootDest, 0o755)

	copyFile(vmlinuz, filepath.Join(bootDest, "vmlinuz"))
	copyFile(initramfs, filepath.Join(bootDest, "initramfs-linux.img"))
	fmt.Println(T("deploy.boot.copied", bootDest))

	// 构建 initrd 列表
	var initrdLines []string
	for _, ucode := range []string{"/boot/amd-ucode.img", "/boot/intel-ucode.img"} {
		if _, err := os.Stat(ucode); err == nil {
			initrdLines = append(initrdLines, fmt.Sprintf("initrd /%s", filepath.Base(ucode)))
		}
	}
	initrdLines = append(initrdLines,
		fmt.Sprintf("initrd /starsleep/%s/initramfs-linux.img", snapName))

	// 生成 systemd-boot 引导条目
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
		fatal(T("write.entry.failed", err))
	}

	fmt.Println(T("deploy.entry.generated", confPath))
	fmt.Println(T("deploy.separator"))
	fmt.Println(T("deploy.done"))
	fmt.Println()
	fmt.Println(T("deploy.reboot.hint"))
	fmt.Println(T("deploy.reboot.entry", entryTitle, snapName))
}

// listBootEntries 列出已部署的引导条目
func listBootEntries() {
	fmt.Println(T("deploy.list.header"))
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
		fmt.Println(T("deploy.list.empty"))
	}
}

// removeBootEntry 移除引导条目
func removeBootEntry(snapName string) {
	conf := filepath.Join(entryDir, fmt.Sprintf("starsleep-%s.conf", snapName))
	bootSnap := filepath.Join(bootDir, snapName)

	if _, err := os.Stat(conf); err == nil {
		os.Remove(conf)
		fmt.Println(T("deploy.removed.entry", conf))
	} else {
		fmt.Println(T("deploy.entry.not.exist", conf))
	}

	if _, err := os.Stat(bootSnap); err == nil {
		os.RemoveAll(bootSnap)
		fmt.Println(T("deploy.removed.boot", bootSnap))
	}
}

// findFile 在目录中按 glob 模式查找第一个匹配文件
func findFile(dir, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// copyFile 复制文件
func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		fatal(T("read.file.failed", src, err))
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		fatal(T("write.file.failed", dst, err))
	}
}
