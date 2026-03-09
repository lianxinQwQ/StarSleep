// pkgmgr 包提供 Arch Linux 包管理器的封装操作。
//
// alpm.go 通过 go-alpm 库直接访问 pacman 数据库，
// 提供显式包查询、孤立包查询和包组展开功能。
package pkgmgr

import (
	"fmt"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"

	alpm "github.com/Jguer/go-alpm/v2"
)

// openHandle 初始化 ALPM 句柄
//
// 将 root 路径和对应的 pacman 数据库路径传给 ALPM。
//
// @param root 目标根目录路径
// @return ALPM 句柄和可能的错误
func openHandle(root string) (*alpm.Handle, error) {
	dbPath := filepath.Join(root, "var/lib/pacman")
	h, err := alpm.Initialize(root, dbPath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("alpm.init"), err)
	}
	return h, nil
}

// ListExplicitPkgs 列出目标根文件系统中所有显式安装的包名
//
// 通过 ALPM 本地数据库遍历所有包，过滤出安装原因为 "explicit" 的包。
// 用于声明式清理时确定哪些包需要降级为依赖。
//
// @param root 目标根目录路径
// @return 显式安装的包名切片和可能的错误
func ListExplicitPkgs(root string) ([]string, error) {
	h, err := openHandle(root)
	if err != nil {
		return nil, err
	}
	defer h.Release()
	localDB, err := h.LocalDB()
	if err != nil {
		return nil, fmt.Errorf(i18n.T("alpm.localdb"), err)
	}
	var pkgs []string
	localDB.PkgCache().ForEach(func(pkg alpm.IPackage) error {
		if pkg.Reason() == alpm.PkgReasonExplicit {
			pkgs = append(pkgs, pkg.Name())
		}
		return nil
	})
	return pkgs, nil
}

// ListOrphans 列出目标根文件系统中的孤立依赖包
//
// 调用 pacman -Qtdq 查找不被任何其他包依赖且非显式安装的包。
//
// @param root 目标根目录路径
// @return 孤立包名切片，无孤立包时返回 nil
func ListOrphans(root string) ([]string, error) {
	dbPath := filepath.Join(root, "var/lib/pacman")
	output, err := util.RunSilent("pacman",
		"--root", root, "--dbpath", dbPath, "-Qtdq")
	if err != nil || strings.TrimSpace(output) == "" {
		return nil, nil
	}
	var orphans []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			orphans = append(orphans, line)
		}
	}
	return orphans, nil
}

// ExpandPkgGroups 将包名列表中的组名展开为实际包名，返回所有包名的集合
//
// 调用 pacman -Sg 解析包组，将组内的所有包名添加到结果集合中。
// 不属于任何组的包名将直接保留在结果中。
//
// @param pkgs 包含包名和/或组名的字符串切片
// @return 展开后的所有包名集合（map[string]bool）
func ExpandPkgGroups(pkgs []string) map[string]bool {
	result := make(map[string]bool, len(pkgs))
	for _, pkg := range pkgs {
		result[pkg] = true
	}
	if len(pkgs) == 0 {
		return result
	}
	args := append([]string{"-Sg", "--"}, pkgs...)
	output, _ := util.RunSilent("pacman", args...)
	if output == "" {
		return result
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			result[fields[1]] = true
		}
	}
	return result
}
