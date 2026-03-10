// copyfiles 包提供文件叠加层的同步功能。
//
// 将配置目录 files/ 子目录中的文件/文件夹复制到目标根目录的指定位置,
// 实现声明式的文件叠加。所有源路径被强制限定在 files/ 子目录中,
// 防止引用外部文件。
//
// 支持两种模式:
//   - 构建模式 (SyncCopyFiles): 复制到 OverlayFS merged 挂载点中
//   - 维护模式 (CopyFilesLive): 直接复制到当前运行系统
package copyfiles

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncCopyFiles 将配置中定义的文件映射列表复制到目标根目录
//
// 从 configDir/files/ 目录读取源文件/目录，复制到 root 下的对应目标路径。
// 叠加文件的源路径始终被限定在 configDir/files/ 子目录内，
// 即使配置中写了绝对路径也会被当作相对路径拼接。
//
// @param root 目标根目录路径（构建时为 OverlayFS merged 挂载点）
// @param configDir 配置目录路径（包含 files/ 子目录）
// @param files 文件映射列表（src → dst 对）
// @throws 源目录不存在、路径验证失败、复制失败时调用 Fatal 退出
func SyncCopyFiles(root, configDir string, files []config.FileMapping) {
	filesBase := filepath.Join(configDir, config.FilesDir)

	// 验证叠加文件源目录存在
	if _, err := os.Stat(filesBase); os.IsNotExist(err) {
		util.Fatal(i18n.T("copyfiles.base.not.exist", filesBase))
	}

	fmt.Println(i18n.T("copyfiles.start"))

	for _, fm := range files {
		copyMapping(root, filesBase, fm)
	}

	fmt.Println(i18n.T("copyfiles.done", len(files)))
}

// CopyFilesLive 将配置中定义的文件映射列表直接复制到当前运行系统
//
// 与 SyncCopyFiles 相同的逻辑，但 root 为 "/"，直接操作当前系统。
//
// @param configDir 配置目录路径
// @param files 文件映射列表
func CopyFilesLive(configDir string, files []config.FileMapping) {
	SyncCopyFiles("/", configDir, files)
}

// copyMapping 执行单个文件映射的复制操作
//
// 根据源路径是文件还是目录，分别调用对应的复制命令:
//   - 目录: cp -a --reflink=auto src/ dst/
//   - 文件: cp -a --reflink=auto src dst
//
// @param root 目标根目录路径
// @param filesBase 叠加文件源基础目录（configDir/files/）
// @param fm 文件映射定义
// @throws 路径验证失败或复制失败时调用 Fatal 退出
func copyMapping(root, filesBase string, fm config.FileMapping) {
	// 安全拼接源路径，确保不逃逸出 filesBase
	src, err := SafeJoin(filesBase, fm.Src)
	if err != nil {
		util.Fatal(i18n.T("copyfiles.src.invalid", fm.Src, err))
	}

	// 目标路径拼接到 root 下，同样做安全清理
	dst := filepath.Join(root, filepath.Clean("/"+fm.Dst))

	// 验证源路径存在
	fi, err := os.Stat(src)
	if err != nil {
		util.Fatal(i18n.T("copyfiles.src.not.exist", fm.Src, src))
	}

	fmt.Println(i18n.T("copyfiles.copy.item", fm.Src, fm.Dst))

	if fi.IsDir() {
		copyDir(src, dst)
	} else {
		copyFile(src, dst)
	}
}

// copyDir 递归复制整个目录到目标路径
//
// 使用 cp -a --reflink=auto 保持所有文件属性和符号链接，
// 并在支持的文件系统上尝试 reflink 零拷贝。
//
// @param src 源目录路径
// @param dst 目标目录路径
// @throws 复制失败时调用 Fatal 退出
func copyDir(src, dst string) {
	// 确保目标父目录存在
	os.MkdirAll(filepath.Dir(dst), 0o755)
	// 先移除目标以确保完整覆盖
	os.RemoveAll(dst)
	if err := util.Run("cp", "-a", "--reflink=auto", src, dst); err != nil {
		util.Fatal(i18n.T("copyfiles.copy.dir.failed", src, err))
	}
}

// copyFile 复制单个文件到目标路径
//
// 使用 cp -a --reflink=auto 保持文件属性，
// 目标文件如已存在则覆盖。
//
// @param src 源文件路径
// @param dst 目标文件路径
// @throws 复制失败时调用 Fatal 退出
func copyFile(src, dst string) {
	// 确保目标父目录存在
	os.MkdirAll(filepath.Dir(dst), 0o755)
	if err := util.Run("cp", "-a", "--reflink=auto", src, dst); err != nil {
		util.Fatal(i18n.T("copyfiles.copy.file.failed", src, err))
	}
}
