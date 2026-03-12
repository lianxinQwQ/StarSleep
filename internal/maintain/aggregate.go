// maintain 包提供动态维护模式的逻辑。
//
// 与 build 不同，maintain 不使用 OverlayFS 分层构建，
// 而是将所有层的包/服务汇总后直接在当前系统上执行。
package maintain

import (
	"starsleep/internal/config"
)

// ChrootStep 保存需要按原始顺序执行的 chroot 层
type ChrootStep struct {
	Helper   string
	Env      []config.EnvVar
	Commands []string
	Packages []string
}

// AggregateResult 按 helper 类型分类汇总的结果
type AggregateResult struct {
	OfficialPkgs []string
	AurPkgs      []string
	Services     []string
	FileMappings []config.FileMapping
	ChrootLayers []ChrootStep
}

// AggregateAll 按 helper 类型分类汇总所有层
func AggregateAll(layers []*config.LayerConfig) *AggregateResult {
	r := &AggregateResult{}
	for _, cfg := range layers {
		switch cfg.Helper {
		case "pacstrap", "pacman":
			r.OfficialPkgs = append(r.OfficialPkgs, cfg.Packages...)
		case "paru":
			r.AurPkgs = append(r.AurPkgs, cfg.Packages...)
		case "enable_service":
			r.Services = append(r.Services, cfg.Services...)
		case "copy_files":
			r.FileMappings = append(r.FileMappings, cfg.Files...)
		case "chroot-cmd":
			r.ChrootLayers = append(r.ChrootLayers, ChrootStep{
				Helper: cfg.Helper, Env: cfg.Env, Commands: cfg.Commands,
			})
		case "chroot-pacman":
			r.ChrootLayers = append(r.ChrootLayers, ChrootStep{
				Helper: cfg.Helper, Env: cfg.Env, Packages: cfg.Packages,
			})
		}
	}
	return r
}
