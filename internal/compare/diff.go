// diff.go — 包对比核心逻辑，供 compare 和 maintain 共用
package compare

import (
	"sort"

	"starsleep/internal/pkgmgr"
)

// PkgDiff 保存配置期望包与系统实际安装包的对比结果
type PkgDiff struct {
	// ExpectedSet 展开包组后的完整期望包集合
	ExpectedSet map[string]bool
	// GroupMembers 包组名 → 成员包列表
	GroupMembers map[string][]string
	// Missing 配置期望但系统完全未安装的包
	Missing []string
	// AsDeps 配置期望但以依赖身份安装（非主动安装）的包
	AsDeps []string
	// Extra 系统主动安装但配置中未定义的包
	Extra []string
	// InstalledSet 全量已安装包集合（含依赖安装）
	InstalledSet map[string]bool
	// ExplicitSet 主动安装包集合
	ExplicitSet map[string]bool
}

// ComputePkgDiff 计算配置包列表与系统实际安装状态的差异。
// configPkgs 为原始配置包名列表（可含包组名）；root 和 dbPath 指定查询目标。
func ComputePkgDiff(configPkgs []string, root, dbPath string) (*PkgDiff, error) {
	// 解析包组
	groupMembers := pkgmgr.ResolveGroupMembers(configPkgs)

	// 构建期望集合（展开包组成员，排除组名本身）
	expectedSet := make(map[string]bool, len(configPkgs))
	for _, pkg := range configPkgs {
		if _, isGroup := groupMembers[pkg]; !isGroup {
			expectedSet[pkg] = true
		}
	}
	for _, members := range groupMembers {
		for _, m := range members {
			expectedSet[m] = true
		}
	}

	// 查询显式安装包
	explicitPkgs, err := pkgmgr.ListExplicitPkgs(root, dbPath)
	if err != nil {
		return nil, err
	}
	explicitSet := make(map[string]bool, len(explicitPkgs))
	for _, pkg := range explicitPkgs {
		explicitSet[pkg] = true
	}

	// 查询全量已安装包
	allInstalled, err := pkgmgr.ListInstalledPkgs(root, dbPath)
	if err != nil {
		return nil, err
	}
	installedSet := make(map[string]bool, len(allInstalled))
	for _, pkg := range allInstalled {
		installedSet[pkg] = true
	}

	// 计算 missing / asDeps
	var missing, asDeps []string
	for pkg := range expectedSet {
		if explicitSet[pkg] {
			continue
		}
		if installedSet[pkg] {
			asDeps = append(asDeps, pkg)
		} else {
			missing = append(missing, pkg)
		}
	}
	sort.Strings(missing)
	sort.Strings(asDeps)

	// 计算 extra
	var extra []string
	for pkg := range explicitSet {
		if !expectedSet[pkg] {
			extra = append(extra, pkg)
		}
	}
	sort.Strings(extra)

	return &PkgDiff{
		ExpectedSet:  expectedSet,
		GroupMembers: groupMembers,
		Missing:      missing,
		AsDeps:       asDeps,
		Extra:        extra,
		InstalledSet: installedSet,
		ExplicitSet:  explicitSet,
	}, nil
}
