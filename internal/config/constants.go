// constants.go — StarSleep 全局默认常量
//
// 所有常量均可被 config.yaml 中 meta 段的对应字段覆盖。
// 若配置文件未定义，则回退至此处的值。
package config



const (
	// DefaultWorkDir 默认工作目录，存放 layers、snapshots、work 等子目录
	DefaultWorkDir = "/starsleep"
	// DefaultConfigDir 默认配置目录，存放 config.yaml、layers/、files/
	DefaultConfigDir = "/starsleep/config"
	// DefaultSnapshotDir 默认快照目录
	DefaultSnapshotDir = "/starsleep/snapshots"
	// DefaultPkgCacheDir pacman 包缓存目录（宿主机侧，挂载进 chroot 复用）
	DefaultPkgCacheDir = "/var/cache/pacman/pkg"
)


