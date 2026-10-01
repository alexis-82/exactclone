package disk

import (
	"os"
	"reflect"
	"testing"
)

func load(t *testing.T, name string, includeLoop bool) []Disk {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	disks, err := parseLsblk(data, includeLoop)
	if err != nil {
		t.Fatal(err)
	}
	return disks
}

func TestParseUSB(t *testing.T) {
	disks := load(t, "lsblk_usb.json", false)
	if len(disks) != 2 {
		t.Fatalf("got %d disks, want 2 (rom and loop excluded)", len(disks))
	}
	sys, usb := disks[0], disks[1]
	if !sys.IsSystem || sys.Removable || sys.Bus != "sata" {
		t.Fatalf("system disk = %+v", sys)
	}
	if usb.IsSystem || !usb.Removable || usb.Bus != "usb" || usb.SizeBytes != 15938355200 || usb.Model != "DataTraveler 3.0" {
		t.Fatalf("usb disk = %+v", usb)
	}
	p := usb.Partitions[0]
	want := Partition{ID: "sdb1", Path: "/dev/sdb1", Number: 1, SizeBytes: 15937306624, FSType: "exfat", Label: "KINGSTON",
		MountPoints: []string{"/media/mario/KINGSTON"}, Supported: true}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("partition = %+v\nwant        %+v", p, want)
	}
}

func TestParseIncludesLoopInDevSafeMode(t *testing.T) {
	disks := load(t, "lsblk_usb.json", true)
	if len(disks) != 3 || disks[2].ID != "loop0" || disks[2].Bus != "loop" {
		t.Fatalf("disks = %+v", disks)
	}
}

func TestParseNVMe(t *testing.T) {
	disks := load(t, "lsblk_nvme.json", false)
	d := disks[0]
	if d.IsSystem || d.Bus != "nvme" || len(d.Partitions) != 3 {
		t.Fatalf("disk = %+v", d)
	}
	nums := []int{d.Partitions[0].Number, d.Partitions[1].Number, d.Partitions[2].Number}
	if !reflect.DeepEqual(nums, []int{1, 2, 3}) {
		t.Fatalf("numbers = %v", nums)
	}
	if !d.Partitions[0].Supported || d.Partitions[1].Supported || !d.Partitions[2].Supported {
		t.Fatalf("supported flags wrong: %+v", d.Partitions)
	}
	if len(d.Partitions[0].MountPoints) != 0 {
		t.Fatalf("null mountpoints must give an empty list, got %v", d.Partitions[0].MountPoints)
	}
}

func TestParseLUKSAndOldLsblk(t *testing.T) {
	disks := load(t, "lsblk_lvm_old.json", false)
	d := disks[0]
	if !d.IsSystem {
		t.Fatal("root on LVM inside LUKS must mark the disk as system")
	}
	if d.SizeBytes != 2000398934016 || d.Removable != true {
		t.Fatalf("string size / usb removable not parsed: %+v", d)
	}
	if got := d.Partitions[0].MountPoints; !reflect.DeepEqual(got, []string{"/mnt/oldboot"}) {
		t.Fatalf("old MOUNTPOINT column not parsed: %v", got)
	}
	if d.Partitions[1].Supported {
		t.Fatal("LUKS partition must be unsupported")
	}
}

func TestPartitionNumber(t *testing.T) {
	for name, want := range map[string]int{"sdb1": 1, "sdb12": 12, "nvme0n1p2": 2, "mmcblk0p1": 1, "sdb": 0} {
		if got := partitionNumber(name); got != want {
			t.Errorf("partitionNumber(%q) = %d, want %d", name, got, want)
		}
	}
}
