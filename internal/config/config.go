// config 包提供 StarSleep 配置文件的加载、解析和管理功能。
//
// 配置目录结构:
//
//	config/
//	├── config.yaml    # 主配置文件（调度层顺序、元信息、继承列表）
//	├── layers/        # 层定义 YAML 文件（支持单步骤和多步骤）
//	└── files/         # copy_files 层引用的叠加文件源目录
package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// MetaConfig 表示 config.yaml 中的 meta 段
//
// 所有字段均为可选，未设置时由调用方回退到代码中的默认常量。
type MetaConfig struct {
	WorkDir     string `yaml:"work_dir,omitempty"`
	SnapshotDir string `yaml:"snapshot_dir,omitempty"`
	PkgCache    string `yaml:"pkg_cache,omitempty"`
}

// MainConfig 表示 config.yaml 的顶层结构
type MainConfig struct {
	Meta    MetaConfig `yaml:"meta,omitempty"`
	Layers  []string   `yaml:"layers"`
	Inherit []string   `yaml:"inherit,omitempty"`
}

// FileMapping 表示一个文件叠加映射对
//
// Src 为源文件/目录路径（相对于 configDir/files/ 目录），
// Dst 为目标文件/目录路径（在构建根目录中的位置）。
type FileMapping struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

// EnvVar 表示一个环境变量设定
//
// 支持两种赋值方式:
//   - Value 非空: 使用固定值（如 LANG=zh_CN.UTF-8）
//   - HostKey 非空: 从当前主机读取指定环境变量的值
//
// 二者同时设置时 Value 优先。
type EnvVar struct {
	Key     string `yaml:"key"`
	Value   string `yaml:"value,omitempty"`
	HostKey string `yaml:"host_key,omitempty"`
}

// LayerConfig 表示一个配置层（单步骤）
//
// helper 类型决定了如何处理该层:
//   - pacstrap: 使用 pacstrap 初始化基础系统
//   - pacman: 使用 pacman 同步官方仓库包
//   - paru: 使用 paru 安装 AUR 包
//   - enable_service: 启用 systemd 服务
//   - copy_files: 将配置目录中的文件叠加到目标系统
//   - chroot-cmd: 通过 arch-chroot 在目标根中执行任意命令
//   - chroot-pacman: 通过 arch-chroot 在目标根中运行 pacman 安装包
type LayerConfig struct {
	Name     string        `yaml:"name"`
	Helper   string        `yaml:"helper"`
	Env      []EnvVar      `yaml:"env,omitempty"`
	Packages []string      `yaml:"packages,omitempty"`
	Services []string      `yaml:"services,omitempty"`
	Files    []FileMapping `yaml:"files,omitempty"`
	Commands []string      `yaml:"commands,omitempty"`
}

// LoadMainConfig 加载 config.yaml 主配置文件
//
// @param configDir 配置目录路径
// @return 解析后的主配置结构体指针
// @return error 文件读取或 YAML 解析失败时返回错误
func LoadMainConfig(configDir string) (*MainConfig, error) {
	path := filepath.Join(configDir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("cfg.read"), path, err)
	}
	var mc MainConfig
	if err := yaml.Unmarshal(data, &mc); err != nil {
		return nil, fmt.Errorf(i18n.T("cfg.parse"), path, err)
	}
	if len(mc.Layers) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("cfg.no.layers"))
	}
	return &mc, nil
}

// loadLayerFile 从 YAML 文件加载层配置
//
// 支持两种格式:
//   - 顶层对象（单步骤）: 直接解析为单个 LayerConfig
//   - 顶层列表（多步骤）: 解析为多个 LayerConfig，按文件内上下顺序排列
//
// @param path YAML 文件的绝对路径
// @return 解析后的层配置切片
// @return error 文件读取或 YAML 解析失败时返回错误
func loadLayerFile(path string) ([]*LayerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("cfg.read"), path, err)
	}

	// 先尝试解析为列表（多步骤格式）
	var multi []*LayerConfig
	if err := yaml.Unmarshal(data, &multi); err == nil && len(multi) > 0 {
		return multi, nil
	}

	// 回退为单个对象（单步骤格式）
	var single LayerConfig
	if err := yaml.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf(i18n.T("cfg.parse"), path, err)
	}
	return []*LayerConfig{&single}, nil
}

