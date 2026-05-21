// chroot_paru — 通过 arch-chroot 执行 paru（AUR 包安装）
package helper

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// ensureBuilderChroot 确保 chroot 内存在 builder 用户并属于 wheel 组，且配置免密 sudo
func ensureBuilderChroot(env []string, root, dbPath string) {
	// 先确保 builder 主组存在（防止 chown builder:builder 失败）
	util.RunWithEnv(env, "arch-chroot", root, "groupadd", "-f", "builder")
	// 检查用户是否存在
	if _, err := util.RunSilent("arch-chroot", root, "id", "builder"); err != nil {
		fmt.Println(i18n.T("chroot.paru.create.user"))
		if err := util.RunWithEnv(env, "arch-chroot", root,
			"useradd", "-m", "-g", "builder", "-G", "wheel", "-s", "/bin/bash", "builder"); err != nil {
			util.Fatal(i18n.T("chroot.paru.create.user.failed", err))
		}
	} else {
		// 用户已存在，确保主组和 wheel 组正确
		util.RunWithEnv(env, "arch-chroot", root, "usermod", "-g", "builder", "-aG", "wheel", "builder")
	}
	// 配置 sudoers 免密码
	sudoersFile := root + "/etc/sudoers.d/builder"
	os.MkdirAll(root+"/etc/sudoers.d", 0o755)
	os.WriteFile(sudoersFile, []byte("builder ALL=(ALL) NOPASSWD: ALL\n"), 0o440)
	// 确保 builder 主目录权限正确（home 目录可能已存在且归属旧 UID）
	util.RunWithEnv(env, "arch-chroot", root,
		"chown", "-R", "builder:builder", "/home/builder")
	// 清除 chroot 内残留 pacman 锁文件，防止 paru 初始化 libalpm 失败
	os.Remove(filepath.Join(root, dbPath, "db.lck"))
}
func ensureBuilderLive(paruCacheDir string) {
	if _, err := util.RunSilent("id", "builder"); err != nil {
		fmt.Println(i18n.T("chroot.paru.create.user"))
		// 先确保同名主组存在，再创建用户（防止 chown builder:builder 失败）
		util.RunSilent("groupadd", "-f", "builder")
		if err := util.Run("useradd", "-m", "-g", "builder", "-G", "wheel", "-s", "/bin/bash", "builder"); err != nil {
			util.Fatal(i18n.T("chroot.paru.create.user.failed", err))
		}
	} else {
		// 补建主组（处理旧版本没有创建组的情况）
		util.RunSilent("groupadd", "-f", "builder")
		util.RunSilent("usermod", "-g", "builder", "-aG", "wheel", "builder")
	}
	// 无论是新建还是已存在，均确保家目录存在且归属正确
	if err := util.Run("install", "-d", "-m", "0755", "-o", "builder", "-g", "builder", "/home/builder"); err != nil {
		util.Run("chown", "-R", "builder:builder", "/home/builder")
	}
	// 配置 sudoers 免密码
	os.MkdirAll("/etc/sudoers.d", 0o755)
	os.WriteFile("/etc/sudoers.d/builder", []byte("builder ALL=(ALL) NOPASSWD: ALL\n"), 0o440)
	// 确保 paru 缓存目录属于 builder
	util.Run("chown", "builder:builder", paruCacheDir)
	util.Run("install", "-d", "-o", "builder", "-g", "builder", paruCacheDir)
	// 配置 builder 的 git 全局信任，防止 cloneDir 下仓库触发 safe.directory 检查
	util.RunSilent("runuser", "-u", "builder", "--",
		"git", "config", "--global", "--replace-all", "safe.directory", "*")
}

// EnsureBuilderUser 导出的 ensureBuilderLive，供 maintain 等外部包使用
func EnsureBuilderUser() {
	ensureBuilderLive(filepath.Join(config.DefaultWorkDir, "shared/paru-cache"))
}

// SyncChrootParu 通过 arch-chroot 在目标根中使用 paru 安装 AUR 软件包
func SyncChrootParu(root, dbPath string, envVars []config.EnvVar, pkgs []string) {
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

	ensureBuilderChroot(resolvedEnv, root, dbPath)

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

	ensureBuilderLive(filepath.Join(config.DefaultWorkDir, "shared/paru-cache"))

	args := []string{"-u", "builder", "--",
		"paru", "-S", "--needed", "--noconfirm"}
	args = append(args, pkgs...)

	if err := util.RunWithEnv(resolvedEnv, "runuser", args...); err != nil {
		util.Fatal(i18n.T("chroot.paru.failed", err))
	}

	fmt.Println(i18n.T("chroot.paru.done", len(pkgs)))
}
