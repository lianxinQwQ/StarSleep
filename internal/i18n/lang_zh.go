package i18n

var zhMessages = map[string]string{
	// ── main.go ──
	"unknown.cmd": "未知命令: %s",
	"usage": `StarSleep - 不可变系统分层构建工具

用法:
  starsleep <命令> [选项]

命令:
  build     分层构建系统快照
  flatten   部署快照到引导
  init      初始化工作环境
  maintain  动态维护模式（直接操作当前系统）
  verify    展平一致性校验

通用选项:
  -c,  --config <路径>   指定配置目录
  -cp, --copy <路径>     复制配置到默认路径后运行
  --lang <语言>          界面语言 (zh/en, 默认: 自动检测)

build 选项:
  --clean [层名...]  清理 layers 后重新构建 (不指定则清理全部)
  --verify          构建后快照前校验展平一致性

verify 选项:
  --flat <展平目录>     展平子卷路径
  --layers <层1> [层2...]  层目录列表

flatten 选项:
  --list              列出已部署的引导条目
  --remove <名称>     移除引导条目
  <快照路径或名称>     要部署的快照 (默认: latest)`,

	// ── util.go ──
	"fatal.prefix": "[StarSleep] 错误: %s\n",
	"need.root":    "需要 root 权限，请使用 sudo 运行",

	// ── build.go ──
	"build.unknown.arg":     "build: 未知参数: %s",
	"verify.tool.not.found": "未找到校验工具: %s",
	"verify.unknown.arg":    "verify: 未知参数: %s",
	"verify.usage":          "用法: starsleep verify --flat <展平目录> --layers <层1> [层2] ...",
	"verify.separator":      "[Verify] ─────────────────────────────────────────────",
	"verify.flat.dir":       "[Verify] 展平目录: %s",
	"verify.layer.count":    "[Verify] 层数: %d",
	"verify.tmpdir.failed":  "[Verify] 创建临时目录失败: %v",
	"verify.mount.failed":   "[Verify] 挂载 OverlayFS 失败: %v",
	"verify.mounted":        "[Verify] OverlayFS 已挂载: %s",
	"verify.lowerdir":       "[Verify] lowerdir 顺序: %s",
	"verify.comparing":      "[Verify] 正向校验: 合并视图 → 展平目录...",
	"verify.rsync.failed":   "[Verify] ✗ rsync 校验命令执行失败: %v",
	"verify.diff.count":     "[Verify] ✗ 发现 %d 处差异:",
	"verify.diff.truncated": "[Verify]   ... 更多差异被截断 (共 %d 处)",
	"verify.result.ok":      "[Verify] ✓ 一致性校验全部通过",
	"verify.result.fail":    "[Verify] ✗ 一致性校验失败",
	"clean.workspace":       "--clean: 清理工作区，从头构建",
	"workspace.cleaned":     "工作区已清理",
	"clean.layer":           "--clean: 清理层 %s",
	"clean.layers.done":     "已清理 %d 个层",
	"stale.mount":           "检测到 %s 残留挂载，正在清理...",
	"load.config.failed":    "加载配置失败: %v",
	"build.start":           "=== StarSleep 分层构建开始 ===",
	"build.time":            "构建时间: %s",
	"stage.count":           "阶段数量: %d",
	"create.flat.failed":    "创建展平子卷失败: %v",
	"flat.ready":            "展平子卷已就绪: %s",
	"layer.backup.detected": "检测到层 %s 的备份（上次构建可能中断），正在恢复...",
	"layer.backup.failed":   "创建层备份失败: %v",
	"build.layer":           ">>> 构建层: %s (%s)",
	"mount.overlay.failed":  "挂载 OverlayFS 失败: %v",
	"unmount.fallback":      "警告: 常规卸载失败，使用延迟卸载...",
	"layer.sync.restore":    "层 %s 同步失败，从备份恢复上次成功状态",
	"layer.build.failed":    "层 %s 构建失败",
	"flatten.layer":         "[Flatten] 展平层 %s ...",
	"flatten.failed":        "展平层 %s 失败: %v",
	"flatten.stats":         "[Flatten] 统计: %d 文件, %d 目录, %d 符号链接, %d 硬链接, %d whiteout, %d 不透明目录",
	"layer.done":            "<<< 层 %s 完成",
	"verify.start":          ">>> 一致性校验: 展平子卷 vs OverlayFS 合并视图",
	"verify.failed":         "一致性校验失败",
	"verify.passed":         "<<< 一致性校验通过",
	"create.snapshot":       ">>> 生成快照: %s",
	"snapshot.failed":       "创建快照失败: %v",
	"build.done":            "=== 构建完成 ===",
	"snapshot.path":         "快照: %s",
	"snapshot.link":         "链接: %s/snapshots/latest -> %s",
	"inherit.not.found":     "提示: 未找到继承列表，跳过",
	"apply.inherit":         "应用继承列表: %d 条路径",
	"inherit.path.missing":  "警告: 继承路径不存在，跳过: %s",
	"copy.dir.failed":       "警告: 复制目录失败: %s: %v",
	"inherit.dir":           "继承目录: %s",
	"copy.file.failed":      "警告: 复制文件失败: %s: %v",
	"inherit.file":          "继承文件: %s",
	"layer.sync.error":      "层 %s 同步失败: %s",
	"layer.sync.panic":      "层 %s 同步异常: %v",

	// ── deploy.go ──
	"flatten.remove.usage":   "用法: starsleep flatten --remove <快照名称>",
	"snapshot.not.exist":     "快照不存在: %s",
	"boot.not.found":         "快照中未找到 boot 目录: %s",
	"kernel.not.found":       "未找到内核文件 (vmlinuz-*): %s",
	"initramfs.not.found":    "未找到 initramfs (initramfs-*.img): %s",
	"deploy.separator":       "[Deploy] ─────────────────────────────────────────────",
	"deploy.snapshot":        "[Deploy] 快照: %s",
	"deploy.kernel":          "[Deploy] 内核: %s",
	"deploy.initramfs":       "[Deploy] Initramfs: %s",
	"deploy.boot.copied":     "[Deploy] 已复制启动文件到: %s/",
	"deploy.entry.generated": "[Deploy] 已生成引导条目: %s",
	"write.entry.failed":     "写入引导条目失败: %v",
	"deploy.done":            "[Deploy] ✓ 部署完成",
	"deploy.reboot.hint":     "[Deploy] 重启后在 systemd-boot 菜单中选择:",
	"deploy.reboot.entry":    "[Deploy]   %s - %s",
	"deploy.list.header":     "[Deploy] 已部署的 StarSleep 引导条目:",
	"deploy.list.empty":      "[Deploy]   (无)",
	"deploy.removed.entry":   "[Deploy] 已移除引导条目: %s",
	"deploy.entry.not.exist": "[Deploy] 引导条目不存在: %s",
	"deploy.removed.boot":    "[Deploy] 已移除启动文件: %s",
	"read.file.failed":       "读取 %s 失败: %v",
	"write.file.failed":      "写入 %s 失败: %v",

	// ── init_cmd.go ──
	"init.unknown.arg":      "init: 未知参数: %s",
	"missing.deps":          "错误: 缺少以下依赖，请先安装:",
	"init.start":            "=== StarSleep 环境初始化 ===",
	"init.workdir":          "工作目录: %s",
	"init.copy.config":      "复制配置: %s → %s",
	"init.copy.config.warn": "警告: 复制配置失败: %v",
	"init.deployed":         "已部署: %s",
	"init.done":             "=== 环境初始化完成 ===",
	"init.tree.header":      "目录结构:",
	"init.tree": `  %s/
  ├── layers/          # 各层 diff 数据
  ├── snapshots/       # 生产快照
  ├── shared/          # 持久化共享数据
  │   ├── home/
  │   └── pacman-cache/
  ├── config/          # 配置文件
  │   ├── layers/      # 层定义 (YAML)
  │   └── inherit.list # 继承列表
  ├── work/            # 构建工作区
  │   ├── flat/        # 展平子卷 (Btrfs)
  │   ├── merged/      # OverlayFS 合并挂载点
  │   └── ovl_work/    # OverlayFS 工作目录
  └── logs/            # 构建日志`,
	"init.next": "下一步: sudo starsleep build",

	// ── maintain.go ──
	"maintain.unknown.arg":    "maintain: 未知参数: %s",
	"maintain.separator":      "[Maintain] ═══════════════════════════════════════════════",
	"maintain.title":          "[Maintain] StarSleep 动态维护模式",
	"maintain.config.dir":     "[Maintain] 配置目录: %s",
	"maintain.layer.count":    "[Maintain] 层数: %d",
	"maintain.official.pkgs":  "[Maintain] 官方仓库包: %d 个",
	"maintain.aur.pkgs":       "[Maintain] AUR 包: %d 个",
	"maintain.services.count": "[Maintain] 服务: %d 个",
	"maintain.step1":          "[Maintain] 步骤 1/5: 清理多余软件包...",
	"maintain.step2":          "[Maintain] 步骤 2/5: 同步官方仓库软件包...",
	"maintain.step2.skip":     "[Maintain] 步骤 2/5: 无官方仓库包需要安装",
	"maintain.step3":          "[Maintain] 步骤 3/5: 同步 AUR 软件包...",
	"maintain.step3.skip":     "[Maintain] 步骤 3/5: 无 AUR 包需要安装",
	"maintain.paru.warn":      "[Maintain] 警告: paru 安装失败: %v",
	"maintain.step4":          "[Maintain] 步骤 4/5: 启用 systemd 服务...",
	"maintain.step4.skip":     "[Maintain] 步骤 4/5: 无服务需要启用",
	"maintain.step5":          "[Maintain] 步骤 5/5: 创建快照并部署引导...",
	"maintain.snapshot.name":  "[Maintain] 快照名称: %s",
	"maintain.detect.failed":  "[Maintain] 无法检测当前快照: %v",
	"maintain.done":           "[Maintain] ✓ 动态维护完成",
	"maintain.query.failed":   "[Maintain] 警告: 查询显式包列表失败: %v",
	"maintain.demote":         "[Maintain] 降级 %d 个包为依赖: %s",
	"maintain.orphans":        "[Maintain] 清理孤立依赖: %s",
	"maintain.orphans.failed": "[Maintain] 警告: 清理孤立依赖失败: %v",

	// ── sync.go ──
	"sync.unknown.tool": "未知的工具: %s\n支持的工具: pacstrap, pacman, paru, enable_service",
	"sync.stage.done":   "[Sync] ✓ 阶段 %s 同步完成",
	"sync.separator":    "[Sync] ─────────────────────────────────────────────",
	"sync.stage":        "[Sync] 阶段: %s",
	"sync.tool":         "[Sync] 工具: %s",
	"sync.target":       "[Sync] 目标: %s",
	"sync.packages":     "[Sync] 软件包: %d 个",
	"sync.services":     "[Sync] 服务: %d 个",

	// ── enable_service.go ──
	"sync.disable.extra":      "[Sync] 禁用多余服务: %s",
	"sync.no.services":        "[Sync] 没有需要启用的服务",
	"sync.enable.start":       "[Sync] 使用 systemctl 启用 systemd 服务...",
	"sync.enable.service":     "[Sync] 启用服务: %s",
	"sync.enable.failed":      "启用服务 %s 失败: %v",
	"sync.enabled.count":      "[Sync] 已启用 %d 个服务",
	"maintain.disable.extra":  "[Maintain] 禁用多余服务: %s",
	"maintain.enable.service": "[Maintain] 启用服务: %s",
	"maintain.enable.failed":  "[Maintain] 警告: 启用服务 %s 失败: %v",

	// ── pacman.go ──
	"sync.query.failed":   "[Sync] 警告: 查询显式包列表失败: %v，跳过降级步骤",
	"sync.demote":         "[Sync] 降级为依赖: %s",
	"sync.orphans":        "[Sync] 清理孤立依赖: %s",
	"sync.orphans.failed": "[Sync] 警告: 清理孤立依赖失败: %v",
	"sync.install.pkgs":   "[Sync] 安装/更新软件包...",
	"pacman.failed":       "pacman 安装失败: %v",
	"sync.pacman":         "[Sync] 使用 pacman 同步官方仓库软件包...",

	// ── pacstrap.go ──
	"sync.pacstrap":    "[Sync] 使用 pacstrap 初始化基础根文件系统...",
	"sync.incremental": "[Sync] 检测到已有系统，执行增量同步...",
	"sync.fresh":       "[Sync] 全新引导...",
	"pacstrap.failed":  "pacstrap 失败: %v",

	// ── paru.go ──
	"sync.paru":   "[Sync] 使用 paru 安装 AUR 软件包...",
	"paru.failed": "paru 安装失败: %v",

	// ── config.go ──
	"cfg.read":          "读取 %s: %w",
	"cfg.parse":         "解析 %s: %w",
	"cfg.scan":          "扫描层配置: %w",
	"cfg.no.files":      "%s 中没有找到配置文件",
	"cfg.copied":        "已复制配置: %s → %s",
	"cfg.copy.failed":   "复制配置失败: %v",
	"cfg.src.not.exist": "源配置不存在: %s",
	"cfg.src.not.dir":   "源配置不是目录: %s",
	"cfg.read.layers":   "读取源 layers: %w",

	// ── alpm.go ──
	"alpm.init":    "初始化 alpm: %w",
	"alpm.localdb": "获取本地数据库: %w",

	// ── overlay.go ──
	"ovl.readdir":    "读取目录 %s: %w",
	"ovl.stat":       "stat %s: %w",
	"ovl.no.stat":    "无法获取 syscall.Stat_t: %s",
	"ovl.mkdir":      "创建目录 %s: %w",
	"ovl.open.src":   "打开源文件 %s: %w",
	"ovl.create.dst": "创建目标文件 %s: %w",
	"ovl.copy":       "复制 %s -> %s: %w",
	"ovl.readlink":   "readlink %s: %w",
	"ovl.mknod":      "mknod %s: %w",
}
