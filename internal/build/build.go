// build 包提供 StarSleep 分层构建的核心逻辑。
//
// 按照配置文件定义的阶段顺序，使用 OverlayFS 逐层构建环境，
// 通过 reflink 展平合并，最终生成 Btrfs 快照。
package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"starsleep/internal/build/helper"
	"starsleep/internal/config"
	"starsleep/internal/flatten"
	"starsleep/internal/i18n"
	"starsleep/internal/util"
	"starsleep/internal/verify"
)

// Run 执行分层构建命令
//
// 解析命令行参数并执行完整的分层构建流程:
//  1. 解析 --clean / --verify 参数
//  2. 清理工作区（可选）
//  3. 加载配置并逐层构建
//  4. 生成 Btrfs 快照并应用继承列表
func Run(args []string) {
	util.CheckRoot()
	configDir, remaining := config.ParseConfigFlags(config.DefaultConfigDir, args)

	clean := false
	doVerify := false
	var cleanLayers []string
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--clean":
			clean = true
			for i+1 < len(remaining) && !strings.HasPrefix(remaining[i+1], "--") {
				i++
				cleanLayers = append(cleanLayers, remaining[i])
			}
		case "--verify":
			doVerify = true
		default:
			util.Fatal(i18n.T("build.unknown.arg", remaining[i]))
		}
	}

	mainCfg, cfgErr := config.LoadMainConfig(configDir)
	if cfgErr != nil {
		util.Fatal(i18n.T("load.config.failed", cfgErr))
	}

	paths := resolveBuildPaths(mainCfg)
	workDir := paths.workDir
	snapshotDir := paths.snapshotDir
	pkgCache := paths.pkgCache
	paruCache := paths.paruCache
	flatDir := filepath.Join(workDir, "work/flat")
	merged := filepath.Join(workDir, "work/merged")
	ovlWork := filepath.Join(workDir, "work/ovl_work")
	logDir := filepath.Join(workDir, "logs")

	os.MkdirAll(logDir, 0o755)
	util.InitLog(logDir)
	defer util.CloseLog()
	ts := util.Timestamp()
	os.MkdirAll(filepath.Join(workDir, "work"), 0o755)

	// 清理工作区（仅在 --clean 时）
	if clean {
		if len(cleanLayers) == 0 {
			util.LogMsg("%s", i18n.T("clean.workspace"))
			os.RemoveAll(filepath.Join(workDir, "layers"))
			os.MkdirAll(filepath.Join(workDir, "layers"), 0o755)
			util.LogMsg("%s", i18n.T("workspace.cleaned"))
		} else {
			for _, name := range cleanLayers {
				layerDir := filepath.Join(workDir, "layers", filepath.Base(name))
				util.LogMsg(i18n.T("clean.layer"), name)
				os.RemoveAll(layerDir)
			}
			util.LogMsg(i18n.T("clean.layers.done"), len(cleanLayers))
		}
	}

	// 清理残留挂载
	if util.IsMountpoint(merged) {
		util.LogMsg(i18n.T("stale.mount"), merged)
		syscall.Sync()
		unmountRecursive(merged)
		time.Sleep(500 * time.Millisecond)
	}

	// 加载配置
	layers, lErr := config.LoadLayers(mainCfg, configDir)
	if lErr != nil {
		util.Fatal(i18n.T("load.config.failed", lErr))
	}

	util.LogMsg("%s", i18n.T("build.start"))
	util.LogMsg(i18n.T("build.time"), ts)
	util.LogMsg(i18n.T("stage.count"), len(layers))

	dbPath := config.ResolveDBPath(mainCfg, config.DefaultDBPath)
	// 初始化展平子卷
	if isBtrfsSubvolume(flatDir) {
		util.Run("btrfs", "subvolume", "delete", flatDir)
	}
	if err := util.Run("btrfs", "subvolume", "create", flatDir); err != nil {
		util.Fatal(i18n.T("create.flat.failed", err))
	}
	util.LogMsg(i18n.T("flat.ready"), flatDir)

	// 逐层构建与展平
	var layerDirs []string
	for i, cfg := range layers {
		upper := filepath.Join(workDir, "layers", cfg.Name)
		upperBak := upper + ".bak"
		ovlWorkDir := filepath.Join(ovlWork, fmt.Sprintf("%s.%d", cfg.Name, time.Now().UnixNano()))
		os.MkdirAll(upper, 0o755)
		os.MkdirAll(merged, 0o755)
		os.MkdirAll(ovlWorkDir, 0o755)

		// 清理可能残留的上次失败的备份
		if _, err := os.Stat(upperBak); err == nil {
			util.LogMsg(i18n.T("layer.backup.detected"), cfg.Name)
			os.RemoveAll(upper)
			os.Rename(upperBak, upper)
		}

		// 创建 reflink 备份
		if err := util.Run("cp", "-a", "--reflink=always", upper, upperBak); err != nil {
			util.Fatal(i18n.T("layer.backup.failed", err))
		}

		util.LogMsg(i18n.T("build.layer"), cfg.Name, cfg.Helper)

		// 挂载 OverlayFS
		ovlOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,index=off,metacopy=off",
			flatDir, upper, ovlWorkDir)
		if err := syscall.Mount("overlay", merged, "overlay", 0, ovlOpts); err != nil {
			util.Fatal(i18n.T("mount.overlay.failed", err))
		}

		// 绑定挂载包缓存
		cacheSrc := pkgCache
		cacheDst := filepath.Join(merged, "var/cache/pacman/pkg")
		os.MkdirAll(cacheDst, 0o755)
		syscall.Mount(cacheSrc, cacheDst, "", syscall.MS_BIND, "")

		// paru 类型额外挂载 paru 缓存
		if cfg.Helper == "chroot-paru" {
			paruCacheSrc := paruCache
			paruCacheDst := filepath.Join(merged, "home/builder/.cache/paru/clone")
			os.MkdirAll(paruCacheDst, 0o755)
			syscall.Mount(paruCacheSrc, paruCacheDst, "", syscall.MS_BIND, "")
		}

		// 绑定挂载虚拟文件系统
		switch cfg.Helper {
		case "pacstrap", "chroot-cmd", "chroot-pacman", "chroot-paru":
			// 这些工具内部管理 /proc /sys /dev 挂载
		default:
			if err := bindVFS(merged); err != nil {
				syscall.Sync()
				cleanupMergedMount(merged)
				os.RemoveAll(ovlWorkDir)
				os.RemoveAll(upper)
				os.Rename(upperBak, upper)
				util.Fatal(err.Error())
			}
		}

		// 调用同步
		expectedPkgs := config.BuildCumulativePkgs(layers, i)
		expectedSvcs := config.BuildCumulativeServices(layers, i)
		layerOk := runSyncSafe(merged, configDir, dbPath, paruCache, cfg, expectedPkgs, expectedSvcs)

		// 卸载
		syscall.Sync()
		cleanupMergedMount(merged)
		os.RemoveAll(ovlWorkDir)

		if !layerOk {
			util.LogMsg(i18n.T("layer.sync.restore"), cfg.Name)
			os.RemoveAll(upper)
			os.Rename(upperBak, upper)
			util.Fatal(i18n.T("layer.build.failed", cfg.Name))
		}

		// 同步成功：删除备份
		os.RemoveAll(upperBak)

		// 展平
		util.LogMsg(i18n.T("flatten.layer"), cfg.Name)
		st, err := flatten.FlattenOverlay(flatDir, upper)
		if err != nil {
			util.Fatal(i18n.T("flatten.failed", cfg.Name, err))
		}
		util.LogMsg(i18n.T("flatten.stats"),
			st.Files, st.Dirs, st.Symlinks, st.Hardlinks, st.Whiteouts, st.Opaques)
		util.LogMsg(i18n.T("layer.done"), cfg.Name)
		layerDirs = append(layerDirs, upper)
	}

	// 一致性校验
	if doVerify {
		util.LogMsg("%s", i18n.T("verify.start"))
		if !verify.RunVerify(flatDir, layerDirs) {
			util.Fatal(i18n.T("verify.failed"))
		}
		util.LogMsg("%s", i18n.T("verify.passed"))
	}

	// 生成快照
	snapshotName := "snapshot-" + ts
	snapshotPath := filepath.Join(snapshotDir, snapshotName)
	util.LogMsg(i18n.T("create.snapshot"), snapshotName)
	if err := util.Run("btrfs", "subvolume", "snapshot", flatDir, snapshotPath); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	latestLink := filepath.Join(snapshotDir, "latest")
	os.Remove(latestLink)
	os.Symlink(snapshotPath, latestLink)

	util.LogMsg("%s", i18n.T("build.done"))
	util.LogMsg(i18n.T("snapshot.path"), snapshotPath)
	util.LogMsg(i18n.T("snapshot.link"), workDir, snapshotName)
}

