package main

import (
	"fmt"
	"path/filepath"
	"strings"

	alpm "github.com/Jguer/go-alpm/v2"
)

func openHandle(root string) (*alpm.Handle, error) {
	dbPath := filepath.Join(root, "var/lib/pacman")
	h, err := alpm.Initialize(root, dbPath)
	if err != nil {
		return nil, fmt.Errorf(T("alpm.init"), err)
	}
	return h, nil
}

// listExplicitPkgs 列出目标系统中所有显式安装的包名
func listExplicitPkgs(root string) ([]string, error) {
	h, err := openHandle(root)
	if err != nil {
		return nil, err
	}
	defer h.Release()

	localDB, err := h.LocalDB()
	if err != nil {
		return nil, fmt.Errorf(T("alpm.localdb"), err)
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

// listOrphans 列出目标系统中所有孤立依赖包
func listOrphans(root string) ([]string, error) {
	dbPath := filepath.Join(root, "var/lib/pacman")
	output, err := runSilent("pacman",
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
