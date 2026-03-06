// maintain.go — starsleep maintain 命令
//
// 动态维护模式：汇总每个 helper 的内容，直接操作当前运行系统。
// 读取所有层配置，按 helper 类型汇总软件包和服务，
// 然后使用 pacman/paru/systemctl 直接在当前系统上执行同步。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdMaintain(args []string) {
	checkRoot()

	configDir, remaining := parseConfigFlags(args)
	if len(remaining) > 0 {
		fatal("maintain: 未知参数: " + remaining[0])
	}

	layers, _, err := loadAllLayers(configDir)
	if err != nil {
		fatal(fmt.Sprintf("加载配置失败: %v", err))
	}

	// 按 helper 类型汇总
	var officialPkgs []string // pacstrap + pacman 层的包
	var aurPkgs []string      // paru 层的包
	var services []string     // enable_service 层的服务

	for _, cfg := range layers {
		switch cfg.Helper {
		case "pacstrap", "pacman":
			officialPkgs = append(officialPkgs, cfg.Packages...)
		case "paru":
			aurPkgs = append(aurPkgs, cfg.Packages...)
		case "enable_service":
			services = append(services, cfg.Services...)
		}
	}

	// 所有包的合集（用于清理判断）
	allPkgs := make([]string, 0, len(officialPkgs)+len(aurPkgs))
	allPkgs = append(allPkgs, officialPkgs...)
	allPkgs = append(allPkgs, aurPkgs...)

	fmt.Println("[Maintain] ═══════════════════════════════════════════════")
	fmt.Println("[Maintain] StarSleep 动态维护模式")
	fmt.Printf("[Maintain] 配置目录: %s\n", configDir)
	fmt.Printf("[Maintain] 层数: %d\n", len(layers))
	fmt.Printf("[Maintain] 官方仓库包: %d 个\n", len(officialPkgs))
	fmt.Printf("[Maintain] AUR 包: %d 个\n", len(aurPkgs))
	fmt.Printf("[Maintain] 服务: %d 个\n", len(services))
	fmt.Println("[Maintain] ═══════════════════════════════════════════════")

	root := "/"
	dbPath := filepath.Join(root, "var/lib/pacman")

	// 1. 清理：降级不在配置中的显式包，移除孤儿
	fmt.Println("[Maintain] 步骤 1/4: 清理多余软件包...")
	maintainCleanup(root, dbPath, allPkgs)

	// 2. 安装官方仓库包
	if len(officialPkgs) > 0 {
		fmt.Println("[Maintain] 步骤 2/4: 同步官方仓库软件包...")
		args := append([]string{
			"-S", "--needed", "--noconfirm",
		}, officialPkgs...)
		if err := run("pacman", args...); err != nil {
			fatal(fmt.Sprintf("pacman 安装失败: %v", err))
		}
	} else {
		fmt.Println("[Maintain] 步骤 2/4: 无官方仓库包需要安装")
	}

	// 3. 安装 AUR 包
	if len(aurPkgs) > 0 {
		fmt.Println("[Maintain] 步骤 3/4: 同步 AUR 软件包...")
		paruArgs := append([]string{
			"-S", "--needed", "--noconfirm",
		}, aurPkgs...)
		if err := run("paru", paruArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "[Maintain] 警告: paru 安装失败: %v\n", err)
		}
	} else {
		fmt.Println("[Maintain] 步骤 3/4: 无 AUR 包需要安装")
	}

	// 4. 启用服务
	if len(services) > 0 {
		fmt.Println("[Maintain] 步骤 4/4: 启用 systemd 服务...")
		enableServiceLive(services)
	} else {
		fmt.Println("[Maintain] 步骤 4/4: 无服务需要启用")
	}

	fmt.Println("[Maintain] ═══════════════════════════════════════════════")
	fmt.Println("[Maintain] ✓ 动态维护完成")
}

// maintainCleanup 在当前系统上执行声明式清理
func maintainCleanup(root, dbPath string, expectedPkgs []string) {
	expectedSet := make(map[string]bool, len(expectedPkgs))
	for _, pkg := range expectedPkgs {
		expectedSet[pkg] = true
	}

	// 降级多余的显式安装包为依赖
	explicitPkgs, err := listExplicitPkgs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Maintain] 警告: 查询显式包列表失败: %v\n", err)
	} else {
		var demote []string
		for _, pkg := range explicitPkgs {
			if !expectedSet[pkg] {
				demote = append(demote, pkg)
			}
		}
		if len(demote) > 0 {
			fmt.Printf("[Maintain] 降级 %d 个包为依赖: %s\n",
				len(demote), strings.Join(demote, " "))
			for _, pkg := range demote {
				runSilent("pacman", "--dbpath", dbPath, "-D", "--asdeps", pkg, "--noconfirm")
			}
		}
	}

	// 循环清理孤立依赖
	for {
		orphans, err := listOrphans(root)
		if err != nil || len(orphans) == 0 {
			break
		}
		fmt.Printf("[Maintain] 清理孤立依赖: %s\n", strings.Join(orphans, " "))
		args := append([]string{"-Rs", "--noconfirm"}, orphans...)
		if err := run("pacman", args...); err != nil {
			fmt.Fprintf(os.Stderr, "[Maintain] 警告: 清理孤立依赖失败: %v\n", err)
			break
		}
	}
}
