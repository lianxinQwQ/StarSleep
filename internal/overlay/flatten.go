// flatten.go — OverlayFS 层 reflink 展平引擎
//
// 将 OverlayFS upper 层变更通过 reflink 合并到展平目录。
// 单次遍历完成所有操作：遇到 whiteout 立即删除、复制时跳过
// overlay 扩展属性、不透明目录在复制前先清空旧内容。
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
	Whiteouts int
	Opaques   int
	Files     int
	Symlinks  int
	Dirs      int
	Hardlinks int
}

// FlattenOverlay 将 upper 层变更通过 reflink 合并到 flat 目录
func FlattenOverlay(flatDir, upperDir string) (*FlattenStats, error) {
	st := &FlattenStats{}
	resetInodeMap()
	if err := walkUpper(upperDir, flatDir, "", st); err != nil {
		return st, err
	}
	return st, nil
}

// walkUpper 递归遍历 upper 层目录，对每个条目按类型处理
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

		// Whiteout: 主次设备号均为 0 的字符设备
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

		// 目录
		if fi.IsDir() {
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

		// 普通文件
		if fi.Mode().IsRegular() {
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

		// 其他特殊文件
		if err := copySpecial(upperPath, flatPath, sysstat); err != nil {
			return err
		}
	}
	return nil
}
