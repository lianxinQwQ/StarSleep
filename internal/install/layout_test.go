package install

import (
	"strings"
	"testing"
)

func TestInstallTopLevelSubvolumesOnlyStarsleep(t *testing.T) {
	subvols := installTopLevelSubvolumes()
	if len(subvols) != 1 || subvols[0] != "starsleep" {
		t.Fatalf("installTopLevelSubvolumes = %#v", subvols)
	}
}

func TestSnapshotFstabMountsStarsleepSharedSubvolumes(t *testing.T) {
	fstab := snapshotFstabContent("ROOT-UUID", "BOOT-UUID")

	for _, want := range []string{
		"subvol=starsleep/shared/home",
		"subvol=starsleep/shared/root",
		"subvol=starsleep/shared/pacman-cache",
		"subvol=starsleep/shared/paru-cache",
		"subvol=starsleep",
	} {
		if !strings.Contains(fstab, want) {
			t.Fatalf("fstab missing %q:\n%s", want, fstab)
		}
	}

	for _, forbidden := range []string{
		"subvol=@",
		"subvol=@home",
		"subvol=@var",
	} {
		if strings.Contains(fstab, forbidden) {
			t.Fatalf("fstab should not contain %q:\n%s", forbidden, fstab)
		}
	}
}
