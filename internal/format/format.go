// Package format creates a new partition table with a single partition
// spanning the whole disk and formats it. Linux uses sfdisk and the mkfs.*
// tools, Windows uses diskpart. The pure parts (file system table, command
// lines, scripts) live here so they are tested on every OS.
package format

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"unicode"
)

// Partition table schemes.
const (
	SchemeMBR = "mbr"
	SchemeGPT = "gpt"
)

var goos = runtime.GOOS

// MaxMBRBytes is the largest disk an MBR partition can span (2^32 sectors of 512 bytes).
const MaxMBRBytes = 1 << 41

// FS is a file system offered for formatting, as shown by the frontend.
type FS struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Tool      string `json:"tool"`      // program that creates it
	Available bool   `json:"available"` // Tool is installed
	MinBytes  int64  `json:"minBytes"`
	MaxBytes  int64  `json:"maxBytes"` // 0: no limit
	LabelMax  int    `json:"labelMax"`
	// LabelBytes: LabelMax counts UTF-8 bytes (ext, XFS, Btrfs, FAT), not characters.
	LabelBytes bool `json:"labelBytes"`
}

// spec describes how a file system is created.
type spec struct {
	FS
	args      []string // tool arguments before label and device
	labelFlag string
	upper     bool   // label stored in upper case (FAT)
	mbrType   string // MBR partition type (sfdisk)
	gptType   string // GPT partition type GUID (sfdisk)
	fatChars  bool   // label restricted to the FAT character set
}

const (
	mib = 1 << 20
	gib = 1 << 30

	mbrLinux     = "83"
	mbrNTFS      = "7" // also exFAT
	mbrFAT32     = "c" // FAT32 LBA
	gptLinux     = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	gptBasicData = "EBD0A0A2-B9E5-4433-87C0-68B6B72699C7"
)

// linuxSpecs: order is the order of the list in the UI.
var linuxSpecs = []spec{
	{FS: FS{ID: "ext4", Name: "ext4", Tool: "mkfs.ext4", MinBytes: 8 * mib, LabelMax: 16, LabelBytes: true}, args: []string{"-F"}, labelFlag: "-L", mbrType: mbrLinux, gptType: gptLinux},
	{FS: FS{ID: "ext3", Name: "ext3", Tool: "mkfs.ext3", MinBytes: 8 * mib, LabelMax: 16, LabelBytes: true}, args: []string{"-F"}, labelFlag: "-L", mbrType: mbrLinux, gptType: gptLinux},
	{FS: FS{ID: "ext2", Name: "ext2", Tool: "mkfs.ext2", MinBytes: 8 * mib, LabelMax: 16, LabelBytes: true}, args: []string{"-F"}, labelFlag: "-L", mbrType: mbrLinux, gptType: gptLinux},
	{FS: FS{ID: "xfs", Name: "XFS", Tool: "mkfs.xfs", MinBytes: 300 * mib, LabelMax: 12, LabelBytes: true}, args: []string{"-f"}, labelFlag: "-L", mbrType: mbrLinux, gptType: gptLinux},
	{FS: FS{ID: "btrfs", Name: "Btrfs", Tool: "mkfs.btrfs", MinBytes: 128 * mib, LabelMax: 255, LabelBytes: true}, args: []string{"-f"}, labelFlag: "-L", mbrType: mbrLinux, gptType: gptLinux},
	{FS: FS{ID: "f2fs", Name: "F2FS", Tool: "mkfs.f2fs", MinBytes: 64 * mib, LabelMax: 512}, args: []string{"-f"}, labelFlag: "-l", mbrType: mbrLinux, gptType: gptLinux},
	{FS: FS{ID: "ntfs", Name: "NTFS", Tool: "mkfs.ntfs", MinBytes: 8 * mib, LabelMax: 32}, args: []string{"-Q"}, labelFlag: "-L", mbrType: mbrNTFS, gptType: gptBasicData},
	{FS: FS{ID: "exfat", Name: "exFAT", Tool: "mkfs.exfat", MinBytes: 8 * mib, LabelMax: 11}, labelFlag: "-L", mbrType: mbrNTFS, gptType: gptBasicData, fatChars: true},
	{FS: FS{ID: "fat32", Name: "FAT32", Tool: "mkfs.fat", MinBytes: 64 * mib, LabelMax: 11, LabelBytes: true}, args: []string{"-F", "32"}, labelFlag: "-n", upper: true, mbrType: mbrFAT32, gptType: gptBasicData, fatChars: true},
}

