// fstab.go — 根据目标分区 UUID 和子卷布局生成 fstab
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// generateFstab 为目标系统生成 fstab 文件
//
// 根据根分区 UUID、EFI 分区路径生成包含 Btrfs 子卷布局的 fstab。
func generateFstab(snapshotRoot, rootUUID, bootPart string) {
	fmt.Println(i18n.T("install.gen.fstab"))

	// 获取 EFI 分区的 UUID
	bootUUID, err := util.RunSilent("blkid", "-s", "UUID", "-o", "value", bootPart)
	if err != nil {
		bootUUID = bootPart // 回退：使用设备路径
	}

	for _, mountpoint := range sharedMountpoints() {
		if err := os.MkdirAll(filepath.Join(snapshotRoot, strings.TrimPrefix(mountpoint, "/")), 0o755); err != nil {
			util.Fatal(fmt.Sprintf("创建挂载点失败 %s: %v", mountpoint, err))
		}
	}

	fstabContent := snapshotFstabContent(rootUUID, bootUUID)
	fstabPath := filepath.Join(snapshotRoot, "etc", "fstab")
	os.MkdirAll(filepath.Dir(fstabPath), 0o755)
	if err := os.WriteFile(fstabPath, []byte(fstabContent), 0o644); err != nil {
		util.Fatal(fmt.Sprintf("写入 fstab 失败: %v", err))
	}

	fmt.Println(i18n.T("install.fstab.done", fstabPath))
}

func snapshotFstabContent(rootUUID, bootUUID string) string {
	return fmt.Sprintf(`# /etc/fstab — StarSleep 系统
# 由 starsleep install 自动生成

# ── EFI 系统分区 ──
UUID=%s  /boot  vfat  defaults,noatime,fmask=0133,dmask=0022  0  2

# ── StarSleep 工作区与共享子卷 ──
UUID=%s  /starsleep  btrfs  subvol=starsleep,compress=zstd,noatime  0  0
UUID=%s  /home  btrfs  subvol=starsleep/shared/home,compress=zstd,noatime  0  0
UUID=%s  /root  btrfs  subvol=starsleep/shared/root,compress=zstd,noatime  0  0
UUID=%s  /var/cache/pacman/pkg  btrfs  subvol=starsleep/shared/pacman-cache,compress=zstd,noatime  0  0
UUID=%s  /home/builder/.cache/paru/clone  btrfs  subvol=starsleep/shared/paru-cache,compress=zstd,noatime  0  0

# ── 临时文件系统 ──
tmpfs  /tmp  tmpfs  defaults,noatime,mode=1777  0  0
`, bootUUID, rootUUID, rootUUID, rootUUID, rootUUID, rootUUID)
}

func sharedMountpoints() []string {
	return []string{
		"/starsleep",
		"/home",
		"/root",
		"/var/cache/pacman/pkg",
		"/home/builder/.cache/paru/clone",
	}
}
