package init_env

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceLayoutSeparatesDirsAndSubvolumes(t *testing.T) {
	root := t.TempDir()
	dirs := WorkspaceDirs(root)
	subvols := WorkspaceSubvolumes(root)

	for _, want := range []string{
		filepath.Join(root, "config/files"),
		filepath.Join(root, "config/inherit"),
		filepath.Join(root, "var/log"),
		filepath.Join(root, "var/cache"),
	} {
		if !containsPath(dirs, want) {
			t.Fatalf("WorkspaceDirs missing %s", want)
		}
	}

	for _, want := range []string{
		filepath.Join(root, "shared/home"),
		filepath.Join(root, "shared/root"),
		filepath.Join(root, "shared/pacman-cache"),
		filepath.Join(root, "shared/paru-cache"),
	} {
		if !containsPath(subvols, want) {
			t.Fatalf("WorkspaceSubvolumes missing %s", want)
		}
		if containsPath(dirs, want) {
			t.Fatalf("WorkspaceDirs should not precreate subvolume %s", want)
		}
	}
}

func TestEnsureBtrfsSubvolumeRejectsExistingPlainDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain")
	if err := mkdirAll(path); err != nil {
		t.Fatal(err)
	}

	err := ensureBtrfsSubvolume(path, func(name string, args ...string) error {
		if name == "btrfs" && len(args) >= 2 && args[0] == "subvolume" && args[1] == "show" {
			return errors.New("not a subvolume")
		}
		t.Fatalf("unexpected command: %s %#v", name, args)
		return nil
	})
	if err == nil {
		t.Fatal("expected plain directory to be rejected")
	}
}

func TestEnsureBtrfsSubvolumeCreatesMissingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	var created bool

	err := ensureBtrfsSubvolume(path, func(name string, args ...string) error {
		if name == "btrfs" && len(args) == 3 && args[0] == "subvolume" && args[1] == "create" && args[2] == path {
			created = true
			return nil
		}
		t.Fatalf("unexpected command: %s %#v", name, args)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("missing path was not created as a subvolume")
	}
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
