// partition.go — 分区检测、交互式分区方案和格式化
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// LsblkOutput lsblk -J 输出的 JSON 结构
type LsblkOutput struct {
	Blockdevices []BlockDevice `json:"blockdevices"`
}

// BlockDevice 表示一个块设备
type BlockDevice struct {
	Name       string        `json:"name"`
	Size       string        `json:"size"`
	Type       string        `json:"type"`
	Mountpoint *string       `json:"mountpoint"`
	Fstype     *string       `json:"fstype"`
	Children   []BlockDevice `json:"children,omitempty"`
}

// detectDisks 获取可用磁盘列表（过滤掉小于 8GB 和 loop 设备）
func detectDisks() ([]BlockDevice, error) {
	output, err := util.RunSilent("lsblk", "-J", "-o", "NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE")
	if err != nil {
		return nil, fmt.Errorf("lsblk 执行失败: %v", err)
	}

	var result LsblkOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("解析磁盘信息失败: %v", err)
	}

	var disks []BlockDevice
	for _, dev := range result.Blockdevices {
		if dev.Type == "disk" && !strings.HasPrefix(dev.Name, "loop") {
			disks = append(disks, dev)
		}
	}

	if len(disks) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("install.no.disk"))
	}

	return disks, nil
}

// interactivePartition 交互式选择磁盘和分区方案
//
// 列出可用磁盘，让用户选择，然后列出该磁盘的现有分区让用户选择
// boot 和 root 分区。
func interactivePartition(disk string) (bootPart, rootPart string) {
	// 列出当前磁盘的分区
	output, err := util.RunSilent("lsblk", "-J", "-o", "NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE", disk)
	if err != nil {
		util.Fatal(fmt.Sprintf("读取磁盘 %s 信息失败: %v", disk, err))
	}

	var result LsblkOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		util.Fatal(fmt.Sprintf("解析磁盘信息失败: %v", err))
	}

	if len(result.Blockdevices) == 0 {
		util.Fatal(fmt.Sprintf("未找到磁盘设备: %s", disk))
	}

	dev := result.Blockdevices[0]
	if len(dev.Children) == 0 {
		fmt.Printf("磁盘 %s (%s) 没有分区，请先手动分区。\n", disk, dev.Size)
		fmt.Println("推荐分区方案 (GPT):")
		fmt.Println("  - EFI 系统分区: 512MB - 1GB (类型 EF00)")
		fmt.Println("  - 根分区: 剩余空间 (类型 8304, Btrfs)")
		os.Exit(1)
	}

	fmt.Printf("\n磁盘: %s (%s)\n", disk, dev.Size)
	fmt.Println("现有分区:")
	for i, child := range dev.Children {
		fsInfo := ""
		if child.Fstype != nil && *child.Fstype != "" {
			fsInfo = fmt.Sprintf(" [%s]", *child.Fstype)
		}
		fmt.Printf("  %d) /dev/%s (%s)%s\n", i+1, child.Name, child.Size, fsInfo)
	}
	fmt.Println()

	// 交互选择 boot 分区
	fmt.Print("请选择 EFI 系统分区编号 (通常为 1): ")
	var bootIdx int
	fmt.Scanf("%d", &bootIdx)
	if bootIdx < 1 || bootIdx > len(dev.Children) {
		util.Fatal("无效的选择")
	}
	bootPart = "/dev/" + dev.Children[bootIdx-1].Name

	// 交互选择 root 分区
	fmt.Print("请选择根分区编号 (通常为 2): ")
	var rootIdx int
	fmt.Scanf("%d", &rootIdx)
	if rootIdx < 1 || rootIdx > len(dev.Children) {
		util.Fatal("无效的选择")
	}
	if rootIdx == bootIdx {
		util.Fatal("boot 分区和根分区不能相同")
	}
	rootPart = "/dev/" + dev.Children[rootIdx-1].Name

	fmt.Printf("\n已选择:\n")
	fmt.Printf("  EFI 系统分区: %s\n", bootPart)
	fmt.Printf("  根分区:       %s\n\n", rootPart)

	if !confirmPrompt("确认以上选择? (y/N): ") {
		fmt.Println("已取消")
		os.Exit(0)
	}

	return bootPart, rootPart
}

