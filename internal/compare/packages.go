// packages.go — 包列表对比逻辑
//
// 从声明式配置中汇总所有期望的软件包列表，查询当前系统实际安装情况，
// 并输出差异报告。智能处理包组，根据 verbose 模式控制输出详细程度。
//
// 对比结果分为四类:
//   - 缺失包: 配置中有但系统完全未安装的包
//   - 依赖安装包: 配置中定义但系统以依赖方式安装（非主动安装）
//   - 多余包: 系统主动安装了但配置中未定义的包
//   - 已安装的包组: 配置中定义的包组，其成员已安装
package compare

import (
	"fmt"
	"sort"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// Packages 执行包列表对比模式
//
// 从配置中汇总所有期望的软件包列表，然后查询当前系统的包列表进行对比。
// 智能处理包组：如果配置中指定了包组（如 gnome、fcitx5-im），
// 会检查组成员是否已安装，已安装的组不报为缺失。
//
// @param layers 所有层配置
// @param configDir 配置目录路径
// @param verbose 是否启用详细模式
func Packages(layers []*config.LayerConfig, configDir, dbPath string, verbose bool) {
	fmt.Println(i18n.T("compare.separator"))
	fmt.Println(i18n.T("compare.pkg.title"))
	fmt.Println(i18n.T("compare.config.dir", configDir))
	fmt.Println(i18n.T("compare.separator"))

	// ── 汇总配置中定义的原始包名（含包组名） ──
	var allConfigPkgs []string
	for _, cfg := range layers {
		switch cfg.Helper {
		case "pacstrap", "pacman", "paru", "chroot-pacman", "chroot-paru":
			allConfigPkgs = append(allConfigPkgs, cfg.Packages...)
		}
	}

	diff, err := ComputePkgDiff(allConfigPkgs, "/", dbPath)
	if err != nil {
		util.Fatal(i18n.T("compare.query.failed", err))
	}

	fmt.Println(i18n.T("compare.pkg.expected", len(diff.ExpectedSet)))
	fmt.Println(i18n.T("compare.pkg.installed", len(diff.ExplicitSet)))
	if verbose {
		fmt.Println(i18n.T("compare.pkg.all.installed", len(diff.InstalledSet)))
	}
	fmt.Println(i18n.T("compare.separator"))

	// 检查包组：标记组成员是否完整安装
	var groupStatus []string
	for groupName, members := range diff.GroupMembers {
		installed := 0
		for _, m := range members {
			if diff.InstalledSet[m] {
				installed++
			}
		}
		status := fmt.Sprintf("%s (%d/%d)", groupName, installed, len(members))
		groupStatus = append(groupStatus, status)
	}
	sort.Strings(groupStatus)

	// ── 输出结果 ──

	// 包组状态（仅详细模式）
	if verbose && len(groupStatus) > 0 {
		fmt.Println(i18n.T("compare.pkg.groups.header", len(groupStatus)))
		for _, gs := range groupStatus {
			fmt.Println(i18n.T("compare.pkg.groups.item", gs))
		}
		fmt.Println()
	}

	// 缺失包（完全未安装的，始终显示）
	if len(diff.Missing) > 0 {
		fmt.Println(i18n.T("compare.pkg.missing.header", len(diff.Missing)))
		for _, pkg := range diff.Missing {
			fmt.Println(i18n.T("compare.pkg.missing.item", pkg))
		}
	} else if verbose {
		fmt.Println(i18n.T("compare.pkg.no.missing"))
	}
	if len(diff.Missing) > 0 || verbose {
		fmt.Println()
	}

	// 依赖安装包（仅详细模式显示，平时视为已安装无差异）
	if verbose {
		if len(diff.AsDeps) > 0 {
			fmt.Println(i18n.T("compare.pkg.asdeps.header", len(diff.AsDeps)))
			for _, pkg := range diff.AsDeps {
				fmt.Println(i18n.T("compare.pkg.asdeps.item", pkg))
			}
		} else {
			fmt.Println(i18n.T("compare.pkg.no.asdeps"))
		}
		fmt.Println()
	}

	// 多余包（始终显示）
	if len(diff.Extra) > 0 {
		fmt.Println(i18n.T("compare.pkg.extra.header", len(diff.Extra)))
		for _, pkg := range diff.Extra {
			fmt.Println(i18n.T("compare.pkg.extra.item", pkg))
		}
	} else if verbose {
		fmt.Println(i18n.T("compare.pkg.no.extra"))
	}

	// ── 汇总 ──
	fmt.Println(i18n.T("compare.separator"))
	totalDiff := len(diff.Missing) + len(diff.Extra)
	if totalDiff == 0 {
		fmt.Println(i18n.T("compare.pkg.match"))
	} else if verbose {
		fmt.Println(i18n.T("compare.pkg.diff.verbose", totalDiff, len(diff.Missing), len(diff.AsDeps), len(diff.Extra)))
	} else {
		fmt.Println(i18n.T("compare.pkg.diff", totalDiff, len(diff.Missing), len(diff.Extra)))
	}
}

