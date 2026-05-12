// config_fetch.go — 从 GitHub 拉取预设配置
package install

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

// profileFiles 定义每个预设配置需要从仓库下载的文件列表
var profileFiles = []string{
	"config.yaml",
	"layers/base.yaml",
	"layers/tools.yaml",
	"layers/services.yaml",
}

// profileExtraFiles 定义每个预设额外需要的文件
var profileExtraFiles = map[string][]string{
	"minimal": {},
	"gnome":   {"layers/desktop.yaml"},
	"dev":     {"layers/desktop.yaml", "layers/dev-tools.yaml", "layers/aurtools.yaml"},
}

// fetchProfile 从 GitHub 拉取预设配置到默认配置目录
func fetchProfile(profile, repoURL string) {
	fmt.Println(i18n.T("install.fetch.config", profile))

	configDir := config.DefaultConfigDir
	os.MkdirAll(filepath.Join(configDir, "layers"), 0o755)
	os.MkdirAll(filepath.Join(configDir, "files"), 0o755)

	files := append(profileFiles, profileExtraFiles[profile]...)
	for _, f := range files {
		url := fmt.Sprintf("%s/%s/%s", repoURL, profile, f)
		fmt.Println(i18n.T("install.fetch.downloading", url))

		data, err := downloadFile(url)
		if err != nil {
			util.Fatal(i18n.T("install.fetch.failed", profile, err, configDir))
		}

		dst := filepath.Join(configDir, f)
		os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			util.Fatal(fmt.Sprintf("写入配置文件失败 %s: %v", dst, err))
		}
	}

	fmt.Println(i18n.T("install.fetch.done"))
}

// downloadFile 下载单个文件并返回内容
func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
