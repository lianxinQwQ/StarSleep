// boot.go — systemd-boot 初始化和引导条目生成
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// initBootloader 在目标 ESP 上初始化 systemd-boot 并创建首个引导条目
func initBootloader(espMount, rootUUID, snapshotName string) {
	fmt.Println(i18n.T("install.init.boot"))

	// 安装 systemd-boot 到 ESP
	if err := util.Run("bootctl", "install", "--esp-path="+espMount); err != nil {
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

	// 复制内核和 initramfs 到 ESP
	deploySnapshotToESP(espMount, snapshotName, rootUUID)

	fmt.Println(i18n.T("install.boot.done"))
}

// deploySnapshotToESP 从 build flatDir 复制内核和 initramfs 到目标 ESP
func deploySnapshotToESP(espMount, snapshotName, rootUUID string) {
	flatDir := filepath.Join("/starsleep", "work", "flat")
	bootSrc := filepath.Join(flatDir, "boot")

	// 查找内核和 initramfs
	vmlinuz := findBootFile(bootSrc, "vmlinuz-*")
	initramfs := findBootFile(bootSrc, "initramfs-*.img")

	if vmlinuz == "" || initramfs == "" {
		util.Fatal(fmt.Sprintf("未找到内核文件: vmlinuz=%s initramfs=%s", vmlinuz, initramfs))
	}

	// 复制到 ESP
	bootDest := filepath.Join(espMount, "starsleep", snapshotName)
	os.MkdirAll(bootDest, 0o755)

	copyFileData(filepath.Join(bootSrc, vmlinuz), filepath.Join(bootDest, "vmlinuz"))
	copyFileData(filepath.Join(bootSrc, initramfs), filepath.Join(bootDest, "initramfs-linux.img"))

	// 微码
	var initrdLines []string
	for _, ucode := range []string{"amd-ucode.img", "intel-ucode.img"} {
		ucodePath := filepath.Join(bootSrc, ucode)
		if _, err := os.Stat(ucodePath); err == nil {
			initrdLines = append(initrdLines, fmt.Sprintf("initrd /%s", ucode))
		}
	}
	initrdLines = append(initrdLines,
		fmt.Sprintf("initrd /starsleep/%s/initramfs-linux.img", snapshotName))

	// 生成引导条目
	confName := fmt.Sprintf("starsleep-%s.conf", snapshotName)
	confPath := filepath.Join(espMount, "loader", "entries", confName)
	os.MkdirAll(filepath.Dir(confPath), 0o755)

	entry := fmt.Sprintf(`title StarSleep - %s
linux /starsleep/%s/vmlinuz
%s
options root="UUID=%s" rootflags=subvol=@ rw quiet splash loglevel=3
`,
		snapshotName,
		snapshotName,
		strings.Join(initrdLines, "\n"),
		rootUUID)

	if err := os.WriteFile(confPath, []byte(entry), 0o644); err != nil {
		util.Fatal(fmt.Sprintf("写入引导条目失败: %v", err))
	}
}

func findBootFile(dir, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return filepath.Base(matches[0])
}

func copyFileData(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		util.Fatal(fmt.Sprintf("读取文件失败 %s: %v", src, err))
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		util.Fatal(fmt.Sprintf("写入文件失败 %s: %v", dst, err))
	}
}
