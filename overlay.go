// overlay.go — OverlayFS 层 reflink 展平引擎
//
// 将 OverlayFS upper 层变更通过 reflink 合并到展平目录。
// 单次遍历完成所有操作：遇到 whiteout 立即删除、复制时跳过
// overlay 扩展属性、不透明目录在复制前先清空旧内容。
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// FICLONE ioctl 编号，用于 btrfs/xfs 等文件系统的零拷贝 reflink 克隆
const FICLONE = 0x40049409

const overlayXattrPrefix = "trusted.overlay."

// flattenStats 记录展平过程中各类操作的统计计数
type flattenStats struct {
	whiteouts int
	opaques   int
	files     int
	symlinks  int
	dirs      int
	hardlinks int
}

// flattenOverlay 将 upper 层变更通过 reflink 合并到 flat 目录
func flattenOverlay(flatDir, upperDir string) (*flattenStats, error) {
	st := &flattenStats{}
	resetInodeMap()
	if err := walkUpper(upperDir, flatDir, "", st); err != nil {
		return st, err
	}
	return st, nil
}

// walkUpper 递归遍历 upper 层目录，对每个条目按类型处理
func walkUpper(upperBase, flatBase, rel string, st *flattenStats) error {
	upperDir := filepath.Join(upperBase, rel)

	entries, err := os.ReadDir(upperDir)
	if err != nil {
		return fmt.Errorf(T("ovl.readdir"), upperDir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		entryRel := filepath.Join(rel, name)
		upperPath := filepath.Join(upperBase, entryRel)
		flatPath := filepath.Join(flatBase, entryRel)

		fi, err := entry.Info()
		if err != nil {
			return fmt.Errorf(T("ovl.stat"), upperPath, err)
		}

		sysstat, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf(T("ovl.no.stat"), upperPath)
		}

		// Whiteout: 主次设备号均为 0 的字符设备
		if fi.Mode()&fs.ModeCharDevice != 0 {
			major := uint32((sysstat.Rdev >> 8) & 0xfff)
			minor := uint32(sysstat.Rdev & 0xff)
			if major == 0 && minor == 0 {
				os.RemoveAll(flatPath)
				st.whiteouts++
				continue
			}
		}

		// 符号链接
		if fi.Mode()&fs.ModeSymlink != 0 {
			if err := ovlCopySymlink(upperPath, flatPath); err != nil {
				return err
			}
			os.Lchown(flatPath, int(sysstat.Uid), int(sysstat.Gid))
			ovlCopyTimes(sysstat, flatPath)
			st.symlinks++
			continue
		}

		// 目录
		if fi.IsDir() {
			opaque := ovlLgetxattrStr(upperPath, "trusted.overlay.opaque")
			if opaque == "y" {
				os.RemoveAll(flatPath)
				st.opaques++
			}

			dirPerm := fs.FileMode(sysstat.Mode & 0o7777)
			if err := os.MkdirAll(flatPath, dirPerm); err != nil {
				return fmt.Errorf(T("ovl.mkdir"), flatPath, err)
			}
			os.Chmod(flatPath, dirPerm)
			os.Lchown(flatPath, int(sysstat.Uid), int(sysstat.Gid))
			ovlCopyXattrsClean(upperPath, flatPath)
			ovlCopyTimes(sysstat, flatPath)
			st.dirs++

			if err := walkUpper(upperBase, flatBase, entryRel, st); err != nil {
				return err
			}
			continue
		}

		// 普通文件
		if fi.Mode().IsRegular() {
			if sysstat.Nlink > 1 {
				if _, linked := ovlTryHardlink(sysstat, flatPath); linked {
					st.hardlinks++
					continue
				}
			}
			if err := ovlReflinkCopy(upperPath, flatPath, sysstat.Mode&0o7777, sysstat); err != nil {
				return err
			}
			st.files++
			continue
		}

		// 其他特殊文件
		if err := ovlCopySpecial(upperPath, flatPath, sysstat); err != nil {
			return err
		}
	}
	return nil
}

// ovlReflinkCopy 通过 FICLONE ioctl 实现零拷贝文件克隆
func ovlReflinkCopy(src, dst string, perm uint32, srcStat *syscall.Stat_t) error {
	os.RemoveAll(dst)

	srcFd, err := syscall.Open(src, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf(T("ovl.open.src"), src, err)
	}
	defer syscall.Close(srcFd)

	dstFd, err := syscall.Open(dst,
		syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf(T("ovl.create.dst"), dst, err)
	}
	defer syscall.Close(dstFd)

	if err := ovlIoctl(dstFd, FICLONE, srcFd); err != nil {
		if err2 := ovlCopyFallback(srcFd, dstFd); err2 != nil {
			return fmt.Errorf(T("ovl.copy"), src, dst, err2)
		}
	}

	// 先 chown 再 chmod（chown 会清除 setuid/setgid 位）
	syscall.Fchown(dstFd, int(srcStat.Uid), int(srcStat.Gid))
	syscall.Fchmod(dstFd, perm)

	ovlCopyXattrsFdClean(srcFd, dstFd)
	ovlCopyTimes(srcStat, dst)
	ovlRegisterInode(srcStat, dst)

	return nil
}

