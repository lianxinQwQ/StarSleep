// overlay 包提供 OverlayFS 层的 reflink 展平合并功能。
//
// copy.go 包含文件复制相关的底层函数：
// reflink 克隆、read/write 回退复制、符号链接复制、特殊文件复制、
// 时间戳保持以及硬链接去重。
package overlay

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"starsleep/internal/i18n"
)

// ficlone ioctl 编号，用于 btrfs/xfs 等文件系统的零拷贝 reflink 克隆。
// 通过引用计数共享底层数据块，避免实际数据复制。
const ficlone = 0x40049409

// reflinkCopy 通过 FICLONE ioctl 实现零拷贝文件克隆
//
// 执行步骤:
//  1. 删除目标文件（如存在）
//  2. 尝试 FICLONE ioctl 零拷贝
//  3. 如 ioctl 失败，回退到 read/write 循环复制
//  4. 设置所有权、权限、扩展属性和时间戳
//  5. 注册硬链接 inode 以便后续去重
//
// @param src 源文件路径
// @param dst 目标文件路径
// @param perm 文件权限位
// @param srcStat 源文件的 syscall.Stat_t 信息
// @return error 复制失败时返回错误
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
	// 先 chown 再 chmod（chown 会清除 setuid/setgid 位，所以必须按此顺序）
	syscall.Fchown(dstFd, int(srcStat.Uid), int(srcStat.Gid))
	syscall.Fchmod(dstFd, perm)
	// 复制扩展属性（跳过 overlay 专属属性）
	copyXattrsFdClean(srcFd, dstFd)
	// 保持源文件的访问/修改时间戳
	copyTimes(srcStat, dst)
	// 注册 inode 以便后续硬链接去重
	registerInode(srcStat, dst)
	return nil
}

// copyFallback 在 reflink 不可用时通过 read/write 循环拷贝
//
// 使用 128KB 缓冲区逐块读写，处理 EAGAIN 临时错误。
//
// @param srcFd 源文件描述符
// @param dstFd 目标文件描述符
// @return error 读写失败时返回错误
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

// ioctlFiclone 执行 FICLONE ioctl 系统调用
//
// @param fd 目标文件描述符
// @param request ioctl 请求编号
// @param arg 源文件描述符
// @return error errno 非零时返回错误
func ioctlFiclone(fd int, request uintptr, arg int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// copySymlink 复制符号链接
//
// 读取源符号链接的目标并在 dst 创建相同的符号链接。
//
// @param src 源符号链接路径
// @param dst 目标符号链接路径
// @return error 读取或创建失败时返回错误
func copySymlink(src, dst string) error {
	os.RemoveAll(dst)
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf(i18n.T("ovl.readlink"), src, err)
	}
	return os.Symlink(target, dst)
}

// copySpecial 复制特殊文件（设备文件、FIFO 等）
//
// 使用 mknod 创建相同类型的特殊文件，并复制所有权、扩展属性和时间戳。
//
// @param src 源特殊文件路径
// @param dst 目标特殊文件路径
// @param srcStat 源文件的 syscall.Stat_t 信息
// @return error 创建失败时返回错误
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

// ── 时间戳处理 ─────────────────────────────────────────

// copyTimes 复制源文件的访问时间和修改时间到目标文件
//
// @param srcStat 源文件的 stat 信息
// @param dst 目标文件路径
func copyTimes(srcStat *syscall.Stat_t, dst string) {
	atime := syscall.Timespec{Sec: srcStat.Atim.Sec, Nsec: srcStat.Atim.Nsec}
	mtime := syscall.Timespec{Sec: srcStat.Mtim.Sec, Nsec: srcStat.Mtim.Nsec}
	times := [2]syscall.Timespec{atime, mtime}
	utimensat(dst, times)
}

// utimensat 使用 utimensat(2) 系统调用设置文件时间戳
//
// 使用 AT_SYMLINK_NOFOLLOW 标志，不跟随符号链接。
//
// @param path 目标文件路径
// @param times 包含 [atime, mtime] 的时间戳数组
// @return error 系统调用失败时返回 errno
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

// inodeKey 用于硬链接去重的 inode 唯一标识（设备号 + inode 号）
type inodeKey struct {
	dev uint64
	ino uint64
}

// inodeMap 全局 inode 映射表，记录已复制文件的 inode 到展平路径的映射
var inodeMap map[inodeKey]string

// resetInodeMap 重置 inode 映射表，在每次展平操作开始前调用
func resetInodeMap() {
	inodeMap = make(map[inodeKey]string)
}

// registerInode 注册具有多个硬链接的文件的 inode 信息
//
// 只有当 nlink > 1 时才记录，后续遇到相同 inode 时可以直接创建硬链接。
//
// @param st 文件的 syscall.Stat_t 信息
// @param flatPath 文件在展平目录中的路径
func registerInode(st *syscall.Stat_t, flatPath string) {
	if st.Nlink > 1 {
		key := inodeKey{dev: st.Dev, ino: st.Ino}
		if _, exists := inodeMap[key]; !exists {
			inodeMap[key] = flatPath
		}
	}
}

// tryHardlink 尝试为具有相同 inode 的文件创建硬链接
//
// 如果该 inode 已经在 inodeMap 中注册，则直接创建硬链接而非复制文件。
//
// @param st 文件的 syscall.Stat_t 信息
// @param flatPath 目标展平路径
// @return 已存在的链接目标路径和是否成功创建硬链接
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