type buildPaths struct {
	workDir     string
	snapshotDir string
	pkgCache    string
	paruCache   string
}

func resolveBuildPaths(mc *config.MainConfig) buildPaths {
	workDir := config.ResolveWorkDir(mc, config.DefaultWorkDir)
	return buildPaths{
		workDir:     workDir,
		snapshotDir: config.ResolveSnapshotDir(mc, filepath.Join(workDir, "snapshots")),
		pkgCache:    config.ResolvePkgCache(mc, filepath.Join(workDir, "shared/pacman-cache")),
		paruCache:   filepath.Join(workDir, "shared/paru-cache"),
	}
}

func cleanupMergedMount(merged string) {
	if err := unmountRecursive(merged); err != nil {
		util.LogMsg("%s", i18n.T("unmount.fallback"))
		syscall.Unmount(merged, syscall.MNT_DETACH)
		for retry := 0; util.IsMountpoint(merged) && retry < 10; retry++ {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// bindVFS 绑定挂载虚拟文件系统到目标根
func bindVFS(root string) error {
	mounts := []struct{ src, dst string }{
		{"/proc", filepath.Join(root, "proc")},
		{"/sys", filepath.Join(root, "sys")},
		{"/dev", filepath.Join(root, "dev")},
	}
	for _, m := range mounts {
		os.MkdirAll(m.dst, 0o755)
		if err := syscall.Mount(m.src, m.dst, "", syscall.MS_BIND, ""); err != nil {
			return fmt.Errorf("%s", i18n.T("mount.vfs.failed", m.src, m.dst, err))
		}
	}
	devpts := filepath.Join(root, "dev/pts")
	os.MkdirAll(devpts, 0o755)
	if err := syscall.Mount("devpts", devpts, "devpts", 0, ""); err != nil {
		return fmt.Errorf("%s", i18n.T("mount.devpts.failed", devpts, err))
	}

	src := "/etc/resolv.conf"
	dst := filepath.Join(root, "etc/resolv.conf")
	if data, err := os.ReadFile(src); err == nil {
		os.MkdirAll(filepath.Dir(dst), 0o755)
		os.WriteFile(dst, data, 0o644)
	}
	return nil
}

// unmountRecursive 递归卸载指定路径下的所有挂载点
func unmountRecursive(path string) error {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return syscall.Unmount(path, 0)
	}
	var mountpoints []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			mp := fields[1]
			if mp == path || hasPathPrefix(mp, path) {
				mountpoints = append(mountpoints, mp)
			}
		}
	}
	var lastErr error
	for i := len(mountpoints) - 1; i >= 0; i-- {
		mp := mountpoints[i]
		if err := syscall.Unmount(mp, 0); err != nil {
			util.LogMsg(i18n.T("unmount.lazy.fallback"), mp)
			if err2 := syscall.Unmount(mp, syscall.MNT_DETACH); err2 != nil {
				lastErr = err2
			}
		}
	}
	return lastErr
}

func hasPathPrefix(path, prefix string) bool {
	if len(path) <= len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix && path[len(prefix)] == '/'
}

func isBtrfsSubvolume(path string) bool {
	return util.Run("btrfs", "subvolume", "show", path) == nil
}

// runSyncSafe 安全地调用 helper.Dispatch，捕获 fatal panic 并返回是否成功
func runSyncSafe(root, configDir, dbPath, paruCacheDir string, cfg *config.LayerConfig, expectedPkgs, expectedSvcs []string) (ok bool) {
	oldMode := util.FatalPanicMode.Load()
	util.FatalPanicMode.Store(true)
	defer func() {
		util.FatalPanicMode.Store(oldMode)
		if r := recover(); r != nil {
			if fe, isFatal := r.(util.FatalError); isFatal {
				util.LogMsg(i18n.T("layer.sync.error"), cfg.Name, fe.Error())
			} else {
				util.LogMsg(i18n.T("layer.sync.panic"), cfg.Name, r)
			}
			ok = false
		}
	}()
	helper.Dispatch(root, configDir, dbPath, paruCacheDir, cfg, expectedPkgs, expectedSvcs)
	return true
}
