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
	"starsleep/internal/pkgmgr"
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
func Packages(layers []*config.LayerConfig, configDir string, verbose bool) {
	fmt.Println(i18n.T("compare.separator"))
	fmt.Println(i18n.T("compare.pkg.title"))
	fmt.Println(i18n.T("compare.config.dir", configDir))
	fmt.Println(i18n.T("compare.separator"))

	// ── 汇总配置中定义的原始包名（含包组名） ──
	var allConfigPkgs []string
	for _, cfg := range layers {
		switch cfg.Helper {
		case "pacstrap", "pacman", "paru":
			allConfigPkgs = append(allConfigPkgs, cfg.Packages...)
		}
	}

	// 解析哪些配置项是包组，获取组→成员映射
	groupMembers := pkgmgr.ResolveGroupMembers(allConfigPkgs)

	// 构建配置中直接指定的非组包名集合（排除包组名本身）
	configPkgSet := make(map[string]bool, len(allConfigPkgs))
	for _, pkg := range allConfigPkgs {
		if _, isGroup := groupMembers[pkg]; !isGroup {
			configPkgSet[pkg] = true
		}
	}
	// 将包组成员也加入期望集合
	for _, members := range groupMembers {
		for _, m := range members {
			configPkgSet[m] = true
		}
	}

	fmt.Println(i18n.T("compare.pkg.expected", len(configPkgSet)))

	// ── 查询当前系统的包列表 ──

	// 显式安装包（主动安装）
	explicitPkgs, err := pkgmgr.ListExplicitPkgs("/")
	if err != nil {
		util.Fatal(i18n.T("compare.query.failed", err))
	}
	explicitSet := make(map[string]bool, len(explicitPkgs))
	for _, pkg := range explicitPkgs {
		explicitSet[pkg] = true
	}

	// 全量已安装包（包含依赖安装的）
	allInstalled, err := pkgmgr.ListInstalledPkgs("/")
	if err != nil {
		util.Fatal(i18n.T("compare.query.failed", err))
	}
	installedSet := make(map[string]bool, len(allInstalled))
	for _, pkg := range allInstalled {
		installedSet[pkg] = true
	}

	fmt.Println(i18n.T("compare.pkg.installed", len(explicitSet)))
	if verbose {
		fmt.Println(i18n.T("compare.pkg.all.installed", len(installedSet)))
	}
	fmt.Println(i18n.T("compare.separator"))

	// ── 计算差异 ──

	// 真正缺失的包: 配置期望但系统完全未安装
	var missing []string
	// 作为依赖安装的包: 存在但非主动安装
	var asDeps []string

	for pkg := range configPkgSet {
		if explicitSet[pkg] {
			// 主动安装，无差异
			continue
		}
		if installedSet[pkg] {
			// 存在但以依赖身份安装
			asDeps = append(asDeps, pkg)
		} else {
			// 完全未安装
			missing = append(missing, pkg)
		}
	}
	sort.Strings(missing)
	sort.Strings(asDeps)

	// 检查包组：标记组成员是否完整安装
	var groupStatus []string
	for groupName, members := range groupMembers {
		installed := 0
		for _, m := range members {
			if installedSet[m] {
				installed++
			}
		}
		status := fmt.Sprintf("%s (%d/%d)", groupName, installed, len(members))
		groupStatus = append(groupStatus, status)
	}
	sort.Strings(groupStatus)

	// 多余包: 系统主动安装但配置中未定义（展开后也不匹配）
	var extra []string
	for pkg := range explicitSet {
		if !configPkgSet[pkg] {
			extra = append(extra, pkg)
		}
	}
	sort.Strings(extra)

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
	if len(missing) > 0 {
		fmt.Println(i18n.T("compare.pkg.missing.header", len(missing)))
		for _, pkg := range missing {
			fmt.Println(i18n.T("compare.pkg.missing.item", pkg))
		}
	} else if verbose {
		fmt.Println(i18n.T("compare.pkg.no.missing"))
	}
	if len(missing) > 0 || verbose {
		fmt.Println()
	}

	// 依赖安装包（仅详细模式显示，平时视为已安装无差异）
	if verbose {
		if len(asDeps) > 0 {
			fmt.Println(i18n.T("compare.pkg.asdeps.header", len(asDeps)))
			for _, pkg := range asDeps {
				fmt.Println(i18n.T("compare.pkg.asdeps.item", pkg))
			}
		} else {
			fmt.Println(i18n.T("compare.pkg.no.asdeps"))
		}
		fmt.Println()
	}

	// 多余包（始终显示）
	if len(extra) > 0 {
		fmt.Println(i18n.T("compare.pkg.extra.header", len(extra)))
		for _, pkg := range extra {
			fmt.Println(i18n.T("compare.pkg.extra.item", pkg))
		}
	} else if verbose {
		fmt.Println(i18n.T("compare.pkg.no.extra"))
	}

	// ── 汇总 ──
	fmt.Println(i18n.T("compare.separator"))
	// 普通模式: 只计缺失和多余为差异；详细模式: 额外显示依赖安装数
	totalDiff := len(missing) + len(extra)
	if totalDiff == 0 {
		fmt.Println(i18n.T("compare.pkg.match"))
	} else if verbose {
		fmt.Println(i18n.T("compare.pkg.diff.verbose", totalDiff, len(missing), len(asDeps), len(extra)))
	} else {
		fmt.Println(i18n.T("compare.pkg.diff", totalDiff, len(missing), len(extra)))
	}
}
