// config_fetch.go — 从 GitHub Archive API 拉取预设配置
package install

import (
	"fmt"
	"os"
	"path/filepath"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// fetchProfile 通过 GitHub Archive API 获取仓库快照，提取指定 profile 配置
//
// GitHub 按需生成 tar.gz（非仓库中提交的压缩包），用管道提取 temp-config/<profile>/ 子目录。
// 新增层文件无需改代码，只提交到仓库即可。
func fetchProfile(profile, repoURL string) {
	fmt.Println(i18n.T("install.fetch.config", profile))

	archiveURL := repoURL + "/archive/refs/heads/main.tar.gz"
	fmt.Println(i18n.T("install.fetch.downloading", archiveURL))

	configDir := config.DefaultConfigDir
	os.MkdirAll(filepath.Join(configDir, "layers"), 0o755)
	os.MkdirAll(filepath.Join(configDir, "files"), 0o755)

	// curl 管道解压：只提取 temp-config/<profile>/ 下的内容
	// GitHub archive 内部路径: <repo>-main/temp-config/<profile>/
	// --strip-components=3 去掉前三层目录，得到 config.yaml layers/ files/
	pipe := fmt.Sprintf(
		`curl -Lfs '%s' | tar -xz --strip-components=3 -C '%s' '*/temp-config/%s/'`,
		archiveURL, configDir, profile,
	)
	if err := util.Run("sh", "-c", pipe); err != nil {
		util.Fatal(i18n.T("install.fetch.failed", profile, err, configDir))
	}

	fmt.Println(i18n.T("install.fetch.done"))
}
