// chroot_cmd — 基于 arch-chroot 的命令执行
package helper

import (
	"fmt"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncChrootCmd 通过 arch-chroot 在目标根中逐条执行配置中的命令
func SyncChrootCmd(root string, envVars []config.EnvVar, commands []string) {
	if len(commands) == 0 {
		return
	}

	resolvedEnv := config.ResolveEnv(envVars)
	fmt.Println(i18n.T("chroot.cmd.start"))

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

// ChrootCmdLive 在当前运行系统中直接执行命令（维护模式）
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
