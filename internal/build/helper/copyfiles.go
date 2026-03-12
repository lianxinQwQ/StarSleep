// copyfiles — 文件叠加复制 helper
package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// SyncCopyFiles 将配置中定义的文件映射列表复制到目标根目录
func SyncCopyFiles(root, configDir string, files []config.FileMapping) {
	filesBase := filepath.Join(configDir, config.FilesDir)

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
func CopyFilesLive(configDir string, files []config.FileMapping) {
	SyncCopyFiles("/", configDir, files)
}

func copyMapping(root, filesBase string, fm config.FileMapping) {
	src, err := SafeJoin(filesBase, fm.Src)
	if err != nil {
		util.Fatal(i18n.T("copyfiles.src.invalid", fm.Src, err))
	}

	dst := filepath.Join(root, filepath.Clean("/"+fm.Dst))

	fi, err := os.Stat(src)
	if err != nil {
		util.Fatal(i18n.T("copyfiles.src.not.exist", fm.Src, src))
	}

	fmt.Println(i18n.T("copyfiles.copy.item", fm.Src, fm.Dst))

	if fi.IsDir() {
		os.MkdirAll(filepath.Dir(dst), 0o755)
		os.RemoveAll(dst)
		if err := util.Run("cp", "-a", "--reflink=auto", src, dst); err != nil {
			util.Fatal(i18n.T("copyfiles.copy.dir.failed", src, err))
		}
	} else {
		os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := util.Run("cp", "-a", "--reflink=auto", src, dst); err != nil {
			util.Fatal(i18n.T("copyfiles.copy.file.failed", src, err))
		}
	}
}

// SafeJoin 将不可信的相对路径安全地拼接到基础目录下
func SafeJoin(base, rel string) (string, error) {
	cleaned := filepath.Clean("/" + rel)
	result := filepath.Join(base, cleaned)
	cleanBase := filepath.Clean(base)
	if result != cleanBase && !strings.HasPrefix(result, cleanBase+string(filepath.Separator)) {
		return "", fmt.Errorf(i18n.T("copyfiles.path.escape"), rel)
	}
	return result, nil
}
