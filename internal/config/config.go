// config 包提供 StarSleep 配置文件的加载、解析和管理功能。
//
// 配置目录结构:
//
//	config/
//	├── layers/        # 层定义 YAML 文件（按文件名排序确定构建顺序）
//	└── inherit.list   # 继承路径列表
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

// FilesDir 是配置目录下存放叠加文件的子目录名。
//
// copy_files 类型的层引用的所有源文件/目录都必须位于此子目录中，
// 路径验证确保不会逃逸到该目录之外。
const FilesDir = "files"

// FileMapping 表示一个文件叠加映射对
//
// Src 为源文件/目录路径（相对于 configDir/files/ 目录），
// Dst 为目标文件/目录路径（在构建根目录中的位置）。
type FileMapping struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

// LayerConfig 表示一个配置层的 YAML 结构
//
// 每个层定义了名称、使用的工具（helper）、待安装的包列表、待启用的服务列表
// 和待叠加的文件映射列表。
// helper 类型决定了如何处理该层:
//   - pacstrap: 使用 pacstrap 初始化基础系统
//   - pacman: 使用 pacman 同步官方仓库包
//   - paru: 使用 paru 安装 AUR 包
//   - enable_service: 启用 systemd 服务
//   - copy_files: 将配置目录中的文件叠加到目标系统
type LayerConfig struct {
	Name     string        `yaml:"name"`
	Helper   string        `yaml:"helper"`
	Packages []string      `yaml:"packages"`
	Services []string      `yaml:"services"`
	Files    []FileMapping `yaml:"files"`
}

// loadLayerConfig 从 YAML 文件加载单个层配置
//
// @param path YAML 文件的绝对路径
// @return 解析后的层配置结构体指针
// @return error 文件读取或 YAML 解析失败时返回错误
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

// LoadAllLayers 加载配置目录下所有层配置，按文件名字母序排序
//
// 扫描 configDir/layers/ 下所有 *.yaml 文件，按文件名排序后逐个加载。
// 文件名序决定了层的构建顺序（如 01-base.yaml 先于 02-desktop.yaml）。
//
// @param configDir 配置目录路径
// @return 解析后的层配置切片、对应的文件路径切片、以及可能的错误
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
//
// inherit.list 文件每行一个路径，支持 # 注释和空行。
// 这些路径将在构建完成后从宿主机复制到快照中。
//
// @param configDir 配置目录路径
// @return 继承路径切片，文件不存在时返回 nil
// @return error 文件读取失败时返回错误
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
		// 去除行内注释
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
//
// 将第 0 层到第 upToIndex 层的所有包名合并为一个列表，
// 用于声明式清理的期望包集合。
//
// @param layers 所有层配置切片
// @param upToIndex 累积到的层索引（包含）
// @return 累积的包名切片
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
//
// 与 BuildCumulativePkgs 类似，将第 0 层到第 upToIndex 层的所有服务合并。
//
// @param layers 所有层配置切片
// @param upToIndex 累积到的层索引（包含）
// @return 累积的服务名切片
func BuildCumulativeServices(layers []*LayerConfig, upToIndex int) []string {
	var svcs []string
	for i := 0; i <= upToIndex && i < len(layers); i++ {
		if len(layers[i].Services) > 0 {
			svcs = append(svcs, layers[i].Services...)
		}
	}
	return svcs
}

// ParseConfigFlags 从命令行参数中提取配置相关标志
//
// 支持的标志:
//   - -c/--config <路径>: 指定配置目录
//   - -cp/--copy <路径>: 先复制配置到默认位置再使用
//
// @param defaultConfigDir 默认配置目录路径
// @param args 原始命令行参数
// @return configDir 解析得到的配置目录路径
// @return remaining 剩余未解析的参数
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
//
// 复制 src/layers/ 目录下的所有文件、src/files/ 目录（叠加文件）
// 和 src/inherit.list（如果存在）。
//
// @param src 源配置目录路径
// @param dst 目标配置目录路径
// @return error 源目录不存在、不是目录或复制失败时返回错误
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

	// 复制 files/ 叠加文件目录（如果存在）
	srcFiles := filepath.Join(src, FilesDir)
	if ffi, err := os.Stat(srcFiles); err == nil && ffi.IsDir() {
		dstFiles := filepath.Join(dst, FilesDir)
		os.RemoveAll(dstFiles)
		if err := util.Run("cp", "-a", "--reflink=auto", srcFiles, dstFiles); err != nil {
			return fmt.Errorf(i18n.T("cfg.copy.failed"), err)
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
