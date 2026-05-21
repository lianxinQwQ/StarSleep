package deploy

import (
	"strings"
	"testing"
)

func TestBootEntryBootsStarsleepSnapshot(t *testing.T) {
	entry := bootEntryContent(Options{
		EntryTitle:   "StarSleep",
		RootUUID:     "ROOT-UUID",
		SubvolPrefix: "starsleep/snapshots",
		KernelOpts:   "rw quiet",
	}, "snapshot-123", "initrd /starsleep/snapshot-123/initramfs-linux.img")

	if !strings.Contains(entry, "rootflags=subvol=/starsleep/snapshots/snapshot-123") {
		t.Fatalf("boot entry does not boot snapshot:\n%s", entry)
	}
	if strings.Contains(entry, "subvol=@") {
		t.Fatalf("boot entry should not boot @:\n%s", entry)
	}
}
