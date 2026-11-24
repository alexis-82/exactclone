package format

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// The scripts are run by the real sfdisk on an image file (no root needed).
func TestSfdiskScript(t *testing.T) {
	if _, err := exec.LookPath("sfdisk"); err != nil {
		t.Skip("sfdisk not installed")
	}
	for _, s := range linuxSpecs {
		for scheme, label := range map[string]string{SchemeMBR: "dos", SchemeGPT: "gpt"} {
			img := sparseFile(t, 64*mib)
			cmd := exec.Command("sfdisk", "--wipe", "always", "--wipe-partitions", "always", img)
			cmd.Stdin = strings.NewReader(sfdiskScript(s, scheme))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s %s: %v\n%s", s.ID, scheme, err, out)
			}
			out, err := exec.Command("sfdisk", "-d", img).Output()
			if err != nil {
				t.Fatal(err)
			}
			typ := s.mbrType
			if scheme == SchemeGPT {
				typ = s.gptType
			}
			dump := string(out)
			if !strings.Contains(dump, "label: "+label) || !strings.Contains(dump, "type="+typ) || strings.Count(dump, "start=") != 1 {
				t.Errorf("%s %s:\n%s", s.ID, scheme, dump)
			}
		}
	}
}

// Every installed mkfs runs with the generated arguments on an image file;
// blkid must find the file system and the label.
func TestMkfsRuns(t *testing.T) {
	if _, err := exec.LookPath("blkid"); err != nil {
		t.Skip("blkid not installed")
	}
	blkType := map[string]string{"fat32": "vfat"}
	for _, s := range linuxSpecs {
		t.Run(s.ID, func(t *testing.T) {
			if !lookPath(s.Tool) {
				t.Skip(s.Tool + " not installed")
			}
			img := sparseFile(t, 512*mib)
			args := mkfsArgs(s, img, "Test1")
			if s.ID == "ntfs" {
				// mkfs.ntfs refuses a regular file without -F; a block device needs none.
				args = append(args[:len(args)-1], "-F", img)
			}
			if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
				t.Fatalf("%v: %v\n%s", args, err, out)
			}
			want := s.ID
			if w, ok := blkType[s.ID]; ok {
				want = w
			}
			if got := blkid(t, img, "TYPE"); got != want {
				t.Errorf("TYPE = %q, want %q", got, want)
			}
			if got := blkid(t, img, "LABEL"); !strings.EqualFold(got, "Test1") {
				t.Errorf("LABEL = %q", got)
			}
		})
	}
}

func sparseFile(t *testing.T, size int64) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "disk.img")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	return p
}

func blkid(t *testing.T, path, tag string) string {
	t.Helper()
	out, _ := exec.Command("blkid", "-p", "-o", "value", "-s", tag, path).Output()
	return strings.TrimSpace(string(out))
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
