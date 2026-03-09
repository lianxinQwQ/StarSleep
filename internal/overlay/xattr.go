// xattr.go — 扩展属性 (xattr) 操作封装
//
// 提供对文件扩展属性的底层操作，包括:
//   - lgetxattr / lsetxattr / llistxattr: 基于路径的操作（不跟随符号链接）
//   - fgetxattr / fsetxattr / flistxattr: 基于文件描述符的操作
//   - copyXattrsClean / copyXattrsFdClean: 复制扩展属性时跳过 overlay 专属属性
package overlay

import (
	"strings"
	"syscall"
	"unsafe"
)

// overlayXattrPrefix OverlayFS 专属扩展属性前缀，复制时应跳过
const overlayXattrPrefix = "trusted.overlay."

// lgetxattrStr 读取文件的扩展属性并返回字符串值
//
// @param path 文件路径
// @param attr 属性名（如 "trusted.overlay.opaque"）
// @return 属性值字符串，读取失败时返回空字符串
func lgetxattrStr(path, attr string) string {
	buf := make([]byte, 256)
	sz, err := lgetxattr(path, attr, buf)
	if err != nil || sz <= 0 {
		return ""
	}
	return string(buf[:sz])
}

// copyXattrsClean 复制文件的所有扩展属性（跳过 overlay 专属属性）
//
// 使用基于路径的 llistxattr/lgetxattr/lsetxattr 接口。
//
// @param src 源文件路径
// @param dst 目标文件路径
func copyXattrsClean(src, dst string) {
	names, err := llistxattr(src)
	if err != nil || len(names) == 0 {
		return
	}
	buf := make([]byte, 64*1024)
	for _, name := range names {
		if strings.HasPrefix(name, overlayXattrPrefix) {
			continue
		}
		sz, err := lgetxattr(src, name, buf)
		if err != nil || sz < 0 {
			continue
		}
		lsetxattr(dst, name, buf[:sz], 0)
	}
}

// copyXattrsFdClean 通过文件描述符复制扩展属性（跳过 overlay 专属属性）
//
// 使用基于 fd 的 flistxattr/fgetxattr/fsetxattr 接口，
// 用于在文件已打开时避免额外的路径解析开销。
//
// @param srcFd 源文件描述符
// @param dstFd 目标文件描述符
func copyXattrsFdClean(srcFd int, dstFd int) {
	names, err := flistxattr(srcFd)
	if err != nil || len(names) == 0 {
		return
	}
	buf := make([]byte, 64*1024)
	for _, name := range names {
		if strings.HasPrefix(name, overlayXattrPrefix) {
			continue
		}
		sz, err := fgetxattr(srcFd, name, buf)
		if err != nil || sz < 0 {
			continue
		}
		fsetxattr(dstFd, name, buf[:sz], 0)
	}
}

// lgetxattr 调用 lgetxattr(2) 系统调用读取文件扩展属性
//
// 不跟随符号链接（l 前缀）。
//
// @param path 文件路径
// @param attr 属性名
// @param buf 接收缓冲区
// @return 读取的字节数和可能的错误
func lgetxattr(path, attr string, buf []byte) (int, error) {
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

// lsetxattr 调用 lsetxattr(2) 系统调用设置文件扩展属性
//
// @param path 文件路径
// @param attr 属性名
// @param data 属性值
// @param flags 标志位（如 XATTR_CREATE / XATTR_REPLACE）
// @return error 系统调用失败时返回 errno
func lsetxattr(path, attr string, data []byte, flags int) error {
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

// llistxattr 调用 llistxattr(2) 系统调用列出文件的所有扩展属性名
//
// 两步调用：先获取需要的缓冲区大小，再读取实际内容。
//
// @param path 文件路径
// @return 属性名切片和可能的错误
func llistxattr(path string) ([]string, error) {
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
	return parseXattrNames(buf[:int(r)]), nil
}

// fgetxattr 调用 fgetxattr(2) 系统调用通过文件描述符读取扩展属性
//
// @param fd 文件描述符
// @param attr 属性名
// @param buf 接收缓冲区
// @return 读取的字节数和可能的错误
func fgetxattr(fd int, attr string, buf []byte) (int, error) {
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

// fsetxattr 调用 fsetxattr(2) 系统调用通过文件描述符设置扩展属性
//
// @param fd 文件描述符
// @param attr 属性名
// @param data 属性值
// @param flags 标志位
// @return error 系统调用失败时返回 errno
func fsetxattr(fd int, attr string, data []byte, flags int) error {
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

// flistxattr 调用 flistxattr(2) 系统调用通过文件描述符列出所有扩展属性名
//
// @param fd 文件描述符
// @return 属性名切片和可能的错误
func flistxattr(fd int) ([]string, error) {
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
	return parseXattrNames(buf[:int(r)]), nil
}

// parseXattrNames 解析扩展属性名列表的原始字节缓冲区
//
// xattr 名称列表是以 NUL ('\0') 分隔的连续字符串。
//
// @param buf 原始字节缓冲区
// @return 解析后的属性名切片
func parseXattrNames(buf []byte) []string {
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
