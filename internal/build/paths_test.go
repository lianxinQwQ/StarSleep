package build

import (
	"path/filepath"
	"testing"

	"starsleep/internal/config"
)

func TestResolveBuildPathsUsesConfiguredWorkDirForDefaults(t *testing.T) {
	mc := &config.MainConfig{
		Meta: config.MetaConfig{
			WorkDir: "/mnt/starsleep-target/starsleep",
		},
	}

	paths := resolveBuildPaths(mc, Options{})

	if paths.workDir != "/mnt/starsleep-target/starsleep" {
		t.Fatalf("workDir = %q", paths.workDir)
	}
	if want := filepath.Join(paths.workDir, "snapshots"); paths.snapshotDir != want {
		t.Fatalf("snapshotDir = %q, want %q", paths.snapshotDir, want)
	}
	if want := filepath.Join(paths.workDir, "shared/pacman-cache"); paths.pkgCache != want {
		t.Fatalf("pkgCache = %q, want %q", paths.pkgCache, want)
	}
	if want := filepath.Join(paths.workDir, "shared/paru-cache"); paths.paruCache != want {
		t.Fatalf("paruCache = %q, want %q", paths.paruCache, want)
	}
}

func TestResolveBuildPathsHonorsExplicitSnapshotAndPackageCache(t *testing.T) {
	mc := &config.MainConfig{
		Meta: config.MetaConfig{
			WorkDir:     "/custom/work",
			SnapshotDir: "/custom/snapshots",
			PkgCache:    "/custom/pacman-cache",
		},
	}

	paths := resolveBuildPaths(mc, Options{})

	if paths.snapshotDir != "/custom/snapshots" {
		t.Fatalf("snapshotDir = %q", paths.snapshotDir)
	}
	if paths.pkgCache != "/custom/pacman-cache" {
		t.Fatalf("pkgCache = %q", paths.pkgCache)
	}
	if want := filepath.Join("/custom/work", "shared/paru-cache"); paths.paruCache != want {
		t.Fatalf("paruCache = %q, want %q", paths.paruCache, want)
	}
}

func TestResolveBuildPathsOptionsOverrideTemporaryInstallPaths(t *testing.T) {
	mc := &config.MainConfig{
		Meta: config.MetaConfig{
			WorkDir:     "/persisted/wrong",
			SnapshotDir: "/persisted/wrong/snapshots",
			PkgCache:    "/persisted/wrong/shared/pacman-cache",
		},
	}

	paths := resolveBuildPaths(mc, Options{
		WorkDir:     "/mnt/starsleep-target/starsleep",
		SnapshotDir: "/mnt/starsleep-target/starsleep/snapshots",
		PkgCache:    "/mnt/starsleep-target/starsleep/shared/pacman-cache",
		ParuCache:   "/mnt/starsleep-target/starsleep/shared/paru-cache",
	})

	if paths.workDir != "/mnt/starsleep-target/starsleep" {
		t.Fatalf("workDir = %q", paths.workDir)
	}
	if paths.snapshotDir != "/mnt/starsleep-target/starsleep/snapshots" {
		t.Fatalf("snapshotDir = %q", paths.snapshotDir)
	}
	if paths.pkgCache != "/mnt/starsleep-target/starsleep/shared/pacman-cache" {
		t.Fatalf("pkgCache = %q", paths.pkgCache)
	}
	if paths.paruCache != "/mnt/starsleep-target/starsleep/shared/paru-cache" {
		t.Fatalf("paruCache = %q", paths.paruCache)
	}
}
