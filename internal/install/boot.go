// boot.go — systemd-boot 初始化和引导条目生成
package install

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// initBootloader 在目标 ESP 上初始化 systemd-boot 并创建首个引导条目
func initBootloader(espMount, rootUUID, snapshotName, entryName string) {
	fmt.Println(i18n.T("install.init.boot"))

	// 安装 systemd-boot 到 ESP，设置 UEFI 固件中的显示名称
	if err := util.Run("bootctl", "install", "--esp-path="+espMount, "--entry-token="+entryName); err != nil {
		util.Fatal(i18n.T("install.bootctl.failed", err))
	}

	// 创建 loader.conf
	loaderConf := fmt.Sprintf(`default starsleep-%s.conf
timeout 3
console-mode max
editor no
`, snapshotName)

	loaderPath := filepath.Join(espMount, "loader", "loader.conf")
	os.MkdirAll(filepath.Dir(loaderPath), 0o755)
	if err := os.WriteFile(loaderPath, []byte(loaderConf), 0o644); err != nil {
		util.Fatal(fmt.Sprintf("写入 loader.conf 失败: %v", err))
	}

	fmt.Println(i18n.T("install.boot.done"))
}

// deploySnapshotToESP 从 build flatDir 复制内核和 initramfs 到目标 ESP
