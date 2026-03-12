// xattr.go — 扩展属性 (xattr) 操作封装
package flatten

import (
	"strings"
	"syscall"
	"unsafe"
)

const overlayXattrPrefix = "trusted.overlay."

func lgetxattrStr(path, attr string) string {
	buf := make([]byte, 256)
	sz, err := lgetxattr(path, attr, buf)
	if err != nil || sz <= 0 {
		return ""
	}
	return string(buf[:sz])
}

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
