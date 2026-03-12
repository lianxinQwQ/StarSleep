// chroot_pacman — 通过 arch-chroot 执行 pacman
package helper

import (
	"fmt"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncChrootPacman 通过 arch-chroot 在目标根中使用 pacman 安装软件包
func SyncChrootPacman(root string, envVars []config.EnvVar, pkgs []string) {
	if len(pkgs) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.pacman.start"))

	for _, ev := range envVars {
		if ev.Value != "" {
			fmt.Println(i18n.T("chroot.env.set", ev.Key, ev.Value))
		} else if ev.HostKey != "" {
			fmt.Println(i18n.T("chroot.env.host", ev.Key, ev.HostKey))
		}
	}

	args := []string{root, "pacman", "-S", "--needed", "--noconfirm"}
	args = append(args, pkgs...)

	if err := util.RunWithEnv(resolvedEnv, "arch-chroot", args...); err != nil {
		util.Fatal(i18n.T("chroot.pacman.failed", err))
	}

	fmt.Println(i18n.T("chroot.pacman.done", len(pkgs)))
}

// ChrootPacmanLive 在当前运行系统中使用 pacman 安装软件包（维护模式）
func ChrootPacmanLive(envVars []config.EnvVar, pkgs []string) {
	if len(pkgs) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.pacman.start"))

	args := []string{"-S", "--needed", "--noconfirm"}
	args = append(args, pkgs...)

	if err := util.RunWithEnv(resolvedEnv, "pacman", args...); err != nil {
		util.Fatal(i18n.T("chroot.pacman.failed", err))
	}

	fmt.Println(i18n.T("chroot.pacman.done", len(pkgs)))
}
