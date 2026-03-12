// compare.go — starsleep compare 命令入口
package compare

import (
	"starsleep/internal/build"
	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
)

type compareMode int

const (
	compareModeNone compareMode = iota
	compareModePackages
	compareModeFiles
)

// Run 执行对比命令
func Run(args []string) {
	util.CheckRoot()

	configDir, remaining := config.ParseConfigFlags(config.DefaultConfigDir, args)

	mode := compareModeNone
	verbose := false
	filesTarget := ""
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--packages":
			mode = compareModePackages
		case "--files":
			mode = compareModeFiles
			i++
			if i < len(remaining) {
				filesTarget = remaining[i]
			} else {
				util.Fatal(i18n.T("compare.files.need.dir"))
			}
		case "--verbose", "-v":
			verbose = true
		default:
			util.Fatal(i18n.T("compare.unknown.arg", remaining[i]))
		}
	}

	if mode == compareModeNone {
		util.Fatal(i18n.T("compare.usage"))
	}

	layers, mainCfg, err := config.LoadAllLayers(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}

	switch mode {
	case compareModePackages:
		Packages(layers, configDir, verbose)
	case compareModeFiles:
		Files(filesTarget, config.DefaultWorkDir, func(snapshotDir string) {
			build.ApplyInheritList(mainCfg, snapshotDir)
		})
	}
}
