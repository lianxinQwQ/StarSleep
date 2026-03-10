// cmd_build.go — starsleep build 命令
//
// 按照配置文件定义的阶段顺序，使用 OverlayFS 逐层构建环境，
// 通过 reflink 展平合并，最终生成 Btrfs 快照。
//
// 构建流程:
//  1. 解析命令行参数（--clean / --verify）
//  2. 清理工作区（可选）
//  3. 清理残留挂载点
//  4. 加载配置文件中的所有层定义
//  5. 初始化展平子卷（Btrfs subvolume）
//  6. 逐层构建：挂载 OverlayFS → 同步包/服务 → 卸载 → reflink 展平
//  7. 一致性校验（可选）
//  8. 生成 Btrfs 快照并应用继承列表
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"starsleep/internal/config"
	"starsleep/internal/i18n"
	"starsleep/internal/overlay"
	"starsleep/internal/util"
)

// cmdBuild 执行分层构建命令
//
// @param args 命令行参数列表，支持:
//   - --clean [层名...]: 清理指定层或全部层后重新构建
//   - --verify: 构建完成后执行展平一致性校验
//   - -c/--config <路径>: 指定配置目录
//
// @throws 配置加载失败、展平子卷创建失败、OverlayFS 挂载失败等情况下调用 Fatal 退出
func cmdBuild(args []string) {
	util.CheckRoot()

	// 解析 -c/--config 标志，提取配置目录路径
	configDir, remaining := config.ParseConfigFlags(defaultConfigDir, args)

	// ── 解析 build 专用参数 ──
	clean := false  // 是否执行清理
	verify := false // 是否在构建后执行一致性校验
	var cleanLayers []string
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--clean":
			clean = true
			// --clean 后可跟指定层名，不指定则清理全部
			for i+1 < len(remaining) && !strings.HasPrefix(remaining[i+1], "--") {
				i++
				cleanLayers = append(cleanLayers, remaining[i])
			}
		case "--verify":
			verify = true
		default:
			util.Fatal(i18n.T("build.unknown.arg", remaining[i]))
		}
	}

	// ── 初始化关键路径 ──
	workDir := defaultWorkDir
	flatDir := filepath.Join(workDir, "work/flat")     // 展平子卷路径
	merged := filepath.Join(workDir, "work/merged")    // OverlayFS 合并挂载点
	ovlWork := filepath.Join(workDir, "work/ovl_work") // OverlayFS 工作目录
	logDir := filepath.Join(workDir, "logs")
	ts := util.Timestamp()

	util.InitLog(logDir)
	defer util.CloseLog()

	os.MkdirAll(logDir, 0o755)
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
	layers, _, err := config.LoadAllLayers(configDir)
	if err != nil {
		util.Fatal(i18n.T("load.config.failed", err))
	}

	util.LogMsg("%s", i18n.T("build.start"))
	util.LogMsg(i18n.T("build.time"), ts)
	util.LogMsg(i18n.T("stage.count"), len(layers))

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
		// upper 是当前层的 diff 数据目录，存放与前一层的差异
		upper := filepath.Join(workDir, "layers", cfg.Name)
		upperBak := upper + ".bak" // reflink 备份，用于同步失败时回滚
		// 为每层创建独立的 ovl_work 目录（带纳秒时间戳避免冲突）
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

		// 挂载 OverlayFS:
		// - lowerdir: 当前展平子卷（只读下层）
		// - upperdir: 当前层 diff 目录（读写上层，记录变更）
		// - workdir: OverlayFS 内部工作目录
		// - index=off, metacopy=off: 禁用索引和元数据拷贝以保证兼容性
		ovlOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,index=off,metacopy=off",
			flatDir, upper, ovlWorkDir)
		if err := syscall.Mount("overlay", merged, "overlay", 0, ovlOpts); err != nil {
			util.Fatal(i18n.T("mount.overlay.failed", err))
		}

		// 绑定挂载包缓存
		cacheSrc := filepath.Join(workDir, "shared/pacman-cache")
		cacheDst := filepath.Join(merged, "var/cache/pacman/pkg")
		os.MkdirAll(cacheDst, 0o755)
		syscall.Mount(cacheSrc, cacheDst, "", syscall.MS_BIND, "")

		// 绑定挂载虚拟文件系统
		// pacstrap / arch-chroot 会自行管理 VFS 挂载，跳过
		switch cfg.Helper {
		case "pacstrap", "chroot-cmd", "chroot-pacman":
			// 这些工具内部管理 /proc /sys /dev 挂载
		default:
			bindVFS(merged)
		}

		// 调用同步：在 OverlayFS 合并视图上执行包安装/服务启用/文件叠加
		// 累积列表包含从第 0 层到当前层的所有包/服务，用于声明式清理
		expectedPkgs := config.BuildCumulativePkgs(layers, i)
		expectedSvcs := config.BuildCumulativeServices(layers, i)
		layerOk := runSyncSafe(merged, configDir, cfg, expectedPkgs, expectedSvcs)

		// 卸载
		syscall.Sync()
		if err := unmountRecursive(merged); err != nil {
			util.LogMsg("%s", i18n.T("unmount.fallback"))
			syscall.Unmount(merged, syscall.MNT_DETACH)
			for retry := 0; util.IsMountpoint(merged) && retry < 10; retry++ {
				time.Sleep(500 * time.Millisecond)
			}
		}

		// 清理 ovl_work
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
		st, err := overlay.FlattenOverlay(flatDir, upper)
		if err != nil {
			util.Fatal(i18n.T("flatten.failed", cfg.Name, err))
		}
		util.LogMsg(i18n.T("flatten.stats"),
			st.Files, st.Dirs, st.Symlinks, st.Hardlinks, st.Whiteouts, st.Opaques)

		util.LogMsg(i18n.T("layer.done"), cfg.Name)
		layerDirs = append(layerDirs, upper)
	}

	// 一致性校验
	if verify {
		util.LogMsg("%s", i18n.T("verify.start"))
		if !runVerify(flatDir, layerDirs) {
			util.Fatal(i18n.T("verify.failed"))
		}
		util.LogMsg("%s", i18n.T("verify.passed"))
	}

	// 生成快照
	snapshotName := "snapshot-" + ts
	snapshotDir := filepath.Join(workDir, "snapshots", snapshotName)
	util.LogMsg(i18n.T("create.snapshot"), snapshotName)

	if err := util.Run("btrfs", "subvolume", "snapshot", flatDir, snapshotDir); err != nil {
		util.Fatal(i18n.T("snapshot.failed", err))
	}

	// 应用继承列表
	applyInheritList(configDir, snapshotDir)

	// 更新 latest 符号链接
	latestLink := filepath.Join(workDir, "snapshots/latest")
	os.Remove(latestLink)
	os.Symlink(snapshotDir, latestLink)

	util.LogMsg("%s", i18n.T("build.done"))
	util.LogMsg(i18n.T("snapshot.path"), snapshotDir)
	util.LogMsg(i18n.T("snapshot.link"), workDir, snapshotName)
}

