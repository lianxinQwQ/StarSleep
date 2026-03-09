// flatten.go — OverlayFS 层 reflink 展平引擎
//
// 将 OverlayFS upper 层变更通过 reflink 合并到展平目录。
// 单次遍历完成所有操作：
//   - whiteout: 遇到后立即删除展平目录中的对应文件
//   - 复制时跳过 overlay 扩展属性（trusted.overlay.*）
//   - 不透明目录: 复制前先清空展平目录中的旧内容
package overlay

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"starsleep/internal/i18n"
)

// FlattenStats 记录展平过程中各类操作的统计计数
type FlattenStats struct {
	Whiteouts int // 删除的 whiteout 文件数
	Opaques   int // 处理的不透明目录数
	Files     int // 复制的普通文件数
	Symlinks  int // 复制的符号链接数
	Dirs      int // 处理的目录数
	Hardlinks int // 通过硬链接去重的文件数
}

// FlattenOverlay 将 upper 层变更通过 reflink 合并到 flat 目录
//
// 遍历 upper 层目录树，对每个条目按类型处理:
// whiteout → 删除、目录 → 创建/合并、文件 → reflink 复制。
//
// @param flatDir 展平目标目录路径
// @param upperDir upper 层目录路径
// @return 展平统计信息和可能的错误
func FlattenOverlay(flatDir, upperDir string) (*FlattenStats, error) {
	st := &FlattenStats{}
	resetInodeMap()
	if err := walkUpper(upperDir, flatDir, "", st); err != nil {
		return st, err
	}
	return st, nil
}

// walkUpper 递归遍历 upper 层目录，对每个条目按类型处理
//
// 处理顺序（按优先级）:
//  1. Whiteout (主次设备号均为 0 的字符设备): 删除展平目录中的对应文件
//  2. 符号链接: 复制链接目标并保持所有权/时间戳
//  3. 目录: 检查 opaque 标记，创建/合并目录并递归处理
//  4. 普通文件: 先尝试硬链接去重，否则 reflink 复制
//  5. 特殊文件: 使用 mknod 复制
//
// @param upperBase upper 层基础路径
// @param flatBase 展平目标基础路径
// @param rel 当前相对路径
// @param st 统计计数器
// @return error 遍历或复制过程中的错误
func walkUpper(upperBase, flatBase, rel string, st *FlattenStats) error {
	upperDir := filepath.Join(upperBase, rel)

	entries, err := os.ReadDir(upperDir)
	if err != nil {
		return fmt.Errorf(i18n.T("ovl.readdir"), upperDir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		entryRel := filepath.Join(rel, name)
		upperPath := filepath.Join(upperBase, entryRel)
		flatPath := filepath.Join(flatBase, entryRel)

		fi, err := entry.Info()
		if err != nil {
			return fmt.Errorf(i18n.T("ovl.stat"), upperPath, err)
		}

		sysstat, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf(i18n.T("ovl.no.stat"), upperPath)
		}

		// Whiteout 检测: OverlayFS 用主次设备号均为 0 的字符设备表示删除
		if fi.Mode()&fs.ModeCharDevice != 0 {
			major := uint32((sysstat.Rdev >> 8) & 0xfff)
			minor := uint32(sysstat.Rdev & 0xff)
			if major == 0 && minor == 0 {
				os.RemoveAll(flatPath)
				st.Whiteouts++
				continue
			}
		}

		// 符号链接
		if fi.Mode()&fs.ModeSymlink != 0 {
			if err := copySymlink(upperPath, flatPath); err != nil {
				return err
			}
			os.Lchown(flatPath, int(sysstat.Uid), int(sysstat.Gid))
			copyTimes(sysstat, flatPath)
			st.Symlinks++
			continue
		}

		// 目录处理
		if fi.IsDir() {
			// 检查 trusted.overlay.opaque 属性，
			// "y" 表示不透明目录，需要先清空展平目录中的旧内容
			opaque := lgetxattrStr(upperPath, "trusted.overlay.opaque")
			if opaque == "y" {
				os.RemoveAll(flatPath)
				st.Opaques++
			}

			dirPerm := fs.FileMode(sysstat.Mode & 0o7777)
			if err := os.MkdirAll(flatPath, dirPerm); err != nil {
				return fmt.Errorf(i18n.T("ovl.mkdir"), flatPath, err)
			}
			os.Chmod(flatPath, dirPerm)
			os.Lchown(flatPath, int(sysstat.Uid), int(sysstat.Gid))
			copyXattrsClean(upperPath, flatPath)
			copyTimes(sysstat, flatPath)
			st.Dirs++

			if err := walkUpper(upperBase, flatBase, entryRel, st); err != nil {
				return err
			}
			continue
		}

		// 普通文件处理
		if fi.Mode().IsRegular() {
			// 对于具有多个硬链接的文件，尝试直接创建硬链接代替复制
			if sysstat.Nlink > 1 {
				if _, linked := tryHardlink(sysstat, flatPath); linked {
					st.Hardlinks++
					continue
				}
			}
			if err := reflinkCopy(upperPath, flatPath, sysstat.Mode&0o7777, sysstat); err != nil {
				return err
			}
			st.Files++
			continue
		}

		// 其他特殊文件（设备文件、FIFO 等）
		if err := copySpecial(upperPath, flatPath, sysstat); err != nil {
			return err
		}
	}
	return nil
}
