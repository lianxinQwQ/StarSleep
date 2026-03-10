// chroot 包中的 pacman 相关功能。
//
// 通过 arch-chroot 在目标根中运行 pacman 安装软件包，
// 适用于需要完整 chroot 环境的包安装场景（如安装内核）。
package chroot

import (
	"fmt"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncChrootPacman 通过 arch-chroot 在目标根中使用 pacman 安装软件包
//
// 使用 "arch-chroot root pacman -S --needed --noconfirm ..." 的方式安装，
// 适用于需要完整系统环境的场景（如 mkinitcpio 需要在 chroot 内执行）。
//
// @param root 目标根目录路径
// @param envVars 环境变量配置列表
// @param pkgs 要安装的包名列表
// @throws pacman 安装失败时调用 Fatal 退出
func SyncChrootPacman(root string, envVars []config.EnvVar, pkgs []string) {
	if len(pkgs) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.pacman.start"))

	// 打印环境变量设定
	for _, ev := range envVars {
		if ev.Value != "" {
			fmt.Println(i18n.T("chroot.env.set", ev.Key, ev.Value))
		} else if ev.HostKey != "" {
			fmt.Println(i18n.T("chroot.env.host", ev.Key, ev.HostKey))
		}
	}

	// 构建 arch-chroot pacman 命令参数
	args := []string{root, "pacman", "-S", "--needed", "--noconfirm"}
	args = append(args, pkgs...)

	if err := util.RunWithEnv(resolvedEnv, "arch-chroot", args...); err != nil {
		util.Fatal(i18n.T("chroot.pacman.failed", err))
	}

	fmt.Println(i18n.T("chroot.pacman.done", len(pkgs)))
}

// ChrootPacmanLive 在当前运行系统中使用 pacman 安装软件包
//
// 维护模式下直接执行 pacman（不需要 chroot），
// 但保持相同的环境变量设定。
//
// @param envVars 环境变量配置列表
// @param pkgs 要安装的包名列表
// @throws pacman 安装失败时调用 Fatal 退出
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
