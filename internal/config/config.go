package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"starsleep/internal/i18n"
	"starsleep/internal/util"

	"gopkg.in/yaml.v3"
)

// LayerConfig 表示一个配置层的 YAML 结构
type LayerConfig struct {
	Name     string   `yaml:"name"`
	Helper   string   `yaml:"helper"`
	Packages []string `yaml:"packages"`
	Services []string `yaml:"services"`
}

// loadLayerConfig 从 YAML 文件加载单个层配置
func loadLayerConfig(path string) (*LayerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("cfg.read"), path, err)
	}
	var cfg LayerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf(i18n.T("cfg.parse"), path, err)
	}
	return &cfg, nil
}

// LoadAllLayers 加载配置目录下所有层配置，按文件名排序
func LoadAllLayers(configDir string) ([]*LayerConfig, []string, error) {
	layersDir := filepath.Join(configDir, "layers")
	matches, err := filepath.Glob(filepath.Join(layersDir, "*.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf(i18n.T("cfg.scan"), err)
	}
	if len(matches) == 0 {
		return nil, nil, fmt.Errorf(i18n.T("cfg.no.files"), layersDir)
	}
	sort.Strings(matches)
	var configs []*LayerConfig
	for _, path := range matches {
		cfg, err := loadLayerConfig(path)
		if err != nil {
			return nil, nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, matches, nil
}

// LoadInheritList 从 inherit.list 文件加载继承路径列表
func LoadInheritList(configDir string) ([]string, error) {
	path := filepath.Join(configDir, "inherit.list")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var paths []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths, scanner.Err()
}

// BuildCumulativePkgs 构建到指定层为止的累积包列表
func BuildCumulativePkgs(layers []*LayerConfig, upToIndex int) []string {
	var pkgs []string
	for i := 0; i <= upToIndex && i < len(layers); i++ {
		if len(layers[i].Packages) > 0 {
			pkgs = append(pkgs, layers[i].Packages...)
		}
	}
	return pkgs
}

// BuildCumulativeServices 构建到指定层为止的累积服务列表
func BuildCumulativeServices(layers []*LayerConfig, upToIndex int) []string {
	var svcs []string
	for i := 0; i <= upToIndex && i < len(layers); i++ {
		if len(layers[i].Services) > 0 {
			svcs = append(svcs, layers[i].Services...)
		}
	}
	return svcs
}

// ParseConfigFlags 从参数中提取 -c/--config 和 -cp/--copy，返回配置目录和剩余参数
func ParseConfigFlags(defaultConfigDir string, args []string) (configDir string, remaining []string) {
	configDir = defaultConfigDir
	var copyFrom string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-c", "--config":
			i++
			if i < len(args) {
				configDir = args[i]
			}
		case "-cp", "--copy":
			i++
			if i < len(args) {
				copyFrom = args[i]
			}
		default:
			remaining = append(remaining, args[i])
		}
		i++
	}
	if copyFrom != "" {
		if err := CopyConfig(copyFrom, defaultConfigDir); err != nil {
			util.Fatal(i18n.T("cfg.copy.failed", err))
		}
		configDir = defaultConfigDir
		util.LogMsg(i18n.T("cfg.copied"), copyFrom, defaultConfigDir)
	}
	return
}

// CopyConfig 将源配置目录的内容复制到目标目录
func CopyConfig(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf(i18n.T("cfg.src.not.exist"), src)
	}
	if !fi.IsDir() {
		return fmt.Errorf(i18n.T("cfg.src.not.dir"), src)
	}
	dstLayers := filepath.Join(dst, "layers")
	os.MkdirAll(dstLayers, 0o755)
	srcLayers := filepath.Join(src, "layers")
	entries, err := os.ReadDir(srcLayers)
	if err != nil {
		return fmt.Errorf(i18n.T("cfg.read.layers"), err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcLayers, entry.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstLayers, entry.Name()), data, 0o644); err != nil {
			return err
		}
	}
	inheritSrc := filepath.Join(src, "inherit.list")
	if _, err := os.Stat(inheritSrc); err == nil {
		data, err := os.ReadFile(inheritSrc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, "inherit.list"), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
