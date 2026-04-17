// chroot_paru — 通过 arch-chroot 执行 paru（AUR 包安装）
package helper

import (
	"fmt"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncChrootParu 通过 arch-chroot 在目标根中使用 paru 安装 AUR 软件包
func SyncChrootParu(root string, envVars []config.EnvVar, pkgs []string) {
	if len(pkgs) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.paru.start"))

	for _, ev := range envVars {
		if ev.Value != "" {
			fmt.Println(i18n.T("chroot.env.set", ev.Key, ev.Value))
		} else if ev.HostKey != "" {
			fmt.Println(i18n.T("chroot.env.host", ev.Key, ev.HostKey))
		}
	}

	// 先以 root 同步 pacman 数据库
	if err := util.RunWithEnv(resolvedEnv, "arch-chroot", root, "pacman", "-Sy", "--noconfirm"); err != nil {
		util.Fatal(i18n.T("chroot.paru.failed", err))
	}

	args := []string{root, "runuser", "-u", "builder", "--",
		"paru", "-S", "--needed", "--noconfirm"}
	args = append(args, pkgs...)

	if err := util.RunWithEnv(resolvedEnv, "arch-chroot", args...); err != nil {
		util.Fatal(i18n.T("chroot.paru.failed", err))
	}

	fmt.Println(i18n.T("chroot.paru.done", len(pkgs)))
}

// ChrootParuLive 在当前运行系统中使用 paru 安装 AUR 软件包（维护模式）
func ChrootParuLive(envVars []config.EnvVar, pkgs []string) {
	if len(pkgs) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.paru.start"))

	args := []string{"-u", "builder", "--",
		"paru", "-S", "--needed", "--noconfirm"}
	args = append(args, pkgs...)

	if err := util.RunWithEnv(resolvedEnv, "runuser", args...); err != nil {
		util.Fatal(i18n.T("chroot.paru.failed", err))
	}

	fmt.Println(i18n.T("chroot.paru.done", len(pkgs)))
}