// ValidateLayerFiles 校验 config.yaml 中引用的层文件与 layers/ 目录的一致性
//
// 检查两类问题:
//   - config.yaml 引用了不存在的层文件 → 报错退出
//   - layers/ 目录中存在未被引用的文件 → 报错退出
//
// @param configDir 配置目录路径
// @param declaredFiles config.yaml 中声明的层文件名列表
// @return error 一致性校验失败时返回错误
func ValidateLayerFiles(configDir string, declaredFiles []string) error {
	layersDir := filepath.Join(configDir, "layers")

	// 检查 config.yaml 引用的文件是否都存在
	for _, name := range declaredFiles {
		path := filepath.Join(layersDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf(i18n.T("cfg.layer.not.exist"), name)
		}
	}

	// 检查 layers/ 目录中是否有未被引用的 yaml 文件
	entries, err := os.ReadDir(layersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf(i18n.T("cfg.scan"), err)
	}

	declared := make(map[string]struct{}, len(declaredFiles))
	for _, name := range declaredFiles {
		declared[name] = struct{}{}
	}

	var unreferenced []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		if _, ok := declared[name]; !ok {
			unreferenced = append(unreferenced, name)
		}
	}
	if len(unreferenced) > 0 {
		return fmt.Errorf(i18n.T("cfg.layer.unreferenced"), strings.Join(unreferenced, ", "))
	}

	return nil
}

// LoadAllLayers 加载配置目录下所有层配置，按 config.yaml 声明的顺序
//
// 从 config.yaml 读取层文件列表，校验文件一致性后按序加载。
// 每个层文件可包含一个或多个步骤（LayerConfig）。
//
// @param configDir 配置目录路径
// @return 解析后的层配置切片、主配置、以及可能的错误
func LoadAllLayers(configDir string) ([]*LayerConfig, *MainConfig, error) {
	mc, err := LoadMainConfig(configDir)
	if err != nil {
		return nil, nil, err
	}

	if err := ValidateLayerFiles(configDir, mc.Layers); err != nil {
		return nil, nil, err
	}

	layersDir := filepath.Join(configDir, "layers")
	var configs []*LayerConfig
	for _, name := range mc.Layers {
		path := filepath.Join(layersDir, name)
		steps, err := loadLayerFile(path)
		if err != nil {
			return nil, nil, err
		}
		configs = append(configs, steps...)
	}
	return configs, mc, nil
}

// LoadInheritList 从主配置中获取继承路径列表
//
// 优先使用 config.yaml 中的 inherit 段，若未定义则返回 nil。
//
// @param mc 主配置结构体指针
// @return 继承路径切片
func LoadInheritList(mc *MainConfig) []string {
	if mc == nil || len(mc.Inherit) == 0 {
		return nil
	}
	return mc.Inherit
}

// ResolveWorkDir 解析工作目录，优先使用配置文件中的值
func ResolveWorkDir(mc *MainConfig, defaultVal string) string {
	if mc != nil && mc.Meta.WorkDir != "" {
		return mc.Meta.WorkDir
	}
	return defaultVal
}

// ResolveSnapshotDir 解析快照目录，优先使用配置文件中的值
func ResolveSnapshotDir(mc *MainConfig, defaultVal string) string {
	if mc != nil && mc.Meta.SnapshotDir != "" {
		return mc.Meta.SnapshotDir
	}
	return defaultVal
}

// ResolvePkgCache 解析包缓存目录，优先使用配置文件中的值
func ResolvePkgCache(mc *MainConfig, defaultVal string) string {
	if mc != nil && mc.Meta.PkgCache != "" {
		return mc.Meta.PkgCache
	}
	return defaultVal
}

// ResolveEnv 将配置中的环境变量列表解析为 KEY=VALUE 字符串切片
//
// 对于每个 EnvVar:
//   - 如果设置了 Value，直接使用该固定值
//   - 如果设置了 HostKey，从当前主机的环境变量中读取对应值
//   - 二者同时设置时 Value 优先
//   - HostKey 指定的环境变量在主机上不存在时跳过该条目
//
// @param envVars 环境变量配置列表
// @return 解析后的 KEY=VALUE 字符串切片
func ResolveEnv(envVars []EnvVar) []string {
	var result []string
	for _, ev := range envVars {
		if ev.Value != "" {
			result = append(result, ev.Key+"="+ev.Value)
			continue
		}
		if ev.HostKey != "" {
			if val, ok := os.LookupEnv(ev.HostKey); ok {
				result = append(result, ev.Key+"="+val)
			}
		}
	}
	return result
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
// 复制 config.yaml、layers/ 目录和 files/ 目录。
// 先清理目标目录中的 layers/ 和 files/ 子目录，再从源目录复制，
// 避免残留的旧文件（如改名后的层）导致构建异常。
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

	// 复制 config.yaml 主配置文件
	srcMain := filepath.Join(src, "config.yaml")
	if data, err := os.ReadFile(srcMain); err == nil {
		os.MkdirAll(dst, 0o755)
		if err := os.WriteFile(filepath.Join(dst, "config.yaml"), data, 0o644); err != nil {
			return err
		}
	}

	// 清理目标 layers/ 目录，防止残留的旧层文件
	dstLayers := filepath.Join(dst, "layers")
	os.RemoveAll(dstLayers)
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

	return nil
}