// bindVFS 绑定挂载虚拟文件系统到目标根
//
// 将宿主机的 /proc、/sys、/dev 绑定挂载到目标根目录下，
// 使 chroot 环境中的工具（如 pacman）能正常工作。
// 同时复制 /etc/resolv.conf 以确保 DNS 可用。
//
// @param root 目标根目录路径（通常是 OverlayFS merged 挂载点）
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

// unmountRecursive 递归卸载指定路径下的所有挂载点（模拟 umount -R）
//
// 从 /proc/mounts 读取当前挂载信息，找到所有以 path 为前缀的挂载点，
// 按路径深度倒序逐一卸载，确保子挂载点先于父挂载点卸载。
// 若常规卸载失败，对该挂载点使用延迟卸载（MNT_DETACH）兜底。
//
// @param path 要卸载的根路径
// @return error 若仍有挂载点残留则返回错误
func unmountRecursive(path string) error {
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

	// 从最深的子挂载开始逐一卸载
	var lastErr error
	for i := len(mountpoints) - 1; i >= 0; i-- {
		mp := mountpoints[i]
		if err := syscall.Unmount(mp, 0); err != nil {
			// 常规卸载失败，使用延迟卸载兜底
			util.LogMsg(i18n.T("unmount.lazy.fallback"), mp)
			if err2 := syscall.Unmount(mp, syscall.MNT_DETACH); err2 != nil {
				lastErr = err2
			}
		}
	}
	return lastErr
}

