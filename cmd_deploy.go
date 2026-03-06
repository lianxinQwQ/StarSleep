// cmd_deploy.go — starsleep flatten 命令
//
// 将 StarSleep 快照的内核和 initramfs 复制到 ESP 分区，
// 并生成 systemd-boot 引导条目。
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
	bootDir      = "/boot/starsleep"
	entryDir     = "/boot/loader/entries"
	subvolPrefix = "starsleep/snapshots"
	entryTitle   = "StarSleep"
	rootLabel    = "arkane_root"
	kernelOpts   = "lsm=landlock,lockdown,yama,integrity,apparmor,bpf quiet splash loglevel=3 systemd.show_status=auto rd.udev.log_level=3 rw"
)

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
		target = filepath.Join(workDir, "snapshots/latest")
	} else {
		target = args[0]
	}

	if fi, err := os.Lstat(target); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, err := os.Readlink(target)
		if err == nil {
			target = resolved
		}
	}

	if _, err := os.Stat(target); err != nil {
		full := filepath.Join(workDir, "snapshots", target)
		if _, err2 := os.Stat(full); err2 == nil {
			target = full
		} else {
			util.Fatal(i18n.T("snapshot.not.exist", target))
		}
	}

	snapName := filepath.Base(target)
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
	fmt.Println(i18n.T("deploy.separator"))
	fmt.Println(i18n.T("deploy.done"))
	fmt.Println()
	fmt.Println(i18n.T("deploy.reboot.hint"))
	fmt.Println(i18n.T("deploy.reboot.entry", entryTitle, snapName))
}

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
