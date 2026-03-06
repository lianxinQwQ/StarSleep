package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// syncEnableService 使用 systemctl --root 启用 systemd 服务，并禁用不在累积列表中的服务
func syncEnableService(root string, services, expectedSvcs []string) {
	// 构建期望集合
	expectedSet := make(map[string]bool, len(expectedSvcs))
	for _, svc := range expectedSvcs {
		expectedSet[svc] = true
	}

	// 清理：禁用不在期望列表中的已启用服务
	enabled := listEnabledServices("--root", root)
	for _, svc := range enabled {
		if !expectedSet[svc] {
			fmt.Println(T("sync.disable.extra", svc))
			runSilent("systemctl", "--root", root, "disable", svc)
		}
	}

	// 启用当前层的服务
	if len(services) == 0 {
		fmt.Println(T("sync.no.services"))
		return
	}

	fmt.Println(T("sync.enable.start"))
	for _, svc := range services {
		fmt.Println(T("sync.enable.service", svc))
		if err := run("systemctl", "--root", root, "enable", svc); err != nil {
			fatal(T("sync.enable.failed", svc, err))
		}
	}
	fmt.Println(T("sync.enabled.count", len(services)))
}

// enableServiceLive 在当前运行系统上同步服务（动态维护模式）
// 启用期望服务，禁用不在期望列表中的已启用服务
func enableServiceLive(services []string) {
	expectedSet := make(map[string]bool, len(services))
	for _, svc := range services {
		expectedSet[svc] = true
	}

	// 禁用不在期望列表中的已启用服务
	enabled := listEnabledServices()
	for _, svc := range enabled {
		if !expectedSet[svc] {
			fmt.Println(T("maintain.disable.extra", svc))
			runSilent("systemctl", "disable", "--now", svc)
		}
	}

	// 启用期望服务
	for _, svc := range services {
		fmt.Println(T("maintain.enable.service", svc))
		if err := run("systemctl", "enable", "--now", svc); err != nil {
			fmt.Fprintln(os.Stderr, T("maintain.enable.failed", svc, err))
		}
	}
}

// listEnabledServices 列出当前已启用的 systemd 服务单元名称
// args 可选: "--root", "/path" 用于 chroot 环境
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
		// 格式: "unit.service enabled ..."
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			name := fields[0]
			// 去除 .service 后缀以匹配配置中的短名
			name = strings.TrimSuffix(name, ".service")
			svcs = append(svcs, name)
		}
	}
	return svcs
}