// hasPathPrefix 判断 path 是否以 prefix 为路径前缀（确保以 '/' 分隔）
//
// @param path 待检查的路径
// @param prefix 前缀路径
// @return 如果 path 以 prefix/ 开头则返回 true
func hasPathPrefix(path, prefix string) bool {
	if len(path) <= len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix && path[len(prefix)] == '/'
}

// splitLines 按换行符分割字符串为行切片
//
// @param s 待分割的字符串
// @return 分割后的行切片
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

// splitFields 按空白字符（空格/制表符）分割字符串为字段切片
//
// @param s 待分割的字符串
// @return 分割后的字段切片
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

// isBtrfsSubvolume 检查路径是否为 Btrfs 子卷
//
// 通过调用 btrfs subvolume show 命令来判断。
//
// @param path 要检查的路径
// @return 如果是 Btrfs 子卷返回 true
func isBtrfsSubvolume(path string) bool {
	return util.Run("btrfs", "subvolume", "show", path) == nil
}

// runSyncSafe 安全地调用 syncLayer，捕获 fatal panic 并返回是否成功
//
// 临时启用 FatalPanicMode，使 util.Fatal 抛出 panic 而非直接 os.Exit，
// 从而允许调用方在同步失败时执行回滚逻辑（恢复 upper 层备份）。
//
// @param root 目标根目录路径
// @param configDir 配置目录路径（copy_files 需要定位 files/ 子目录）
// @param cfg 当前层配置
// @param expectedPkgs 到当前层为止的累积包列表
// @param expectedSvcs 到当前层为止的累积服务列表
// @return ok 同步是否成功
func runSyncSafe(root, configDir string, cfg *config.LayerConfig, expectedPkgs, expectedSvcs []string) (ok bool) {
	oldMode := util.FatalPanicMode
	util.FatalPanicMode = true
	defer func() {
		util.FatalPanicMode = oldMode
		if r := recover(); r != nil {
			if fe, isFatal := r.(util.FatalError); isFatal {
				util.LogMsg(i18n.T("layer.sync.error"), cfg.Name, fe.Error())
			} else {
				util.LogMsg(i18n.T("layer.sync.panic"), cfg.Name, r)
			}
			ok = false
		}
	}()

	syncLayer(root, configDir, cfg, expectedPkgs, expectedSvcs)
	return true
}

// applyInheritList 从当前系统复制继承路径到快照
//
// 根据 inherit.list 配置文件中列出的路径，将宿主机上的文件或目录
// 通过 reflink 复制到新生成的快照中，实现配置/数据的跨快照继承。
//
// @param configDir 配置目录路径（包含 inherit.list 文件）
// @param snapshotDir 目标快照目录路径
func applyInheritList(configDir, snapshotDir string) {
	paths, err := config.LoadInheritList(configDir)
	if err != nil || len(paths) == 0 {
		if err != nil {
			util.LogMsg("%s", i18n.T("inherit.not.found"))
		}
		return
	}

	util.LogMsg(i18n.T("apply.inherit"), len(paths))
	for _, entry := range paths {
		fi, err := os.Stat(entry)
		if err != nil {
			util.LogMsg(i18n.T("inherit.path.missing"), entry)
			continue
		}

		dst := filepath.Join(snapshotDir, entry)
		os.MkdirAll(filepath.Dir(dst), 0o755)

		if fi.IsDir() {
			if err := util.Run("cp", "-ax", "--reflink=auto", entry, filepath.Dir(dst)+"/"); err != nil {
				util.LogMsg(i18n.T("copy.dir.failed"), entry, err)
			} else {
				util.LogMsg(i18n.T("inherit.dir"), entry)
			}
		} else {
			if err := util.Run("cp", "-a", "--reflink=auto", entry, dst); err != nil {
				util.LogMsg(i18n.T("copy.file.failed"), entry, err)
			} else {
				util.LogMsg(i18n.T("inherit.file"), entry)
			}
		}
	}
}
