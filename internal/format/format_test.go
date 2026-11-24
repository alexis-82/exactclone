package format

import (
	"fmt"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	got := list("linux", func(tool string) bool { return tool == "mkfs.ext4" })
	if len(got) != len(linuxSpecs) || got[0].ID != "ext4" || !got[0].Available || got[1].Available {
		t.Fatalf("list = %+v", got)
	}
	win := list("windows", func(string) bool { return true })
	if fmt.Sprint(ids(win)) != "[ntfs exfat fat32 refs]" {
		t.Fatalf("windows = %v", ids(win))
	}
	if fat, _ := lookup("windows", "fat32"); fat.MaxBytes != 32*gib {
		t.Fatalf("windows fat32 max = %d", fat.MaxBytes)
	}
}

func ids(l []FS) []string {
	var out []string
	for _, f := range l {
		out = append(out, f.ID)
	}
	return out
}

func TestCheckLabel(t *testing.T) {
	ext4, _ := lookup("linux", "ext4")
	fat, _ := lookup("linux", "fat32")
	ntfs, _ := lookup("linux", "ntfs")
	cases := []struct {
		s     spec
		label string
		ok    bool
	}{
		{ext4, "", true},
		{ext4, "dati backup", true},
		{ext4, "1234567890123456", true},
		{ext4, "12345678901234567", false},
		{ext4, "àèìòù-àèìò", false}, // 10 characters, 19 bytes
		{ext4, "àèìòù-ab", true},    // 13 bytes
		{ext4, `a"b`, false},
		{ext4, "a\nb", false},
		{ntfs, "àèìòù-àèìòù-àèìòù-àèìòù-àèìòù-ab", true}, // 32 characters
		{ntfs, "àèìòù-àèìòù-àèìòù-àèìòù-àèìòù-abc", false},
		{fat, "USB_2026", true},
		{fat, "USB.2026", false},
		{fat, "123456789012", false},
	}
	for _, c := range cases {
		if err := checkLabel(c.s, c.label); (err == nil) != c.ok {
			t.Errorf("%s %q: err = %v, want ok=%v", c.s.ID, c.label, err, c.ok)
		}
	}
}

func TestMkfsArgs(t *testing.T) {
	cases := []struct {
		id, label, want string
	}{
		{"ext4", "dati", "mkfs.ext4 -F -L dati /dev/sdb1"},
		{"ext4", "", "mkfs.ext4 -F /dev/sdb1"},
		{"fat32", "usb", "mkfs.fat -F 32 -n USB /dev/sdb1"},
		{"ntfs", "Win", "mkfs.ntfs -Q -L Win /dev/sdb1"},
		{"f2fs", "x", "mkfs.f2fs -f -l x /dev/sdb1"},
		{"exfat", "", "mkfs.exfat /dev/sdb1"},
	}
	for _, c := range cases {
		s, _ := lookup("linux", c.id)
		if got := strings.Join(mkfsArgs(s, "/dev/sdb1", c.label), " "); got != c.want {
			t.Errorf("%s: %q, want %q", c.id, got, c.want)
		}
	}
	// The spec's args slice must not be modified by append.
	s, _ := lookup("linux", "fat32")
	mkfsArgs(s, "/dev/a1", "x")
	if fmt.Sprint(s.args) != "[-F 32]" {
		t.Fatalf("args modified: %v", s.args)
	}
}

func TestSfdiskScript(t *testing.T) {
	ext4, _ := lookup("linux", "ext4")
	fat, _ := lookup("linux", "fat32")
	if got := sfdiskScript(ext4, SchemeMBR); got != "label: dos\n,,type=83\n" {
		t.Errorf("ext4 mbr: %q", got)
	}
	if got := sfdiskScript(fat, SchemeGPT); got != "label: gpt\n,,type="+gptBasicData+"\n" {
		t.Errorf("fat32 gpt: %q", got)
	}
}

func TestPartitionPath(t *testing.T) {
	for in, want := range map[string]string{
		"/dev/sdb":     "/dev/sdb1",
		"/dev/nvme0n1": "/dev/nvme0n1p1",
		"/dev/mmcblk0": "/dev/mmcblk0p1",
		"/dev/loop7":   "/dev/loop7p1",
	} {
		if got := partitionPath(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

func TestDiskpartScript(t *testing.T) {
	fat, _ := lookup("windows", "fat32")
	got := diskpartScript(2, fat, SchemeMBR, "usb")
	want := "select disk 2\r\nonline disk noerr\r\nattributes disk clear readonly noerr\r\nclean\r\n" +
		"convert mbr\r\ncreate partition primary\r\nformat fs=fat32 label=\"USB\" quick\r\nassign noerr\r\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	ntfs, _ := lookup("windows", "ntfs")
	if got := diskpartScript(0, ntfs, SchemeGPT, ""); !strings.Contains(got, "convert gpt\r\n") || !strings.Contains(got, "format fs=ntfs quick\r\n") {
		t.Errorf("ntfs gpt:\n%s", got)
	}
}
