// build.go — starsleep build 命令
//
// 按照配置文件定义的阶段顺序，使用 OverlayFS 逐层构建环境，
// 通过 reflink 展平合并，最终生成 Btrfs 快照。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func cmdBuild(args []string) {
	checkRoot()

	configDir, remaining := parseConfigFlags(args)

	clean := false
	verify := false
	for _, arg := range remaining {
		switch arg {
		case "--clean":
			clean = true
		case "--verify":
			verify = true
		default:
			fatal(T("build.unknown.arg", arg))
		}
	}

	workDir := defaultWorkDir
	flatDir := filepath.Join(workDir, "work/flat")
	merged := filepath.Join(workDir, "work/merged")
	ovlWork := filepath.Join(workDir, "work/ovl_work")
	logDir := filepath.Join(workDir, "logs")
	ts := timestamp()

	initLog(logDir)
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	verifyTool := filepath.Join(workDir, "starsleep-verify")
	if verify {
		if _, err := os.Stat(verifyTool); err != nil {
			fatal(T("verify.tool.not.found", verifyTool))
		}
	}

	os.MkdirAll(logDir, 0o755)
	os.MkdirAll(filepath.Join(workDir, "work"), 0o755)

	// 清理工作区（仅在 --clean 时）
	if clean {
		logMsg("%s", T("clean.workspace"))
		os.RemoveAll(filepath.Join(workDir, "layers"))
		os.MkdirAll(filepath.Join(workDir, "layers"), 0o755)
		logMsg("%s", T("workspace.cleaned"))
	}

	// 清理残留挂载
	if isMountpoint(merged) {
		logMsg(T("stale.mount"), merged)
		syscall.Sync()
		unmountRecursive(merged)
		time.Sleep(500 * time.Millisecond)
	}

	// 加载配置
	layers, _, err := loadAllLayers(configDir)
	if err != nil {
		fatal(T("load.config.failed", err))
	}

	logMsg("%s", T("build.start"))
	logMsg(T("build.time"), ts)
	logMsg(T("stage.count"), len(layers))

	// 初始化展平子卷
	if isBtrfsSubvolume(flatDir) {
		run("btrfs", "subvolume", "delete", flatDir)
	}
	if err := run("btrfs", "subvolume", "create", flatDir); err != nil {
		fatal(T("create.flat.failed", err))
	}
	logMsg(T("flat.ready"), flatDir)

	// 逐层构建与展平
	// 每层使用 reflink 备份保护：构建前 cp --reflink=always 备份 upper，
	// 直接在 upper 上操作，成功则删除备份；失败则丢弃 upper 恢复备份。
	// 使用 reflink 而非 btrfs 子卷快照，避免 OverlayFS 跨设备 EXDEV 错误。
	var layerDirs []string

	for i, cfg := range layers {
		upper := filepath.Join(workDir, "layers", cfg.Name)
		upperBak := upper + ".bak"
		ovlWorkDir := filepath.Join(ovlWork, fmt.Sprintf("%s.%d", cfg.Name, time.Now().UnixNano()))

		os.MkdirAll(upper, 0o755)
		os.MkdirAll(merged, 0o755)
		os.MkdirAll(ovlWorkDir, 0o755)

		// 清理可能残留的上次失败的备份（说明上次此层失败，恢复）
		if _, err := os.Stat(upperBak); err == nil {
			logMsg(T("layer.backup.detected"), cfg.Name)
			os.RemoveAll(upper)
			os.Rename(upperBak, upper)
		}

		// 创建 reflink 备份（btrfs 上 --reflink=always 零拷贝 CoW）
		if err := run("cp", "-a", "--reflink=always", upper, upperBak); err != nil {
			fatal(T("layer.backup.failed", err))
		}

		logMsg(T("build.layer"), cfg.Name, cfg.Helper)

		// 挂载 OverlayFS（upper 为普通目录，与 flat 子卷共享 st_dev）
		ovlOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,index=off,metacopy=off",
			flatDir, upper, ovlWorkDir)
		if err := syscall.Mount("overlay", merged, "overlay", 0, ovlOpts); err != nil {
			fatal(T("mount.overlay.failed", err))
		}

		// 绑定挂载包缓存
		cacheSrc := filepath.Join(workDir, "shared/pacman-cache")
		cacheDst := filepath.Join(merged, "var/cache/pacman/pkg")
		os.MkdirAll(cacheDst, 0o755)
		syscall.Mount(cacheSrc, cacheDst, "", syscall.MS_BIND, "")

		// 绑定挂载虚拟文件系统（pacstrap 会自行挂载，跳过）
		if cfg.Helper != "pacstrap" {
			bindVFS(merged)
		}

		// 调用同步
		expectedPkgs := buildCumulativePkgs(layers, i)
		expectedSvcs := buildCumulativeServices(layers, i)
		layerOk := runSyncSafe(merged, cfg, expectedPkgs, expectedSvcs)

		// 卸载
		syscall.Sync()
		if err := unmountRecursive(merged); err != nil {
			logMsg("%s", T("unmount.fallback"))
			syscall.Unmount(merged, syscall.MNT_DETACH)
			for retry := 0; isMountpoint(merged) && retry < 10; retry++ {
				time.Sleep(500 * time.Millisecond)
			}
		}

		// 清理 ovl_work
		os.RemoveAll(ovlWorkDir)

		if !layerOk {
			// 同步失败：丢弃修改后的 upper，从备份恢复
			logMsg(T("layer.sync.restore"), cfg.Name)
			os.RemoveAll(upper)
			os.Rename(upperBak, upper)
			fatal(T("layer.build.failed", cfg.Name))
		}

		// 同步成功：删除备份
		os.RemoveAll(upperBak)

		// 展平
		logMsg(T("flatten.layer"), cfg.Name)
		st, err := flattenOverlay(flatDir, upper)
		if err != nil {
			fatal(T("flatten.failed", cfg.Name, err))
		}
		logMsg(T("flatten.stats"),
			st.files, st.dirs, st.symlinks, st.hardlinks, st.whiteouts, st.opaques)

		logMsg(T("layer.done"), cfg.Name)
		layerDirs = append(layerDirs, upper)
	}

	// 一致性校验
	if verify {
		logMsg("%s", T("verify.start"))
		verifyArgs := []string{"--flat", flatDir, "--layers"}
		verifyArgs = append(verifyArgs, layerDirs...)
		if err := run(verifyTool, verifyArgs...); err != nil {
			fatal(T("verify.failed"))
		}
		logMsg("%s", T("verify.passed"))
	}

	// 生成快照
	snapshotName := "snapshot-" + ts
	snapshotDir := filepath.Join(workDir, "snapshots", snapshotName)
	logMsg(T("create.snapshot"), snapshotName)

	if err := run("btrfs", "subvolume", "snapshot", flatDir, snapshotDir); err != nil {
		fatal(T("snapshot.failed", err))
	}

	// 应用继承列表
	applyInheritList(configDir, snapshotDir)

	// 更新 latest 符号链接
	latestLink := filepath.Join(workDir, "snapshots/latest")
	os.Remove(latestLink)
	os.Symlink(snapshotDir, latestLink)

	logMsg("%s", T("build.done"))
	logMsg(T("snapshot.path"), snapshotDir)
	logMsg(T("snapshot.link"), workDir, snapshotName)
}

