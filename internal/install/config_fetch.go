// config_fetch.go — 从 GitHub Archive API 拉取预设配置
package install

import (
	"fmt"

	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// fetchProfile 通过 GitHub Archive API 获取仓库快照，提取指定 profile 配置到目标 starsleep/config/
func fetchProfile(profile, repoURL, branch string) {
	fmt.Println(i18n.T("install.fetch.config", profile))

	if branch == "" {
		branch = "main"
	}

	archiveURL := fmt.Sprintf("%s/archive/refs/heads/%s.tar.gz", repoURL, branch)
	fmt.Println(i18n.T("install.fetch.downloading", archiveURL))

	configDir := TargetConfigDir

	// curl 管道解压：只提取 temp-config/<profile>/ 下的内容
	// GitHub archive 内部路径: <repo>-<branch>/temp-config/<profile>/
	// --strip-components=3 去掉前三层目录，得到 config.yaml layers/ files/
	pipe := fmt.Sprintf(
		`set -o pipefail && curl -Lfs -A 'starsleep' '%s' | tar -xz --wildcards --strip-components=3 -C '%s' '*/temp-config/%s/'`,
		archiveURL, configDir, profile,
	)
	if err := util.Run("sh", "-c", pipe); err != nil {
		util.Fatal(i18n.T("install.fetch.failed", profile, err, configDir))
	}

	fmt.Println(i18n.T("install.fetch.done"))
}
