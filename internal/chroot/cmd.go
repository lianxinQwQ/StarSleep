// chroot 包提供基于 arch-chroot 的命令执行功能。
//
// 支持在目标根目录中通过 arch-chroot 执行任意命令，
// 并可预设环境变量（固定值或继承主机环境变量）。
//
// 两种模式:
//   - SyncChrootCmd: 在 OverlayFS merged 挂载点中执行命令（构建模式）
//   - ChrootCmdLive: 直接在当前运行系统中执行命令（维护模式）
package chroot

import (
	"fmt"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncChrootCmd 通过 arch-chroot 在目标根中逐条执行配置中的命令
//
// 每条命令通过 "arch-chroot root /bin/bash -c <cmd>" 的方式执行，
// 环境变量在 arch-chroot 进程级别设置。
//
// @param root 目标根目录路径（构建时为 OverlayFS merged 挂载点）
// @param envVars 环境变量配置列表
// @param commands 要执行的命令字符串列表
// @throws 命令执行失败时调用 Fatal 退出
func SyncChrootCmd(root string, envVars []config.EnvVar, commands []string) {
	if len(commands) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.cmd.start"))

	// 打印将使用的环境变量
	for _, ev := range envVars {
		if ev.Value != "" {
			fmt.Println(i18n.T("chroot.env.set", ev.Key, ev.Value))
		} else if ev.HostKey != "" {
			fmt.Println(i18n.T("chroot.env.host", ev.Key, ev.HostKey))
		}
	}

	for _, cmd := range commands {
		fmt.Println(i18n.T("chroot.cmd.exec", cmd))
		if err := util.RunWithEnv(resolvedEnv, "arch-chroot", root, "/bin/bash", "-c", cmd); err != nil {
			util.Fatal(i18n.T("chroot.cmd.failed", cmd, err))
		}
	}

	fmt.Println(i18n.T("chroot.cmd.done", len(commands)))
}

// ChrootCmdLive 在当前运行系统中直接执行命令
//
// 维护模式下 root 为 "/"，但仍通过 bash -c 执行以保持一致的语义。
// 不使用 arch-chroot（当前系统已经是目标）。
//
// @param envVars 环境变量配置列表
// @param commands 要执行的命令字符串列表
// @throws 命令执行失败时调用 Fatal 退出
func ChrootCmdLive(envVars []config.EnvVar, commands []string) {
	if len(commands) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.cmd.start"))

	for _, cmd := range commands {
		fmt.Println(i18n.T("chroot.cmd.exec", cmd))
		if err := util.RunWithEnv(resolvedEnv, "/bin/bash", "-c", cmd); err != nil {
			util.Fatal(i18n.T("chroot.cmd.failed", cmd, err))
		}
	}

	fmt.Println(i18n.T("chroot.cmd.done", len(commands)))
}
