// init_cmd.go — starsleep init 命令
//
// 创建工作目录结构、复制配置模板
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func cmdInit(args []string) {
	checkRoot()

	configDir, remaining := parseConfigFlags(args)
	if len(remaining) > 0 {
		fatal("init: 未知参数: " + remaining[0])
	}

	workDir := defaultWorkDir

	// 依赖检查
	requiredCmds := []string{
		"pacstrap", "pacman", "arch-chroot",
		"mount", "umount", "btrfs", "getfattr", "rsync", "setfattr",
	}
	var missing []string
	for _, cmd := range requiredCmds {
		if _, err := exec.LookPath(cmd); err != nil {
			missing = append(missing, cmd)
		}
	}
	// 检查 overlay 内核模块
	if err := run("modprobe", "overlay"); err != nil {
		missing = append(missing, "overlay (kernel module)")
	}
	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, "错误: 缺少以下依赖，请先安装:")
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "  - %s\n", m)
		}
		os.Exit(1)
	}

	fmt.Println("=== StarSleep 环境初始化 ===")
	fmt.Printf("工作目录: %s\n", workDir)

	// 创建目录结构
	dirs := []string{
		filepath.Join(workDir, "layers"),
		filepath.Join(workDir, "snapshots"),
		filepath.Join(workDir, "shared/home"),
		filepath.Join(workDir, "shared/pacman-cache"),
		filepath.Join(workDir, "config/layers"),
		filepath.Join(workDir, "work/merged"),
		filepath.Join(workDir, "work/ovl_work"),
		filepath.Join(workDir, "logs"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o755)
	}

	// 复制配置
	if configDir != defaultConfigDir {
		fmt.Printf("复制配置: %s → %s\n", configDir, defaultConfigDir)
		if err := copyConfig(configDir, filepath.Join(workDir, "config")); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 复制配置失败: %v\n", err)
		}
	}

	// 部署 starsleep-verify（仍为 bash 脚本）
	selfDir, _ := os.Executable()
	if selfDir != "" {
		selfDir = filepath.Dir(selfDir)
	}
	verifyScript := filepath.Join(selfDir, "starsleep-verify")
	if _, err := os.Stat(verifyScript); err == nil {
		data, err := os.ReadFile(verifyScript)
		if err == nil {
			dst := filepath.Join(workDir, "starsleep-verify")
			os.WriteFile(dst, data, 0o755)
			fmt.Printf("已部署: %s\n", dst)
		}
	}

	fmt.Println()
	fmt.Println("=== 环境初始化完成 ===")
	fmt.Println("目录结构:")
	fmt.Printf("  %s/\n", workDir)
	fmt.Println("  ├── layers/          # 各层 diff 数据")
	fmt.Println("  ├── snapshots/       # 生产快照")
	fmt.Println("  ├── shared/          # 持久化共享数据")
	fmt.Println("  │   ├── home/")
	fmt.Println("  │   └── pacman-cache/")
	fmt.Println("  ├── config/          # 配置文件")
	fmt.Println("  │   ├── layers/      # 层定义 (YAML)")
	fmt.Println("  │   └── inherit.list # 继承列表")
	fmt.Println("  ├── work/            # 构建工作区")
	fmt.Println("  │   ├── flat/        # 展平子卷 (Btrfs)")
	fmt.Println("  │   ├── merged/      # OverlayFS 合并挂载点")
	fmt.Println("  │   └── ovl_work/    # OverlayFS 工作目录")
	fmt.Println("  └── logs/            # 构建日志")
	fmt.Println()
	fmt.Println("下一步: sudo starsleep build")
}