// ovlCopyFallback 在 reflink 不可用时通过 read/write 循环拷贝
func ovlCopyFallback(srcFd, dstFd int) error {
	buf := make([]byte, 128*1024)
	for {
		n, err := syscall.Read(srcFd, buf)
		if n > 0 {
			if _, werr := syscall.Write(dstFd, buf[:n]); werr != nil {
				return werr
			}
		}
		if n == 0 {
			return nil
		}
		if err != nil {
			if err == syscall.EAGAIN {
				continue
			}
			return err
		}
	}
}

func ovlIoctl(fd int, request uintptr, arg int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func ovlCopySymlink(src, dst string) error {
	os.RemoveAll(dst)
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf(T("ovl.readlink"), src, err)
	}
	return os.Symlink(target, dst)
}

func ovlCopySpecial(src, dst string, srcStat *syscall.Stat_t) error {
	os.RemoveAll(dst)
	if err := syscall.Mknod(dst, srcStat.Mode, int(srcStat.Rdev)); err != nil {
		return fmt.Errorf(T("ovl.mknod"), dst, err)
	}
	os.Lchown(dst, int(srcStat.Uid), int(srcStat.Gid))
	ovlCopyXattrsClean(src, dst)
	ovlCopyTimes(srcStat, dst)
	return nil
}

// ── 扩展属性操作 ─────────────────────────────────────────

func ovlLgetxattrStr(path, attr string) string {
	buf := make([]byte, 256)
	sz, err := ovlLgetxattr(path, attr, buf)
	if err != nil || sz <= 0 {
		return ""
	}
	return string(buf[:sz])
}

func ovlCopyXattrsClean(src, dst string) {
	names, err := ovlLlistxattr(src)
	if err != nil || len(names) == 0 {
		return
	}
	buf := make([]byte, 64*1024)
	for _, name := range names {
		if strings.HasPrefix(name, overlayXattrPrefix) {
			continue
		}
		sz, err := ovlLgetxattr(src, name, buf)
		if err != nil || sz < 0 {
			continue
		}
		ovlLsetxattr(dst, name, buf[:sz], 0)
	}
}

func ovlCopyXattrsFdClean(srcFd int, dstFd int) {
	names, err := ovlFlistxattr(srcFd)
	if err != nil || len(names) == 0 {
		return
	}
	buf := make([]byte, 64*1024)
	for _, name := range names {
		if strings.HasPrefix(name, overlayXattrPrefix) {
			continue
		}
		sz, err := ovlFgetxattr(srcFd, name, buf)
		if err != nil || sz < 0 {
			continue
		}
		ovlFsetxattr(dstFd, name, buf[:sz], 0)
	}
}

func ovlLgetxattr(path, attr string, buf []byte) (int, error) {
	pathb, err := syscall.BytePtrFromString(path)
	if err != nil {
		return 0, err
	}
	attrb, err := syscall.BytePtrFromString(attr)
	if err != nil {
		return 0, err
	}
	var bufp unsafe.Pointer
	if len(buf) > 0 {
		bufp = unsafe.Pointer(&buf[0])
	}
	r, _, errno := syscall.Syscall6(
		syscall.SYS_LGETXATTR,
		uintptr(unsafe.Pointer(pathb)),
		uintptr(unsafe.Pointer(attrb)),
		uintptr(bufp),
		uintptr(len(buf)),
		0, 0,
	)
	if errno != 0 {
		return 0, errno
	}
	return int(r), nil
}

