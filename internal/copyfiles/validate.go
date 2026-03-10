// copyfiles 包中的路径验证与安全拼接功能。
//
// 所有源路径都被强制限定在配置目录的 files/ 子目录中，
// 通过路径清理和前缀检查防止路径穿越攻击。
package copyfiles

import (
	"fmt"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
)

// SafeJoin 将不可信的相对路径安全地拼接到基础目录下
//
// 对输入路径执行以下安全处理:
//  1. 在路径前添加 "/" 再执行 Clean，消除所有 ".." 和多余分隔符
//  2. 拼接到 base 目录下
//  3. 验证结果路径确实位于 base 目录内
//
// 即使 rel 以 "/" 开头（如 "/etc/foo"）也会被当作相对路径处理，
// 确保结果始终在 base 目录内，防止路径穿越。
//
// 附注：本意是防止用户失误导致的路径错误，而不是针对恶意攻击者的复杂绕过手段
// 实际上用户仍然可以使用软链接等方案解决路径限制，但这至少能防止一些简单的配置错误。
// 我的AI理解成防入侵了（
//
// @param base 基础目录路径（必须是绝对路径）
// @param rel 不可信的相对路径（可能包含 / 前缀或 .. 等恶意构造）
// @return 安全拼接后的绝对路径
// @return error 结果路径逃逸出 base 目录时返回错误
func SafeJoin(base, rel string) (string, error) {
	// 先将 rel 当作绝对路径做 Clean，消除 ".." 和多余 "/"
	// 例如: "../../etc/passwd" → Clean("/../../etc/passwd") → "/etc/passwd"
	// 例如: "/etc/passwd" → Clean("//etc/passwd") → "/etc/passwd"
	cleaned := filepath.Clean("/" + rel)

	// 拼接到 base，Go 的 filepath.Join 会合并路径
	result := filepath.Join(base, cleaned)

	// 规范化 base 用于前缀检查
	cleanBase := filepath.Clean(base)

	// 确保结果路径以 base 目录为前缀（在 base 内部）
	if result != cleanBase && !strings.HasPrefix(result, cleanBase+string(filepath.Separator)) {
		return "", fmt.Errorf(i18n.T("copyfiles.path.escape"), rel)
	}

	return result, nil
}
