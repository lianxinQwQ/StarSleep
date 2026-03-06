package pkgmgr

import (
	"fmt"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"

	alpm "github.com/Jguer/go-alpm/v2"
)

func openHandle(root string) (*alpm.Handle, error) {
	dbPath := filepath.Join(root, "var/lib/pacman")
	h, err := alpm.Initialize(root, dbPath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("alpm.init"), err)
	}
	return h, nil
}

// ListExplicitPkgs 列出目标根文件系统中所有显式安装的包名
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

// ListOrphans 列出目标根文件系统中孤立依赖包
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
