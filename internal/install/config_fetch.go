// config_fetch.go — 从 GitHub Archive API 拉取预设配置
package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"

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

	if err := fetchProfileArchive(archiveURL, configDir, profile); err != nil {
		util.Fatal(i18n.T("install.fetch.failed", profile, err, configDir))
	}

	fmt.Println(i18n.T("install.fetch.done"))
}

func fetchProfileArchive(archiveURL, configDir, profile string) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	reader, writer := io.Pipe()
	curl := exec.Command("curl", "-Lfs", "-A", "starsleep", "--", archiveURL)
	tar := exec.Command("tar",
		"-xz",
		"--wildcards",
		"--strip-components=3",
		"-C", configDir,
		"--",
		fmt.Sprintf("*/temp-config/%s/", profile),
	)

	curl.Stdout = writer
	curl.Stderr = os.Stderr
	tar.Stdin = reader
	tar.Stdout = os.Stdout
	tar.Stderr = os.Stderr

	if err := tar.Start(); err != nil {
		reader.Close()
		writer.Close()
		return err
	}
	if err := curl.Start(); err != nil {
		writer.CloseWithError(err)
		reader.CloseWithError(err)
		tar.Wait()
		return err
	}

	curlErr := curl.Wait()
	writer.CloseWithError(curlErr)
	tarErr := tar.Wait()
	reader.Close()

	if curlErr != nil {
		return curlErr
	}
	return tarErr
}
