package disk

import (
	"os"
	"strings"
	"testing"
)

// Runs without admin rights: the partition table is then read from volumes only.
func TestListWindows(t *testing.T) {
	disks, err := List()
	if err != nil {
		t.Fatal(err)
	}
	systemDrive := os.Getenv("SystemDrive") + `\`
	systems := 0
	for _, d := range disks {
		t.Logf("%s %q bus=%s size=%d removable=%v system=%v", d.ID, d.Model, d.Bus, d.SizeBytes, d.Removable, d.IsSystem)
		for _, p := range d.Partitions {
			t.Logf("   #%d %s fs=%s label=%q size=%d mounts=%v supported=%v", p.Number, p.Path, p.FSType, p.Label, p.SizeBytes, p.MountPoints, p.Supported)
		}
		if d.SizeBytes <= 0 || !strings.HasPrefix(d.Path, `\\.\PhysicalDrive`) {
			t.Errorf("bad disk %+v", d)
		}
		if d.IsSystem {
			systems++
			found := false
			for _, p := range d.Partitions {
				for _, m := range p.MountPoints {
					if strings.EqualFold(m, systemDrive) {
						found = true
						if p.FSType != "ntfs" || !p.Supported {
							t.Errorf("system partition = %+v", p)
						}
					}
				}
			}
			if !found {
				t.Errorf("system disk %s has no %s partition", d.ID, systemDrive)
			}
		}
	}
	if systems != 1 {
		t.Fatalf("%d system disks, want 1", systems)
	}
}

// Review R2: the disk with the boot system partition (EFI) is a system disk
// even when it is not the disk of C:.
func TestBootSystemPartitionDisk(t *testing.T) {
	vols := listVolumes()
	boot := bootDiskNumbers(vols)
	if len(boot) == 0 {
		t.Skip("SystemPartition not set or not resolvable on this machine")
	}
	sys := systemDiskNumbers(vols)
	for d := range boot {
		if !sys[d] {
			t.Fatalf("boot disk %d not marked as system", d)
		}
	}
	found := false
	for _, v := range vols {
		if boot[v.disk] && strings.HasPrefix(ntDeviceName(v.guidPath), `\Device\HarddiskVolume`) {
			found = true
		}
	}
	if !found {
		t.Fatal("NT device names not resolved")
	}
}

func TestNormalizeFS(t *testing.T) {
	for in, want := range map[string]string{"FAT32": "vfat", "FAT": "vfat", "NTFS": "ntfs", "exFAT": "exfat"} {
		if got := NormalizeFS(in); got != want {
			t.Errorf("NormalizeFS(%q) = %q", in, got)
		}
	}
}