// bindVFS 绑定挂载虚拟文件系统到目标根
func bindVFS(root string) {
	mounts := []struct{ src, dst string }{
		{"/proc", filepath.Join(root, "proc")},
		{"/sys", filepath.Join(root, "sys")},
		{"/dev", filepath.Join(root, "dev")},
	}
	for _, m := range mounts {
		os.MkdirAll(m.dst, 0o755)
		syscall.Mount(m.src, m.dst, "", syscall.MS_BIND, "")
	}
	// devpts
	devpts := filepath.Join(root, "dev/pts")
	os.MkdirAll(devpts, 0o755)
	syscall.Mount("devpts", devpts, "devpts", 0, "")
	// resolv.conf
	src := "/etc/resolv.conf"
	dst := filepath.Join(root, "etc/resolv.conf")
	if data, err := os.ReadFile(src); err == nil {
		os.MkdirAll(filepath.Dir(dst), 0o755)
		os.WriteFile(dst, data, 0o644)
	}
}

// unmountRecursive 递归卸载（模拟 umount -R）
func unmountRecursive(path string) error {
	// 读取 /proc/mounts 找到所有子挂载点，按路径长度逆序卸载
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return syscall.Unmount(path, 0)
	}

	var mountpoints []string
	for _, line := range splitLines(string(data)) {
		fields := splitFields(line)
		if len(fields) >= 2 {
			mp := fields[1]
			if mp == path || hasPathPrefix(mp, path) {
				mountpoints = append(mountpoints, mp)
			}
		}
	}

	// 逆序卸载（最深的先卸载）
	for i := len(mountpoints) - 1; i >= 0; i-- {
		syscall.Unmount(mountpoints[i], 0)
	}
	return nil
}

// hasPathPrefix 检查 path 是否以 prefix/ 开头
func hasPathPrefix(path, prefix string) bool {
	if len(path) <= len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix && path[len(prefix)] == '/'
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		fields = append(fields, s[start:i])
	}
	return fields
}

// isBtrfsSubvolume 检查路径是否为 btrfs 子卷
func isBtrfsSubvolume(path string) bool {
	return run("btrfs", "subvolume", "show", path) == nil
}

// runSyncSafe 调用 syncLayer 并捕获 fatal panic，返回是否成功。
// 通过启用 fatalPanicMode 将 fatal() 转为 panic，由 recover 捕获。
func runSyncSafe(root string, cfg *LayerConfig, expectedPkgs, expectedSvcs []string) (ok bool) {
	oldMode := fatalPanicMode
	fatalPanicMode = true
	defer func() {
		fatalPanicMode = oldMode
		if r := recover(); r != nil {
			if fe, isFatal := r.(fatalError); isFatal {
				logMsg(T("layer.sync.error"), cfg.Name, fe.msg)
			} else {
				logMsg(T("layer.sync.panic"), cfg.Name, r)
			}
			ok = false
		}
	}()

	syncLayer(root, cfg, expectedPkgs, expectedSvcs)
	return true
}

// applyInheritList 从当前系统复制继承路径到快照
func applyInheritList(configDir, snapshotDir string) {
	paths, err := loadInheritList(configDir)
	if err != nil || len(paths) == 0 {
		if err != nil {
			logMsg("%s", T("inherit.not.found"))
		}
		return
	}

	logMsg(T("apply.inherit"), len(paths))
	for _, entry := range paths {
		fi, err := os.Stat(entry)
		if err != nil {
			logMsg(T("inherit.path.missing"), entry)
			continue
		}

		dst := filepath.Join(snapshotDir, entry)
		os.MkdirAll(filepath.Dir(dst), 0o755)

		if fi.IsDir() {
			if err := run("cp", "-ax", "--reflink=auto", entry, filepath.Dir(dst)+"/"); err != nil {
				logMsg(T("copy.dir.failed"), entry, err)
			} else {
				logMsg(T("inherit.dir"), entry)
			}
		} else {
			if err := run("cp", "-a", "--reflink=auto", entry, dst); err != nil {
				logMsg(T("copy.file.failed"), entry, err)
			} else {
				logMsg(T("inherit.file"), entry)
			}
		}
	}
}
