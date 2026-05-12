// fstab.go — 根据目标分区 UUID 和子卷布局生成 fstab
package install

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// generateFstab 为目标系统生成 fstab 文件
//
// 根据根分区 UUID、EFI 分区路径生成包含 Btrfs 子卷布局的 fstab。
func generateFstab(rootUUID, bootPart string) {
	fmt.Println(i18n.T("install.gen.fstab"))

	// 获取 EFI 分区的 UUID
	bootUUID, err := util.RunSilent("blkid", "-s", "UUID", "-o", "value", bootPart)
	if err != nil {
		bootUUID = bootPart // 回退：使用设备路径
	}

	fstabContent := fmt.Sprintf(`# /etc/fstab — StarSleep 系统
# 由 starsleep install 自动生成

# ── EFI 系统分区 ──
UUID=%s  /boot  vfat  defaults,noatime,fmask=0133,dmask=0022  0  2

# ── Btrfs 根文件系统 ──
UUID=%s  /           btrfs  subvol=@,compress=zstd,noatime,ssd  0  0
UUID=%s  /home       btrfs  subvol=@home,compress=zstd,noatime  0  0
UUID=%s  /var        btrfs  subvol=@var,compress=zstd,noatime   0  0
UUID=%s  /starsleep  btrfs  subvol=@starsleep,compress=zstd     0  0

# ── 临时文件系统 ──
tmpfs  /tmp  tmpfs  defaults,noatime,mode=1777  0  0
`, bootUUID, rootUUID, rootUUID, rootUUID, rootUUID)

	fstabPath := filepath.Join(TargetMount, "@", "etc", "fstab")
	os.MkdirAll(filepath.Dir(fstabPath), 0o755)
	if err := os.WriteFile(fstabPath, []byte(fstabContent), 0o644); err != nil {
		util.Fatal(fmt.Sprintf("写入 fstab 失败: %v", err))
	}

	fmt.Println(i18n.T("install.fstab.done", fstabPath))
}
