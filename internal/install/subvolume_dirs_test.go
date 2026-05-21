package install

import (
	"path/filepath"
	"testing"

	"starsleep/internal/init_env"
)

func TestStarsleepMountDirsDoNotPrecreateCacheSubvolumes(t *testing.T) {
	root := t.TempDir()
	dirs := init_env.WorkspaceDirs(root)

	for _, forbidden := range []string{
		filepath.Join(root, "shared/home"),
		filepath.Join(root, "shared/root"),
		filepath.Join(root, "shared/pacman-cache"),
		filepath.Join(root, "shared/paru-cache"),
	} {
		for _, dir := range dirs {
			if dir == forbidden {
				t.Fatalf("starsleep mount dirs precreate cache subvolume path %s", forbidden)
			}
		}
	}
}
