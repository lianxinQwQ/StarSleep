package overlay

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"starsleep/internal/i18n"
)

// ficlone ioctl 编号，用于 btrfs/xfs 等文件系统的零拷贝 reflink 克隆
const ficlone = 0x40049409

// reflinkCopy 通过 FICLONE ioctl 实现零拷贝文件克隆
func reflinkCopy(src, dst string, perm uint32, srcStat *syscall.Stat_t) error {
	os.RemoveAll(dst)
	srcFd, err := syscall.Open(src, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf(i18n.T("ovl.open.src"), src, err)
	}
	defer syscall.Close(srcFd)
	dstFd, err := syscall.Open(dst,
		syscall.O_WRONLY|syscall.O_CREAT|syscall.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf(i18n.T("ovl.create.dst"), dst, err)
	}
	defer syscall.Close(dstFd)
	if err := ioctlFiclone(dstFd, ficlone, srcFd); err != nil {
		if err2 := copyFallback(srcFd, dstFd); err2 != nil {
			return fmt.Errorf(i18n.T("ovl.copy"), src, dst, err2)
		}
	}
	// 先 chown 再 chmod（chown 会清除 setuid/setgid 位）
	syscall.Fchown(dstFd, int(srcStat.Uid), int(srcStat.Gid))
	syscall.Fchmod(dstFd, perm)
	copyXattrsFdClean(srcFd, dstFd)
	copyTimes(srcStat, dst)
	registerInode(srcStat, dst)
	return nil
}

// copyFallback 在 reflink 不可用时通过 read/write 循环拷贝
func copyFallback(srcFd, dstFd int) error {
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

func ioctlFiclone(fd int, request uintptr, arg int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func copySymlink(src, dst string) error {
	os.RemoveAll(dst)
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf(i18n.T("ovl.readlink"), src, err)
	}
	return os.Symlink(target, dst)
}

func copySpecial(src, dst string, srcStat *syscall.Stat_t) error {
	os.RemoveAll(dst)
	if err := syscall.Mknod(dst, srcStat.Mode, int(srcStat.Rdev)); err != nil {
		return fmt.Errorf(i18n.T("ovl.mknod"), dst, err)
	}
	os.Lchown(dst, int(srcStat.Uid), int(srcStat.Gid))
	copyXattrsClean(src, dst)
	copyTimes(srcStat, dst)
	return nil
}

// ── 时间戳 ───────────────────────────────────────────────

func copyTimes(srcStat *syscall.Stat_t, dst string) {
	atime := syscall.Timespec{Sec: srcStat.Atim.Sec, Nsec: srcStat.Atim.Nsec}
	mtime := syscall.Timespec{Sec: srcStat.Mtim.Sec, Nsec: srcStat.Mtim.Nsec}
	times := [2]syscall.Timespec{atime, mtime}
	utimensat(dst, times)
}

func utimensat(path string, times [2]syscall.Timespec) error {
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

type inodeKey struct {
	dev uint64
	ino uint64
}

var inodeMap map[inodeKey]string

func resetInodeMap() {
	inodeMap = make(map[inodeKey]string)
}

func registerInode(st *syscall.Stat_t, flatPath string) {
	if st.Nlink > 1 {
		key := inodeKey{dev: st.Dev, ino: st.Ino}
		if _, exists := inodeMap[key]; !exists {
			inodeMap[key] = flatPath
		}
	}
}

func tryHardlink(st *syscall.Stat_t, flatPath string) (string, bool) {
	key := inodeKey{dev: st.Dev, ino: st.Ino}
	existing, ok := inodeMap[key]
	if !ok {
		return "", false
	}
	os.RemoveAll(flatPath)
	if err := os.Link(existing, flatPath); err != nil {
		return "", false
	}
	return existing, true
}
