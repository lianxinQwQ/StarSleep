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
				fatal("用法: starsleep flatten --remove <快照名称>")
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
			fatal("快照不存在: " + target)
		}
	}

	snapName := filepath.Base(target)
	snapBoot := filepath.Join(target, "boot")

	if _, err := os.Stat(snapBoot); err != nil {
		fatal("快照中未找到 boot 目录: " + snapBoot)
	}

	// 查找内核和 initramfs
	vmlinuz := findFile(snapBoot, "vmlinuz-*")
	initramfs := findFile(snapBoot, "initramfs-*.img")

	if vmlinuz == "" {
		fatal("未找到内核文件 (vmlinuz-*): " + snapBoot)
	}
	if initramfs == "" {
		fatal("未找到 initramfs (initramfs-*.img): " + snapBoot)
	}

	fmt.Println("[Deploy] ─────────────────────────────────────────────")
	fmt.Printf("[Deploy] 快照: %s\n", snapName)
	fmt.Printf("[Deploy] 内核: %s\n", filepath.Base(vmlinuz))
	fmt.Printf("[Deploy] Initramfs: %s\n", filepath.Base(initramfs))
	fmt.Println("[Deploy] ─────────────────────────────────────────────")

	// 复制 boot 文件到 ESP
	bootDest := filepath.Join(bootDir, snapName)
	os.MkdirAll(bootDest, 0o755)

	copyFile(vmlinuz, filepath.Join(bootDest, "vmlinuz"))
	copyFile(initramfs, filepath.Join(bootDest, "initramfs-linux.img"))
	fmt.Printf("[Deploy] 已复制启动文件到: %s/\n", bootDest)

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
		fatal(fmt.Sprintf("写入引导条目失败: %v", err))
	}

	fmt.Printf("[Deploy] 已生成引导条目: %s\n", confPath)
	fmt.Println("[Deploy] ─────────────────────────────────────────────")
	fmt.Println("[Deploy] ✓ 部署完成")
	fmt.Println()
	fmt.Println("[Deploy] 重启后在 systemd-boot 菜单中选择:")
	fmt.Printf("[Deploy]   %s - %s\n", entryTitle, snapName)
}

// listBootEntries 列出已部署的引导条目
func listBootEntries() {
	fmt.Println("[Deploy] 已部署的 StarSleep 引导条目:")
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
		fmt.Println("  (无)")
	}
}

// removeBootEntry 移除引导条目
func removeBootEntry(snapName string) {
	conf := filepath.Join(entryDir, fmt.Sprintf("starsleep-%s.conf", snapName))
	bootSnap := filepath.Join(bootDir, snapName)

	if _, err := os.Stat(conf); err == nil {
		os.Remove(conf)
		fmt.Printf("[Deploy] 已移除引导条目: %s\n", conf)
	} else {
		fmt.Printf("[Deploy] 引导条目不存在: %s\n", conf)
	}

	if _, err := os.Stat(bootSnap); err == nil {
		os.RemoveAll(bootSnap)
		fmt.Printf("[Deploy] 已移除启动文件: %s\n", bootSnap)
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
		fatal(fmt.Sprintf("读取 %s 失败: %v", src, err))
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		fatal(fmt.Sprintf("写入 %s 失败: %v", dst, err))
	}
}
