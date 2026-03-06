package main

var enMessages = map[string]string{
	// ── main.go ──
	"unknown.cmd": "Unknown command: %s",
	"usage": `StarSleep - Immutable System Layered Build Tool

Usage:
  starsleep <command> [options]

Commands:
  build     Build system snapshot in layers
  flatten   Deploy snapshot to boot
  init      Initialize work environment
  maintain  Dynamic maintenance mode (operate on running system)

General options:
  -c,  --config <path>   Specify config directory
  -cp, --copy <path>     Copy config to default path before running
  --lang <lang>          UI language (zh/en, default: auto-detect)

build options:
  --clean [layers...]  Clean layers and rebuild (all if none specified)
  --verify             Verify flatten consistency before snapshot

flatten options:
  --list              List deployed boot entries
  --remove <name>     Remove boot entry
  <snapshot-path>     Snapshot to deploy (default: latest)`,

	// ── util.go ──
	"fatal.prefix": "[StarSleep] Error: %s\n",
	"need.root":    "Root privileges required, please run with sudo",

	// ── build.go ──
	"build.unknown.arg":     "build: unknown argument: %s",
	"verify.tool.not.found": "Verify tool not found: %s",
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
  │   ├── home/
  │   └── pacman-cache/
  ├── config/          # Configuration files
  │   ├── layers/      # Layer definitions (YAML)
  │   └── inherit.list # Inherit list
  ├── work/            # Build workspace
  │   ├── flat/        # Flat subvolume (Btrfs)
  │   ├── merged/      # OverlayFS merged mount
  │   └── ovl_work/    # OverlayFS work dir
  └── logs/            # Build logs`,
	"init.next": "Next step: sudo starsleep build",

	// ── maintain.go ──
	"maintain.unknown.arg":    "maintain: unknown argument: %s",
	"maintain.separator":      "[Maintain] ═══════════════════════════════════════════════",
	"maintain.title":          "[Maintain] StarSleep Dynamic Maintenance Mode",
	"maintain.config.dir":     "[Maintain] Config dir: %s",
	"maintain.layer.count":    "[Maintain] Layers: %d",
	"maintain.official.pkgs":  "[Maintain] Official repo packages: %d",
	"maintain.aur.pkgs":       "[Maintain] AUR packages: %d",
	"maintain.services.count": "[Maintain] Services: %d",
	"maintain.step1":          "[Maintain] Step 1/4: Cleaning up extra packages...",
	"maintain.step2":          "[Maintain] Step 2/4: Syncing official repo packages...",
	"maintain.step2.skip":     "[Maintain] Step 2/4: No official packages to install",
	"maintain.step3":          "[Maintain] Step 3/4: Syncing AUR packages...",
	"maintain.step3.skip":     "[Maintain] Step 3/4: No AUR packages to install",
	"maintain.paru.warn":      "[Maintain] Warning: paru installation failed: %v",
	"maintain.step4":          "[Maintain] Step 4/4: Enabling systemd services...",
	"maintain.step4.skip":     "[Maintain] Step 4/4: No services to enable",
	"maintain.done":           "[Maintain] ✓ Dynamic maintenance complete",
	"maintain.query.failed":   "[Maintain] Warning: failed to query explicit packages: %v",
	"maintain.demote":         "[Maintain] Demoting %d packages to deps: %s",
	"maintain.orphans":        "[Maintain] Cleaning orphan deps: %s",
	"maintain.orphans.failed": "[Maintain] Warning: failed to clean orphan deps: %v",

	// ── sync.go ──
	"sync.unknown.tool": "Unknown tool: %s\nSupported tools: pacstrap, pacman, paru, enable_service",
	"sync.stage.done":   "[Sync] ✓ Stage %s sync complete",
	"sync.separator":    "[Sync] ─────────────────────────────────────────────",
	"sync.stage":        "[Sync] Stage: %s",
	"sync.tool":         "[Sync] Tool: %s",
	"sync.target":       "[Sync] Target: %s",
	"sync.packages":     "[Sync] Packages: %d",
	"sync.services":     "[Sync] Services: %d",

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
	"sync.pacstrap":    "[Sync] Initializing base root filesystem with pacstrap...",
	"sync.incremental": "[Sync] Existing system detected, performing incremental sync...",
	"sync.fresh":       "[Sync] Fresh bootstrap...",
	"pacstrap.failed":  "pacstrap failed: %v",

	// ── paru.go ──
	"sync.paru":   "[Sync] Installing AUR packages with paru...",
	"paru.failed": "paru installation failed: %v",

	// ── config.go ──
	"cfg.read":          "reading %s: %w",
	"cfg.parse":         "parsing %s: %w",
	"cfg.scan":          "scanning layer configs: %w",
	"cfg.no.files":      "no config files found in %s",
	"cfg.copied":        "Config copied: %s → %s",
	"cfg.copy.failed":   "Failed to copy config: %v",
	"cfg.src.not.exist": "source config does not exist: %s",
	"cfg.src.not.dir":   "source config is not a directory: %s",
	"cfg.read.layers":   "reading source layers: %w",

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
}
