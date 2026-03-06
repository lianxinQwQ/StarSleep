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

// SyncEnableService 使用 systemctl --root 启用 systemd 服务，并禁用不在累积列表中的服务
func SyncEnableService(root string, services, expectedSvcs []string) {
	allConfigured := append(expectedSvcs, services...)
	expectedSet := resolveServiceWithDeps(root, allConfigured)

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

// EnableServiceLive 在当前运行系统上同步服务（动态维护模式）
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
func resolveServiceWithDeps(root string, services []string) map[string]bool {
	result := make(map[string]bool)
	var resolve func(string)
	resolve = func(svc string) {
		name := strings.TrimSuffix(svc, ".service")
		if result[name] {
			return
		}
		result[name] = true

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
