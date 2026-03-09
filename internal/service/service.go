// service 包提供 systemd 服务的声明式管理功能。
//
// 支持两种模式:
//   - 构建模式 (SyncEnableService): 通过 systemctl --root 操作离线根目录
//   - 维护模式 (EnableServiceLive): 通过 systemctl enable/disable --now 操作当前系统
package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncEnableService 使用 systemctl --root 声明式地同步服务状态
//
// 先禁用不在累积列表中的已启用服务，再启用配置中指定的服务。
// 通过解析服务单元文件的 Also=/Alias= 指令来避免误禁用隐式依赖。
//
// @param root 目标根目录路径
// @param services 当前层要启用的服务列表
// @param expectedSvcs 到当前层为止的累积服务列表
// @throws 服务启用失败时调用 Fatal 退出
func SyncEnableService(root string, services, expectedSvcs []string) {
	// 合并累积服务和当前层服务，解析 Also=/Alias= 依赖
	allConfigured := append(expectedSvcs, services...)
	expectedSet := resolveServiceWithDeps(root, allConfigured)

	// 禁用不在期望集合中的已启用服务
	enabled := listEnabledServices("--root", root)
	for _, svc := range enabled {
		if !expectedSet[svc] {
			fmt.Println(i18n.T("sync.disable.extra", svc))
			util.RunSilent("systemctl", "--root", root, "disable", svc)
		}
	}

	if len(services) == 0 {
		fmt.Println(i18n.T("sync.no.services"))
		return
	}

	fmt.Println(i18n.T("sync.enable.start"))
	for _, svc := range services {
		fmt.Println(i18n.T("sync.enable.service", svc))
		if err := util.Run("systemctl", "--root", root, "enable", svc); err != nil {
			util.Fatal(i18n.T("sync.enable.failed", svc, err))
		}
	}
	fmt.Println(i18n.T("sync.enabled.count", len(services)))
}

// EnableServiceLive 在当前运行系统上声明式地同步服务状态
//
// 与 SyncEnableService 类似，但使用 --now 标志立即启动/停止服务。
//
// @param services 要启用的服务列表
func EnableServiceLive(services []string) {
	expectedSet := resolveServiceWithDeps("/", services)

	enabled := listEnabledServices()
	for _, svc := range enabled {
		if !expectedSet[svc] {
			fmt.Println(i18n.T("maintain.disable.extra", svc))
			util.RunSilent("systemctl", "disable", "--now", svc)
		}
	}

	for _, svc := range services {
		fmt.Println(i18n.T("maintain.enable.service", svc))
		if err := util.Run("systemctl", "enable", "--now", svc); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T("maintain.enable.failed", svc, err))
		}
	}
}

// resolveServiceWithDeps 解析配置的服务及其 Also=/Alias= 隐式依赖
//
// 递归解析每个服务单元文件的 [Install] 段，
// 提取 Also= 和 Alias= 指令中引用的服务名，
// 返回完整的服务名集合（不包含 .service 后缀）。
//
// @param root 目标根目录路径
// @param services 服务名切片
// @return 完整的服务名集合
func resolveServiceWithDeps(root string, services []string) map[string]bool {
	result := make(map[string]bool)
	var resolve func(string)
	resolve = func(svc string) {
		name := strings.TrimSuffix(svc, ".service")
		if result[name] {
			return
		}
		result[name] = true

		// 在 /etc/systemd/system 和 /usr/lib/systemd/system 中查找单元文件
		unitFile := name + ".service"
		paths := []string{
			filepath.Join(root, "etc/systemd/system", unitFile),
			filepath.Join(root, "usr/lib/systemd/system", unitFile),
		}
		for _, unitPath := range paths {
			data, err := os.ReadFile(unitPath)
			if err != nil {
				continue
			}
			inInstall := false
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "[Install]" {
					inInstall = true
					continue
				}
				if strings.HasPrefix(line, "[") {
					inInstall = false
					continue
				}
				if !inInstall {
					continue
				}
				// 解析 [Install] 段中的 Also= 和 Alias= 指令
				if strings.HasPrefix(line, "Also=") {
					for _, dep := range strings.Fields(strings.TrimPrefix(line, "Also=")) {
						resolve(dep)
					}
				}
				if strings.HasPrefix(line, "Alias=") {
					for _, alias := range strings.Fields(strings.TrimPrefix(line, "Alias=")) {
						result[strings.TrimSuffix(alias, ".service")] = true
					}
				}
			}
			break
		}
	}
	for _, svc := range services {
		resolve(svc)
	}
	return result
}

// listEnabledServices 列出当前已启用的 systemd 服务单元名称
//
// 调用 systemctl list-unit-files --state=enabled 获取已启用服务列表。
// 返回的服务名不包含 .service 后缀。
//
// @param args 额外的 systemctl 参数（如 "--root", "/path"）
// @return 已启用的服务名切片
func listEnabledServices(args ...string) []string {
	cmdArgs := append(args, "list-unit-files", "--state=enabled", "--type=service", "--no-legend", "--no-pager")
	cmd := exec.Command("systemctl", cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil
	}

	var svcs []string
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			name := fields[0]
			name = strings.TrimSuffix(name, ".service")
			svcs = append(svcs, name)
		}
	}
	return svcs
}