// windowsSpecs: diskpart formats them all. Windows refuses FAT32 volumes
// larger than 32 GB. ReFS needs an edition that can create it (Pro for
// Workstations, Enterprise, Server): elsewhere diskpart fails with its error.
var windowsSpecs = []spec{
	{FS: FS{ID: "ntfs", Name: "NTFS", Tool: "diskpart", MinBytes: 8 * mib, LabelMax: 32}},
	{FS: FS{ID: "exfat", Name: "exFAT", Tool: "diskpart", MinBytes: 8 * mib, LabelMax: 11}, fatChars: true},
	{FS: FS{ID: "fat32", Name: "FAT32", Tool: "diskpart", MinBytes: 64 * mib, MaxBytes: 32 * gib, LabelMax: 11, LabelBytes: true}, upper: true, fatChars: true},
	{FS: FS{ID: "refs", Name: "ReFS", Tool: "diskpart", MinBytes: 2 * gib, LabelMax: 32}},
}

func specs(goos string) []spec {
	if goos == "windows" {
		return windowsSpecs
	}
	return linuxSpecs
}

func lookup(goos, id string) (spec, bool) {
	for _, s := range specs(goos) {
		if s.ID == id {
			return s, true
		}
	}
	return spec{}, false
}

// list returns the file systems of goos; installed reports whether a tool exists.
func list(goos string, installed func(tool string) bool) []FS {
	out := []FS{}
	for _, s := range specs(goos) {
		fs := s.FS
		fs.Available = installed(fs.Tool)
		out = append(out, fs)
	}
	return out
}

func lookPath(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

// Find returns the file system id of the running OS, with its availability.
func Find(id string) (FS, bool) {
	s, ok := lookup(goos, id)
	if !ok {
		return FS{}, false
	}
	s.Available = lookPath(s.Tool)
	return s.FS, true
}

// List returns the file systems of the running OS.
func List() []FS { return list(goos, lookPath) }

// fatForbidden are the characters not allowed in FAT and exFAT labels.
const fatForbidden = `*?.,;:/\|+=<>[]"`

// CheckLabel reports whether label is valid for the file system id.
func CheckLabel(id, label string) error {
	s, ok := lookup(goos, id)
	if !ok {
		return fmt.Errorf("unknown file system %q", id)
	}
	return checkLabel(s, label)
}

func checkLabel(s spec, label string) error {
	n, unit := len([]rune(label)), "characters"
	if s.LabelBytes {
		n, unit = len(label), "bytes"
	}
	if n > s.LabelMax {
		return fmt.Errorf("label longer than %d %s", s.LabelMax, unit)
	}
	for _, r := range label {
		// '"' would break the quoting of the diskpart script.
		if unicode.IsControl(r) || r == '"' || (s.fatChars && strings.ContainsRune(fatForbidden, r)) {
			return fmt.Errorf("character %q not allowed in the label", r)
		}
	}
	return nil
}

// sfdiskScript creates scheme with one partition over the whole disk.
func sfdiskScript(s spec, scheme string) string {
	if scheme == SchemeGPT {
		return "label: gpt\n,,type=" + s.gptType + "\n"
	}
	return "label: dos\n,,type=" + s.mbrType + "\n"
}

// mkfsArgs is the command line that formats device on Linux.
func mkfsArgs(s spec, device, label string) []string {
	args := append([]string{s.Tool}, s.args...)
	if label != "" {
		if s.upper {
			label = strings.ToUpper(label)
		}
		args = append(args, s.labelFlag, label)
	}
	return append(args, device)
}

// partitionPath is the first partition of a Linux disk: sdb -> sdb1,
// nvme0n1 / mmcblk0 / loop0 -> ...p1.
func partitionPath(diskPath string) string {
	if last := diskPath[len(diskPath)-1]; last >= '0' && last <= '9' {
		return diskPath + "p1"
	}
	return diskPath + "1"
}

// diskpartScript formats disk number n on Windows. The disk may be offline
// (left so by the zero fill) or read-only: noerr ignores "already online".
func diskpartScript(n int, s spec, scheme, label string) string {
	if s.upper {
		label = strings.ToUpper(label)
	}
	format := "format fs=" + s.ID + " quick"
	if label != "" {
		format = fmt.Sprintf("format fs=%s label=\"%s\" quick", s.ID, label)
	}
	return strings.Join([]string{
		fmt.Sprintf("select disk %d", n),
		"online disk noerr",
		"attributes disk clear readonly noerr",
		"clean",
		"convert " + scheme,
		"create partition primary",
		format,
		"assign noerr",
		"",
	}, "\r\n")
}