// formatPartitions 格式化 boot 和 root 分区
func formatPartitions(bootPart, rootPart string, force bool) {
	// 检测分区是否已挂载，已挂载则询问卸载
	checkAlreadyMounted(bootPart, force)
	checkAlreadyMounted(rootPart, force)

	// 检测并确认格式化
	checkForce := func(part string) {
		fstype, err := util.RunSilent("blkid", "-s", "TYPE", "-o", "value", part)
		if err == nil && strings.TrimSpace(fstype) != "" {
			if !force {
				if !confirmPrompt(i18n.T("install.confirm.format", part)) {
					fmt.Println("已取消格式化，退出安装")
					os.Exit(0)
				}
			}
		}
	}

	checkForce(bootPart)
	checkForce(rootPart)

	// 格式化 EFI 分区为 FAT32
	fmt.Println(i18n.T("install.format.boot", bootPart))
	if err := util.Run("mkfs.fat", "-F32", bootPart); err != nil {
		util.Fatal(fmt.Sprintf("格式化 EFI 分区失败: %v", err))
	}

	// 格式化根分区为 Btrfs
	fmt.Println(i18n.T("install.format.root", rootPart))
	if err := util.Run("mkfs.btrfs", "-f", rootPart); err != nil {
		util.Fatal(fmt.Sprintf("格式化根分区失败: %v", err))
	}

	fmt.Println(i18n.T("install.format.done"))
}

// checkAlreadyMounted 检测分区是否已挂载，若已挂载则询问用户是否卸载
func checkAlreadyMounted(part string, force bool) {
	mountpoint, err := util.RunSilent("findmnt", "-n", "-o", "TARGET", part)
	if err != nil || mountpoint == "" {
		return // 未挂载
	}
	mountpoint = strings.TrimSpace(mountpoint)
	fmt.Printf(i18n.T("install.mounted.detected"), part, mountpoint)
	if force {
		fmt.Println(i18n.T("install.mounted.force"))
		umountPartition(part, mountpoint)
		return
	}
	if confirmPrompt(i18n.T("install.mounted.umount.confirm", part)) {
		umountPartition(part, mountpoint)
	} else {
		util.Fatal(fmt.Sprintf("分区 %s 已挂载到 %s，无法继续", part, mountpoint))
	}
}

// umountPartition 卸载指定分区
func umountPartition(part, mountpoint string) {
	fmt.Printf(i18n.T("install.unmounting"), part)
	if err := util.Run("umount", part); err != nil {
		// 尝试 lazy unmount
		if err := util.Run("umount", "-l", part); err != nil {
			util.Fatal(fmt.Sprintf("卸载分区 %s (挂载点: %s) 失败: %v", part, mountpoint, err))
		}
	}
	fmt.Printf(i18n.T("install.unmounted"), part)
}

// createSubvolLayout 在目标分区上创建 Btrfs 子卷布局
//
// 返回根分区的 UUID。
func createSubvolLayout(rootPart string) string {
	// 获取 UUID
	rootUUID, err := util.RunSilent("blkid", "-s", "UUID", "-o", "value", rootPart)
	if err != nil {
		util.Fatal(fmt.Sprintf("获取根分区 UUID 失败: %v", err))
	}

	// Btrfs 子卷布局 (参考 openSUSE/Ubuntu 风格)
	subvols := []string{"@", "@home", "@var", "starsleep"}
	for _, sv := range subvols {
		subvolPath := filepath.Join(TargetMount, sv)
		fmt.Println(i18n.T("install.create.subvol", sv))
		if err := util.Run("btrfs", "subvolume", "create", subvolPath); err != nil {
			util.Fatal(fmt.Sprintf("创建子卷 %s 失败: %v", sv, err))
		}
	}

	fmt.Println(i18n.T("install.subvol.layout"))
	return rootUUID
}
