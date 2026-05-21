// lang_en.go — 英文翻译映射表
package i18n

// enMessages 包含所有英文翻译条目，键为 i18n.T() 的 key，值为对应的英文模板
var enMessages = map[string]string{
	// ── main.go ──
	"unknown.cmd": "Unknown command: %s",
	"usage": `StarSleep - Immutable System Layered Build Tool

Usage:
  starsleep <command> [options]

Commands:
  build     Build system snapshot in layers
  compare   Compare current system against config
  config    Manage config directory (collect inherit files, import/export archive)
  flatten   Deploy snapshot to boot
  init      Initialize work environment
  install   Install system to target partitions
  maintain  Dynamic maintenance mode (operate on running system)
  verify    Verify flatten consistency

General options:
  -c,  --config <path>   Specify config directory
  -cp, --copy <path>     Copy config to default path before running
  --lang <lang>          UI language (zh/en, default: auto-detect)

build options:
  --clean [layers...]    Clean layers and rebuild (all if none specified)
  --verify               Verify flatten consistency before snapshot

config options:
  --collect              Collect inherit-listed files from host into config/inherit/
  --export <file>        Pack entire config directory into a .tar.gz archive
  --import <file>        Restore config directory from a .tar.gz archive

verify options:
  --flat <flat-dir>         Flat subvolume path
  --layers <l1> [l2...]     Layer directory list

compare options:
  --packages             Compare explicitly installed package lists (no build)
  --files <target-dir>   Compare files between latest snapshot and target directory
  -v, --verbose          Verbose output (show group status, no-diff hints, etc.)

maintain options:
  --disauto   prompt for confirmation before demoting/installing

flatten options:
  --list              List deployed boot entries
  --remove <name>     Remove boot entry
  --use-inherit-store Use files from config/inherit/ instead of live host when applying inherit
  <snapshot-path>     Snapshot to deploy (default: latest)

install options:
  --boot <part>     EFI system partition, asked interactively if omitted
  --root <part>     Root partition, asked interactively if omitted
  --disk <dev>      Whole disk device, can interactively select disk & partitions
  --profile <name>  Preset configuration (minimal/gnome/dev), asked interactively if omitted
  --name <name>     Display name in UEFI firmware boot menu (default: StarSleep)
  --branch <branch> Git branch where preset configs reside (default: main)
  --force           Skip confirmation prompts, force format
  --repo <URL>      Git repository URL for preset configs (optional, e.g. https://github.com/user/repo)`,

	// ── util.go ──
	"fatal.prefix": "[StarSleep] Error: %s\n",
	"need.root":    "Root privileges required, please run with sudo",

	// ── build.go ──
	"build.unknown.arg":     "build: unknown argument: %s",
	"verify.tool.not.found": "Verify tool not found: %s",
	"verify.unknown.arg":    "verify: unknown argument: %s",
	"verify.usage":          "Usage: starsleep verify --flat <flat-dir> --layers <layer1> [layer2] ...",
	"verify.separator":      "[Verify] ─────────────────────────────────────────────",
	"verify.flat.dir":       "[Verify] Flat dir: %s",
	"verify.layer.count":    "[Verify] Layers: %d",
	"verify.tmpdir.failed":  "[Verify] Failed to create temp dir: %v",
	"verify.mount.failed":   "[Verify] Failed to mount OverlayFS: %v",
	"verify.mounted":        "[Verify] OverlayFS mounted: %s",
	"verify.lowerdir":       "[Verify] lowerdir order: %s",
	"verify.comparing":      "[Verify] Comparing: merged view → flat dir...",
	"verify.rsync.failed":   "[Verify] ✗ rsync check command failed: %v",
	"verify.diff.count":     "[Verify] ✗ Found %d differences:",
	"verify.diff.truncated": "[Verify]   ... more diffs truncated (total %d)",
	"verify.result.ok":      "[Verify] ✓ Consistency check passed",
	"verify.result.fail":    "[Verify] ✗ Consistency check failed",
	"clean.workspace":       "--clean: Cleaning workspace, rebuild from scratch",
	"workspace.cleaned":     "Workspace cleaned",
	"clean.layer":           "--clean: Cleaning layer %s",
	"clean.layers.done":     "Cleaned %d layers",
	"stale.mount":           "Detected stale mount at %s, cleaning up...",
	"load.config.failed":    "Failed to load config: %v",
	"build.start":           "=== StarSleep Layered Build Started ===",
	"build.time":            "Build time: %s",
	"stage.count":           "Stage count: %d",
	"create.flat.failed":    "Failed to create flat subvolume: %v",
	"flat.ready":            "Flat subvolume ready: %s",
	"layer.backup.detected": "Backup detected for layer %s (previous build may have been interrupted), restoring...",
	"layer.backup.failed":   "Failed to create layer backup: %v",
	"build.layer":           ">>> Building layer: %s (%s)",
	"mount.overlay.failed":  "Failed to mount OverlayFS: %v",
	"unmount.fallback":      "Warning: normal unmount failed, using lazy unmount...",
	"unmount.lazy.fallback": "Warning: %s normal unmount failed, trying lazy unmount...",
	"layer.sync.restore":    "Layer %s sync failed, restoring from backup",
	"layer.build.failed":    "Layer %s build failed",
	"flatten.layer":         "[Flatten] Flattening layer %s ...",
	"flatten.failed":        "Failed to flatten layer %s: %v",
	"flatten.stats":         "[Flatten] Stats: %d files, %d dirs, %d symlinks, %d hardlinks, %d whiteouts, %d opaques",
	"layer.done":            "<<< Layer %s done",
	"verify.start":          ">>> Consistency check: flat subvolume vs OverlayFS merged view",
	"verify.failed":         "Consistency check failed",
	"verify.passed":         "<<< Consistency check passed",
	"create.snapshot":       ">>> Creating snapshot: %s",
	"snapshot.failed":       "Failed to create snapshot: %v",
	"build.done":            "=== Build Complete ===",
	"snapshot.path":         "Snapshot: %s",
	"snapshot.link":         "Link: %s/snapshots/latest -> %s",
	"inherit.not.found":     "Note: inherit list not found, skipping",
	"apply.inherit":         "Applying inherit list: %d paths",
	"inherit.path.missing":  "Warning: inherited path does not exist, skipping: %s",
	"copy.dir.failed":       "Warning: failed to copy directory: %s: %v",
	"inherit.dir":           "Inherited directory: %s",
	"copy.file.failed":      "Warning: failed to copy file: %s: %v",
	"inherit.file":          "Inherited file: %s",
	"layer.sync.error":      "Layer %s sync failed: %s",
	"layer.sync.panic":      "Layer %s sync panic: %v",
	"mount.vfs.failed":      "Failed to bind-mount virtual filesystem %s → %s: %v",
	"mount.devpts.failed":   "Failed to mount devpts at %s: %v",

	// ── deploy.go ──
	"flatten.remove.usage":   "Usage: starsleep flatten --remove <snapshot-name>",
	"snapshot.not.exist":     "Snapshot does not exist: %s",
	"boot.not.found":         "boot directory not found in snapshot: %s",
	"kernel.not.found":       "Kernel file not found (vmlinuz-*): %s",
	"initramfs.not.found":    "Initramfs not found (initramfs-*.img): %s",
	"deploy.separator":       "[Deploy] ─────────────────────────────────────────────",
	"deploy.snapshot":        "[Deploy] Snapshot: %s",
	"deploy.kernel":          "[Deploy] Kernel: %s",
	"deploy.initramfs":       "[Deploy] Initramfs: %s",
	"deploy.boot.copied":     "[Deploy] Boot files copied to: %s/",
	"deploy.entry.generated": "[Deploy] Boot entry generated: %s",
	"write.entry.failed":     "Failed to write boot entry: %v",
	"deploy.done":            "[Deploy] ✓ Deployment complete",
	"deploy.reboot.hint":     "[Deploy] After reboot, select from systemd-boot menu:",
	"deploy.reboot.entry":    "[Deploy]   %s - %s",
	"deploy.list.header":     "[Deploy] Deployed StarSleep boot entries:",
	"deploy.list.empty":      "[Deploy]   (none)",
	"deploy.removed.entry":   "[Deploy] Removed boot entry: %s",
	"deploy.entry.not.exist": "[Deploy] Boot entry does not exist: %s",
	"deploy.removed.boot":    "[Deploy] Removed boot files: %s",
	"root.uuid.not.found":    "Cannot resolve root partition UUID, please ensure findmnt/blkid are available",
	"create.subvol.failed":   "Failed to create Btrfs subvolume: %s: %v",
	"read.file.failed":       "Failed to read %s: %v",
	"write.file.failed":      "Failed to write %s: %v",

	// ── init_cmd.go ──
	"init.unknown.arg":      "init: unknown argument: %s",
	"missing.deps":          "Error: missing required dependencies, please install:",
	"init.start":            "=== StarSleep Environment Initialization ===",
	"init.workdir":          "Work directory: %s",
	"init.copy.config":      "Copy config: %s → %s",
	"init.copy.config.warn": "Warning: failed to copy config: %v",
	"init.deployed":         "Deployed: %s",
	"init.done":             "=== Environment Initialization Complete ===",
	"init.tree.header":      "Directory structure:",
	"init.tree": `  %s/
  ├── layers/          # Per-layer diff data
  ├── snapshots/       # Production snapshots
  ├── shared/          # Persistent shared data
  │   ├── home/            # User homes (btrfs subvol)
  │   ├── root/            # root home (btrfs subvol)
  │   ├── pacman-cache/    # pacman package cache (btrfs subvol)
  │   └── paru-cache/      # paru package cache (btrfs subvol)
  ├── config/          # Configuration files
  │   ├── layers/      # Layer definitions (YAML)
  │   ├── files/       # Overlay files source dir
  │   └── inherit/     # Collected inherit file snapshots (--collect)
  ├── work/            # Build workspace
  │   ├── flat/        # Flat subvolume (btrfs, managed by build)
  │   ├── merged/      # OverlayFS merged mount
  │   └── ovl_work/    # OverlayFS work dir
  └── logs/            # Build logs`,
	"init.next": "Next step: sudo starsleep build",

	// ── maintain.go ──
	"maintain.unknown.arg":     "maintain: unknown argument: %s",
	"maintain.separator":       "[Maintain] ═══════════════════════════════════════════════",
	"maintain.title":           "[Maintain] StarSleep Dynamic Maintenance Mode",
	"maintain.config.dir":      "[Maintain] Config dir: %s",
	"maintain.layer.count":     "[Maintain] Layers: %d",
	"maintain.total.pkgs":      "[Maintain] Expected total packages: %d",
	"maintain.services.count":  "[Maintain] Services: %d",
	"maintain.pre.snapshot":    "[Maintain] Prep: creating pre-maintain backup snapshot: %s",
	"maintain.step1":           "[Maintain] Step 1/8: Demoting extra packages...",
	"maintain.step2":           "[Maintain] Step 2/8: Installing missing packages...",
	"maintain.step2.skip":      "[Maintain] Step 2/8: No missing packages, skipping",
	"maintain.step3":           "[Maintain] Step 3/8: Cleaning orphan deps...",
	"maintain.step.syu":        "[Maintain] Step 4/8: Full update (paru -Syu)...",
	"maintain.paru.warn":       "[Maintain] Warning: paru installation failed: %v",
	"maintain.step4":           "[Maintain] Step 5/8: Enabling systemd services...",
	"maintain.step4.skip":      "[Maintain] Step 5/8: No services to enable",
	"maintain.step5":           "[Maintain] Step 8/8: Creating snapshot and deploying boot entry...",
	"maintain.snapshot.name":   "[Maintain] Snapshot name: %s",
	"maintain.detect.failed":   "[Maintain] Failed to detect current snapshot: %v",
	"maintain.done":            "[Maintain] ✓ Dynamic maintenance complete",
	"maintain.query.failed":    "[Maintain] Warning: failed to query explicit packages: %v",
	"maintain.demote":          "[Maintain] Demoting %d packages to deps: %s",
	"maintain.orphans":         "[Maintain] Cleaning orphan deps: %s",
	"maintain.orphans.failed":  "[Maintain] Warning: failed to clean orphan deps: %v",
	"maintain.confirm.demote":  "[Maintain] The above packages will be demoted to deps, continue?",
	"maintain.confirm.orphans": "[Maintain] The above orphan deps will be removed, continue?",
	"maintain.confirm.install": "[Maintain] Packages to install: %s",
	"maintain.confirm.prompt":  "Continue? [y/N] ",
	"maintain.aborted":         "operation aborted by user",

	// ── sync.go ──
	"sync.unknown.tool": "Unknown tool: %s\nSupported tools: pacstrap, pacman, paru, enable_service, copy_files, chroot-cmd, chroot-pacman, chroot-paru",
	"sync.stage.done":   "[Sync] ✓ Stage %s sync complete",
	"sync.separator":    "[Sync] ─────────────────────────────────────────────",
	"sync.stage":        "[Sync] Stage: %s",
	"sync.tool":         "[Sync] Tool: %s",
	"sync.target":       "[Sync] Target: %s",
	"sync.packages":     "[Sync] Packages: %d",
	"sync.services":     "[Sync] Services: %d",
	"sync.files":        "[Sync] File mappings: %d",

	// ── copyfiles ──
	"copyfiles.path.escape":      "Path escape: %s",
	"copyfiles.base.not.exist":   "Overlay files source directory does not exist: %s",
	"copyfiles.start":            "[CopyFiles] Starting file overlay...",
	"copyfiles.done":             "[CopyFiles] ✓ Done, %d mappings applied",
	"copyfiles.src.invalid":      "[CopyFiles] Invalid source path: %s: %v",
	"copyfiles.src.not.exist":    "[CopyFiles] Source path does not exist: %s (%s)",
	"copyfiles.copy.item":        "[CopyFiles] %s → %s",
	"copyfiles.copy.dir.failed":  "Failed to copy directory: %s: %v",
	"copyfiles.copy.file.failed": "Failed to copy file: %s: %v",

	// ── maintain copy_files ──
	"maintain.step.copyfiles":      "[Maintain] Step 6/8: Overlaying config files...",
	"maintain.step.copyfiles.skip": "[Maintain] Step 6/8: No files to overlay",

	// ── chroot ──
	"chroot.cmd.start":               "[Chroot] Starting chroot command execution...",
	"chroot.cmd.exec":                "[Chroot] Exec: %s",
	"chroot.cmd.failed":              "chroot command failed: %s: %v",
	"chroot.cmd.done":                "[Chroot] ✓ Done, %d commands executed",
	"chroot.pacman.start":            "[Chroot] Running pacman via arch-chroot...",
	"chroot.pacman.failed":           "chroot pacman installation failed: %v",
	"chroot.pacman.done":             "[Chroot] ✓ Done, %d packages installed",
	"chroot.paru.start":              "[Chroot] Running paru via arch-chroot...",
	"chroot.paru.failed":             "chroot paru installation failed: %v",
	"chroot.paru.done":               "[Chroot] ✓ paru done, %d packages installed",
	"chroot.paru.create.user":        "[Chroot] Creating builder user...",
	"chroot.paru.create.user.failed": "[Chroot] Failed to create builder user: %v",
	"chroot.env.set":                 "[Chroot] Env: %s=%s",
	"chroot.env.host":                "[Chroot] Env: %s ← host $%s",
	"sync.commands":                  "[Sync] Commands: %d",

	// ── maintain chroot ──
	"maintain.step.chroot":      "[Maintain] Step 7/8: Executing chroot commands...",
	"maintain.step.chroot.skip": "[Maintain] Step 7/8: No chroot commands to execute",

	// ── enable_service.go ──
	"sync.disable.extra":      "[Sync] Disabling extra service: %s",
	"sync.no.services":        "[Sync] No services to enable",
	"sync.enable.start":       "[Sync] Enabling systemd services with systemctl...",
	"sync.enable.service":     "[Sync] Enabling service: %s",
	"sync.enable.failed":      "Failed to enable service %s: %v",
	"sync.enabled.count":      "[Sync] Enabled %d services",
	"maintain.disable.extra":  "[Maintain] Disabling extra service: %s",
	"maintain.enable.service": "[Maintain] Enabling service: %s",
	"maintain.enable.failed":  "[Maintain] Warning: failed to enable service %s: %v",

	// ── pacman.go ──
	"sync.query.failed":   "[Sync] Warning: failed to query explicit packages: %v, skipping demote",
	"sync.demote":         "[Sync] Demoting to dep: %s",
	"sync.orphans":        "[Sync] Cleaning orphan deps: %s",
	"sync.orphans.failed": "[Sync] Warning: failed to clean orphan deps: %v",
	"sync.install.pkgs":   "[Sync] Installing/updating packages...",
	"pacman.failed":       "pacman installation failed: %v",
	"sync.pacman":         "[Sync] Syncing official repo packages with pacman...",

	// ── pacstrap.go ──
	"sync.pacstrap":         "[Sync] Initializing base root filesystem with pacstrap...",
	"sync.incremental":      "[Sync] Existing system detected, performing incremental sync...",
	"sync.fresh":            "[Sync] Fresh bootstrap...",
	"sync.refresh.stale.db": "[Sync] Stale sync databases found in target, refreshing to avoid outdated package versions...",
	"pacstrap.failed":       "pacstrap failed: %v",

	// ── paru.go ──
	"sync.paru":   "[Sync] Installing AUR packages with paru...",
	"paru.failed": "paru installation failed: %v",

	// ── config.go ──
	"cfg.read":               "reading %s: %w",
	"cfg.parse":              "parsing %s: %w",
	"cfg.scan":               "scanning layer configs: %w",
	"cfg.no.files":           "no config files found in %s",
	"cfg.no.layers":          "no layers defined in config.yaml",
	"cfg.layer.not.exist":    "layer file referenced in config.yaml does not exist: %s",
	"cfg.layer.unreferenced": "unreferenced files found in layers/ directory: %s",
	"cfg.copied":             "Config copied: %s → %s",
	"cfg.copy.failed":        "Failed to copy config: %v",
	"cfg.src.not.exist":      "source config does not exist: %s",
	"cfg.src.not.dir":        "source config is not a directory: %s",
	"cfg.read.layers":        "reading source layers: %w",

	// ── alpm.go ──
	"alpm.init":    "initializing alpm: %w",
	"alpm.localdb": "getting local database: %w",

	// ── overlay.go ──
	"ovl.readdir":    "reading directory %s: %w",
	"ovl.stat":       "stat %s: %w",
	"ovl.no.stat":    "cannot get syscall.Stat_t: %s",
	"ovl.mkdir":      "creating directory %s: %w",
	"ovl.open.src":   "opening source file %s: %w",
	"ovl.create.dst": "creating target file %s: %w",
	"ovl.copy":       "copying %s -> %s: %w",
	"ovl.readlink":   "readlink %s: %w",
	"ovl.mknod":      "mknod %s: %w",

	// ── compare.go ──
	"compare.unknown.arg":           "compare: unknown argument: %s",
	"compare.usage":                 "Usage: starsleep compare --packages [-v] [-c <config-dir>]\n       starsleep compare --files <target-dir> [-v] [-c <config-dir>]",
	"compare.files.need.dir":        "--files requires a target directory",
	"compare.separator":             "[Compare] ═══════════════════════════════════════════════",
	"compare.config.dir":            "[Compare] Config dir: %s",
	"compare.pkg.title":             "[Compare] Package List Comparison Mode",
	"compare.pkg.expected":          "[Compare] Expected packages from config (groups expanded): %d",
	"compare.pkg.installed":         "[Compare] Explicitly installed packages: %d",
	"compare.pkg.all.installed":     "[Compare] Total installed packages: %d",
	"compare.query.failed":          "[Compare] Failed to query package list: %v",
	"compare.pkg.groups.header":     "[Compare] Package group status (%d groups):",
	"compare.pkg.groups.item":       "[Compare]   ■ %s",
	"compare.pkg.missing.header":    "[Compare] Missing packages (in config but not installed): %d",
	"compare.pkg.missing.item":      "[Compare]   - %s",
	"compare.pkg.no.missing":        "[Compare] ✓ No missing packages",
	"compare.pkg.asdeps.header":     "[Compare] Dep-installed packages (present but not explicit): %d",
	"compare.pkg.asdeps.item":       "[Compare]   ~ %s",
	"compare.pkg.no.asdeps":         "[Compare] ✓ No dep-installed packages",
	"compare.pkg.extra.header":      "[Compare] Extra packages (explicitly installed but not in config): %d",
	"compare.pkg.extra.item":        "[Compare]   + %s",
	"compare.pkg.no.extra":          "[Compare] ✓ No extra packages",
	"compare.pkg.match":             "[Compare] ✓ Package lists match perfectly",
	"compare.pkg.diff":              "[Compare] ✗ %d differences found (missing: %d, extra: %d)",
	"compare.pkg.diff.verbose":      "[Compare] ✗ %d differences found (missing: %d, dep-installed: %d, extra: %d)",
	"compare.file.title":            "[Compare] File Comparison Mode (based on latest snapshot)",
	"compare.file.target":           "[Compare] Comparison target: %s",
	"compare.file.snapshot":         "[Compare] Base snapshot: %s",
	"compare.file.creating.snap":    "[Compare] Creating temporary snapshot: %s",
	"compare.file.applying.inherit": "[Compare] Applying inherit list...",
	"compare.file.comparing":        "[Compare] Comparing with rsync: %s ↔ %s",
	"compare.file.match":            "[Compare] ✓ Files are identical",
	"compare.file.diff.count":       "[Compare] ✗ Found %d file differences:",
	"compare.file.diff.truncated":   "[Compare]   ... more diffs truncated (total %d)",
	"compare.no.snapshot":           "Latest snapshot not found, please run build first: %v",
	"compare.rsync.failed":          "[Compare] rsync comparison command failed: %v",
	"compare.cleanup.done":          "[Compare] Temporary snapshot cleaned up",
	"compare.target.not.dir":        "Comparison target is not a directory: %s",
	"compare.src.not.dir":           "Corresponding directory not found in snapshot: %s",

	// ── config_cmd.go ──
	"config.unknown.arg":         "config: unknown argument: %s",
	"config.usage":               "Usage: starsleep config --collect | --export <file> | --import <file>",
	"config.export.usage":        "Usage: starsleep config --export <output.tar.gz>",
	"config.import.usage":        "Usage: starsleep config --import <input.tar.gz>",
	"config.collect.start":       "Collecting inherit files: %d paths",
	"config.collect.copied":      "Collected: %s",
	"config.collect.copy.failed": "Warning: collect failed: %s: %v",
	"config.collect.done":        "Inherit file collection complete, stored in inherit/ directory",
	"config.export.start":        "Exporting config to: %s",
	"config.export.done":         "Export complete: %s",
	"config.export.failed":       "Export failed: %v",
	"config.import.start":        "Importing config from: %s",
	"config.import.done":         "Import complete",
	"config.import.failed":       "Import failed: %v",
	"apply.inherit.store":        "Applying inherit list (from local store): %d paths",
	"inherit.store.not.found":    "Inherit store directory not found: %s, please run starsleep config --collect first",
	"inherit.store.missing":      "Warning: path not found in inherit store, skipping: %s",

	// ── install.go ──
	"install.usage":                   "Usage: starsleep install [--boot <part>] [--root <part>] [--profile <name>] [--name <name>] [--disk <dev>] [--branch <branch>] [--repo <URL>] [--force]\n\n  All options are optional; omitted ones will be asked interactively.",
	"install.missing.boot":            "Error: missing boot partition",
	"install.missing.root":            "Error: missing root partition",
	"install.missing.partition":       "Error: both boot and root partitions must not be empty",
	"install.no.disk":                 "Error: no usable disk found (at least 8GB)",
	"install.separator":               "[Install] ─────────────────────────────────────────────",
	"install.title":                   "[Install] StarSleep System Installation",
	"install.profile":                 "[Install] Preset profile: %s",
	"install.boot.partition":          "[Install] EFI system partition: %s",
	"install.root.partition":          "[Install] Root partition: %s",
	"install.force.mode":              "[Install] --force: skip confirmations",
	"install.confirm.format":          "[Install] Confirm: partition %s has existing filesystem, format and continue? (y/N): ",
	"install.format.boot":             "[Install] Formatting EFI partition: %s (FAT32)...",
	"install.format.root":             "[Install] Formatting root partition: %s (Btrfs)...",
	"install.format.done":             "[Install] Partition formatting complete",
	"install.mounted.detected":        "[Install] Partition %s is already mounted at %s\n",
	"install.mounted.force":           "[Install] --force: auto-unmounting",
	"install.mounted.umount.confirm":  "[Install] Unmount %s to continue? (y/N): ",
	"install.unmounting":              "[Install] Unmounting %s...",
	"install.unmounted":               "[Install] Unmounted %s\n",
	"install.fetch.config":            "[Install] Fetching preset config from GitHub: %s",
	"install.fetch.downloading":       "[Install] Downloading: %s",
	"install.fetch.failed":            "[Install] Failed to fetch preset config %s: %v\n[Install] Please check network connection or manually download config to %s",
	"install.fetch.done":              "[Install] Config download complete",
	"install.mount.target":            "[Install] Mounting target partition to %s",
	"install.mount.boot":              "[Install] Mounting EFI partition %s to %s",
	"install.mount.starsleep":         "[Install] Mounting starsleep subvolume to %s",
	"install.create.subvol":           "[Install] Creating Btrfs subvolume: %s",
	"install.subvol.layout":           "[Install] Subvolume layout created: @, @home, @var, starsleep",
	"install.init.workdir":            "[Install] Initializing work directory...",
	"install.build.start":             "[Install] Starting system build...",
	"install.build.done":              "[Install] System build complete",
	"install.gen.fstab":               "[Install] Generating fstab...",
	"install.fstab.done":              "[Install] fstab generated: %s",
	"install.init.boot":               "[Install] Initializing systemd-boot...",
	"install.bootctl.failed":          "[Install] bootctl install failed: %v",
	"install.boot.done":               "[Install] systemd-boot installed",
	"install.init.shared":             "[Install] Creating shared directories...",
	"install.shared.done":             "[Install] Shared directories ready",
	"install.copy.product":            "[Install] Copying build product to target...",
	"install.copy.done":               "[Install] Product copy complete",
	"install.done":                    "[Install] ✓ Installation complete!",
	"install.reboot.hint":             "[Install] You can now reboot into the new system:",
	"install.reboot.cmd":              "[Install]   sudo reboot",
	"install.summary.title":           "[Install] ═══════════════════════════════════════════════",
	"install.summary.profile":         "[Install] Profile:     %s",
	"install.summary.boot":            "[Install] EFI:         %s",
	"install.summary.root":            "[Install] Root:        %s",
	"install.summary.uuid":            "[Install] Root UUID:   %s",
	"install.summary.snapshot":        "[Install] Snapshot:    %s",
	"install.summary.boot.entry":      "[Install] Boot entry:  starsleep-%s.conf",
	"install.summary.bottom":          "[Install] ═══════════════════════════════════════════════",
	"install.entry.name.prompt":       "[Install] Please enter the system name to display in UEFI firmware boot menu",
	"install.entry.name.input":        "[Install] (default: StarSleep): ",
	"install.entry.name.confirm":      "[Install] UEFI boot name: %s",
	"install.interactive.header":      "[Install] Starting interactive parameter configuration",
	"install.partition.method":        "[Install] Choose partition selection method:",
	"install.partition.method.disk":   "Select by disk (list disks and partitions)",
	"install.partition.method.manual": "Enter partition paths manually",
	"install.partition.method.select": "[Install] Please enter number (1-2): ",
	"install.partition.method.retry":  "[Install] Invalid choice, please re-enter (1-2): ",
	"install.disk.select":             "[Install] Available disks:",
	"install.disk.select.prompt":      "[Install] Please select disk number: ",
	"install.partition.enter.boot":    "[Install] Enter EFI system partition path (e.g. /dev/sda1): ",
	"install.partition.enter.root":    "[Install] Enter root partition path (e.g. /dev/sda2): ",
	"install.profile.select":          "[Install] Choose preset profile:",
	"install.profile.minimal.desc":    "Minimal CLI system (server/headless)",
	"install.profile.gnome.desc":      "GNOME desktop environment (daily use)",
	"install.profile.dev.desc":        "Full development environment (GNOME + dev toolchain + AUR)",
	"install.profile.select.prompt":   "[Install] Please enter number (1-3): ",
	"install.profile.select.retry":    "[Install] Invalid choice, please re-enter (1-3): ",
}