func ovlLsetxattr(path, attr string, data []byte, flags int) error {
	pathb, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}
	attrb, err := syscall.BytePtrFromString(attr)
	if err != nil {
		return err
	}
	var datap unsafe.Pointer
	if len(data) > 0 {
		datap = unsafe.Pointer(&data[0])
	}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_LSETXATTR,
		uintptr(unsafe.Pointer(pathb)),
		uintptr(unsafe.Pointer(attrb)),
		uintptr(datap),
		uintptr(len(data)),
		uintptr(flags),
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func ovlLlistxattr(path string) ([]string, error) {
	pathb, err := syscall.BytePtrFromString(path)
	if err != nil {
		return nil, err
	}
	r, _, errno := syscall.Syscall(
		syscall.SYS_LLISTXATTR,
		uintptr(unsafe.Pointer(pathb)),
		0, 0,
	)
	if errno != 0 {
		return nil, errno
	}
	sz := int(r)
	if sz == 0 {
		return nil, nil
	}
	buf := make([]byte, sz)
	r, _, errno = syscall.Syscall(
		syscall.SYS_LLISTXATTR,
		uintptr(unsafe.Pointer(pathb)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(sz),
	)
	if errno != 0 {
		return nil, errno
	}
	return ovlParseXattrNames(buf[:int(r)]), nil
}

func ovlFgetxattr(fd int, attr string, buf []byte) (int, error) {
	attrb, err := syscall.BytePtrFromString(attr)
	if err != nil {
		return 0, err
	}
	var bufp unsafe.Pointer
	if len(buf) > 0 {
		bufp = unsafe.Pointer(&buf[0])
	}
	r, _, errno := syscall.Syscall6(
		syscall.SYS_FGETXATTR,
		uintptr(fd),
		uintptr(unsafe.Pointer(attrb)),
		uintptr(bufp),
		uintptr(len(buf)),
		0, 0,
	)
	if errno != 0 {
		return 0, errno
	}
	return int(r), nil
}

func ovlFsetxattr(fd int, attr string, data []byte, flags int) error {
	attrb, err := syscall.BytePtrFromString(attr)
	if err != nil {
		return err
	}
	var datap unsafe.Pointer
	if len(data) > 0 {
		datap = unsafe.Pointer(&data[0])
	}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_FSETXATTR,
		uintptr(fd),
		uintptr(unsafe.Pointer(attrb)),
		uintptr(datap),
		uintptr(len(data)),
		uintptr(flags),
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func ovlFlistxattr(fd int) ([]string, error) {
	r, _, errno := syscall.Syscall(
		syscall.SYS_FLISTXATTR,
		uintptr(fd),
		0, 0,
	)
	if errno != 0 {
		return nil, errno
	}
	sz := int(r)
	if sz == 0 {
		return nil, nil
	}
	buf := make([]byte, sz)
	r, _, errno = syscall.Syscall(
		syscall.SYS_FLISTXATTR,
		uintptr(fd),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(sz),
	)
	if errno != 0 {
		return nil, errno
	}
	return ovlParseXattrNames(buf[:int(r)]), nil
}

func ovlParseXattrNames(buf []byte) []string {
	var names []string
	for len(buf) > 0 {
		idx := 0
		for idx < len(buf) && buf[idx] != 0 {
			idx++
		}
		if idx > 0 {
			names = append(names, string(buf[:idx]))
		}
		if idx >= len(buf) {
			break
		}
		buf = buf[idx+1:]
	}
	return names
}

// ── 时间戳 ───────────────────────────────────────────────

func ovlCopyTimes(srcStat *syscall.Stat_t, dst string) {
	atime := syscall.Timespec{Sec: srcStat.Atim.Sec, Nsec: srcStat.Atim.Nsec}
	mtime := syscall.Timespec{Sec: srcStat.Mtim.Sec, Nsec: srcStat.Mtim.Nsec}
	times := [2]syscall.Timespec{atime, mtime}
	ovlUtimensat(dst, times)
}

func ovlUtimensat(path string, times [2]syscall.Timespec) error {
	pathb, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_UTIMENSAT,
		uintptr(0xffffffffffffff9c), // AT_FDCWD
		uintptr(unsafe.Pointer(pathb)),
		uintptr(unsafe.Pointer(&times[0])),
		0x100, // AT_SYMLINK_NOFOLLOW
		0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// ── 硬链接去重 ───────────────────────────────────────────

type ovlInodeKey struct {
	dev uint64
	ino uint64
}

var ovlInodeMap map[ovlInodeKey]string

func resetInodeMap() {
	ovlInodeMap = make(map[ovlInodeKey]string)
}

func ovlRegisterInode(st *syscall.Stat_t, flatPath string) {
	if st.Nlink > 1 {
		key := ovlInodeKey{dev: st.Dev, ino: st.Ino}
		if _, exists := ovlInodeMap[key]; !exists {
			ovlInodeMap[key] = flatPath
		}
	}
}

func ovlTryHardlink(st *syscall.Stat_t, flatPath string) (string, bool) {
	key := ovlInodeKey{dev: st.Dev, ino: st.Ino}
	existing, ok := ovlInodeMap[key]
	if !ok {
		return "", false
	}
	os.RemoveAll(flatPath)
	if err := os.Link(existing, flatPath); err != nil {
		return "", false
	}
	return existing, true
}
