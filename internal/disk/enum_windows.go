package disk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	ioctlStorageQueryProperty        = 0x002D1400
	ioctlDiskGetDriveGeometryEx      = 0x000700A0
	ioctlDiskGetDriveLayoutEx        = 0x00070050
	ioctlVolumeGetVolumeDiskExtents  = 0x00560000
	partitionStyleMBR                = 0
	partitionStyleGPT                = 1
	partitionInformationExSize       = 144
	driveLayoutHeaderSize            = 48
	maxPhysicalDrives                = 64
	diskExtentSize                   = 24
	busTypeUSB, busTypeSATA          = 7, 11
	busTypeNVMe, busTypeATA          = 17, 3
	busTypeSAS, busTypeSCSI          = 10, 1
	busTypeFileBackedVirtual, busSD  = 15, 12
	busTypeRAID, busTypeSpaces       = 8, 16
	busTypeMMC                       = 13
	storageDeviceDescriptorBusOffset = 28
)

// windowsReadableFS are the file systems Windows reads natively.
var windowsReadableFS = map[string]bool{"ntfs": true, "vfat": true, "exfat": true, "refs": true}

func busName(t uint32) string {
	switch t {
	case busTypeUSB:
		return "usb"
	case busTypeSATA, busTypeATA:
		return "sata"
	case busTypeNVMe:
		return "nvme"
	case busTypeSAS:
		return "sas"
	case busTypeSCSI:
		return "scsi"
	case busTypeFileBackedVirtual:
		return "virtual"
	case busSD, busTypeMMC:
		return "sd"
	case busTypeRAID, busTypeSpaces:
		return "raid"
	}
	return "other"
}

// NormalizeFS maps Windows file system names to lsblk-style names.
func NormalizeFS(name string) string {
	switch n := strings.ToLower(name); n {
	case "fat", "fat32":
		return "vfat"
	default:
		return n
	}
}

func openNoAccess(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}

func openRead(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}

func ioctl(h windows.Handle, code uint32, in []byte, outSize int) ([]byte, error) {
	for {
		out := make([]byte, outSize)
		var inPtr *byte
		if len(in) > 0 {
			inPtr = &in[0]
		}
		var n uint32
		err := windows.DeviceIoControl(h, code, inPtr, uint32(len(in)), &out[0], uint32(len(out)), &n, nil)
		if err == windows.ERROR_INSUFFICIENT_BUFFER || err == windows.ERROR_MORE_DATA {
			outSize *= 2
			continue
		}
		if err != nil {
			return nil, err
		}
		return out[:n], nil
	}
}

func cString(buf []byte, off uint32) string {
	if off == 0 || int(off) >= len(buf) {
		return ""
	}
	end := int(off)
	for end < len(buf) && buf[end] != 0 {
		end++
	}
	return strings.TrimSpace(string(buf[off:end]))
}

// List returns the physical disks of the machine.
func List() ([]Disk, error) {
	vols := listVolumes()
	systemDisks := systemDiskNumbers(vols)
	disks := []Disk{}
	for i := 0; i < maxPhysicalDrives; i++ {
		path := fmt.Sprintf(`\\.\PhysicalDrive%d`, i)
		h, err := openNoAccess(path)
		if err != nil {
			continue
		}
		d, err := describeDisk(h, i, path)
		windows.CloseHandle(h)
		if err != nil {
			continue
		}
		d.IsSystem = systemDisks[uint32(i)]
		d.Partitions = partitionsOf(path, uint32(i), vols)
		disks = append(disks, d)
	}
	if len(disks) == 0 {
		return nil, errors.New("no physical disk found")
	}
	return disks, nil
}

func describeDisk(h windows.Handle, n int, path string) (Disk, error) {
	d := Disk{ID: fmt.Sprintf("PhysicalDrive%d", n), Path: path}
	// STORAGE_PROPERTY_QUERY{PropertyId: StorageDeviceProperty, QueryType: PropertyStandardQuery}
	desc, err := ioctl(h, ioctlStorageQueryProperty, make([]byte, 12), 1024)
	if err != nil {
		return d, err
	}
	removable := desc[10] != 0
	vendor := cString(desc, binary.LittleEndian.Uint32(desc[12:]))
	product := cString(desc, binary.LittleEndian.Uint32(desc[16:]))
	d.Serial = cString(desc, binary.LittleEndian.Uint32(desc[24:]))
	d.Bus = busName(binary.LittleEndian.Uint32(desc[storageDeviceDescriptorBusOffset:]))
	d.Model = strings.TrimSpace(vendor + " " + product)
	d.Removable = removable || d.Bus == "usb"

	geo, err := ioctl(h, ioctlDiskGetDriveGeometryEx, nil, 256)
	if err != nil {
		return d, err // no media (e.g. empty card reader)
	}
	d.SizeBytes = int64(binary.LittleEndian.Uint64(geo[24:]))
	return d, nil
}

// volume is a mounted Windows volume located on a single disk extent.
type volume struct {
	guidPath  string // \\?\Volume{GUID}\
	disk      uint32
	offset    int64
	length    int64
	fs        string
	label     string
	paths     []string // drive letters / mount folders
	multiDisk bool     // spanned/striped dynamic volume
}

func listVolumes() []volume {
	var vols []volume
	buf := make([]uint16, windows.MAX_PATH)
	h, err := windows.FindFirstVolume(&buf[0], uint32(len(buf)))
	if err != nil {
		return nil
	}
	defer windows.FindVolumeClose(h)
	for {
		name := windows.UTF16ToString(buf)
		if v, ok := describeVolume(name); ok {
			vols = append(vols, v...)
		}
		if err := windows.FindNextVolume(h, &buf[0], uint32(len(buf))); err != nil {
			break
		}
	}
	return vols
}

func describeVolume(guidPath string) ([]volume, bool) {
	h, err := openNoAccess(strings.TrimSuffix(guidPath, `\`))
	if err != nil {
		return nil, false
	}
	out, err := ioctl(h, ioctlVolumeGetVolumeDiskExtents, nil, 8+diskExtentSize*4)
	windows.CloseHandle(h)
	if err != nil {
		return nil, false // CD-ROM, unmounted, ...
	}
	count := int(binary.LittleEndian.Uint32(out))
	base := volume{guidPath: guidPath, multiDisk: count > 1, paths: volumePaths(guidPath)}
	base.fs, base.label = volumeInfo(guidPath)
	var vols []volume
	for i := 0; i < count; i++ {
		e := out[8+i*diskExtentSize:]
		v := base
		v.disk = binary.LittleEndian.Uint32(e)
		v.offset = int64(binary.LittleEndian.Uint64(e[8:]))
		v.length = int64(binary.LittleEndian.Uint64(e[16:]))
		vols = append(vols, v)
	}
	return vols, true
}

func volumeInfo(guidPath string) (fs, label string) {
	root, _ := windows.UTF16PtrFromString(guidPath)
	lbl := make([]uint16, windows.MAX_PATH+1)
	fsn := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeInformation(root, &lbl[0], uint32(len(lbl)), nil, nil, nil, &fsn[0], uint32(len(fsn))); err != nil {
		return "", ""
	}
	return NormalizeFS(windows.UTF16ToString(fsn)), windows.UTF16ToString(lbl)
}

func volumePaths(guidPath string) []string {
	name, _ := windows.UTF16PtrFromString(guidPath)
	buf := make([]uint16, 1024)
	var n uint32
	if err := windows.GetVolumePathNamesForVolumeName(name, &buf[0], uint32(len(buf)), &n); err != nil {
		return nil
	}
	var paths []string
	start := 0
	for i := 0; i < int(n) && i < len(buf); i++ {
		if buf[i] == 0 {
			if i > start {
				paths = append(paths, windows.UTF16ToString(buf[start:i]))
			}
			start = i + 1
		}
	}
	return paths
}

// partitionsOf reads the partition table (needs admin rights) and matches each
// partition with its volume. Without admin rights only partitions that have a
// volume are listed.
func partitionsOf(path string, disk uint32, vols []volume) []Partition {
	parts := []Partition{}
	h, err := openRead(path)
	if err == nil {
		layout, lerr := ioctl(h, ioctlDiskGetDriveLayoutEx, nil, driveLayoutHeaderSize+partitionInformationExSize*16)
		windows.CloseHandle(h)
		if lerr == nil {
			return parseLayout(layout, disk, vols)
		}
	}
	for _, v := range vols {
		if v.disk == disk {
			parts = append(parts, partitionFromVolume(Partition{SizeBytes: v.length}, v))
		}
	}
	return parts
}

func parseLayout(b []byte, disk uint32, vols []volume) []Partition {
	style := binary.LittleEndian.Uint32(b)
	count := int(binary.LittleEndian.Uint32(b[4:]))
	parts := []Partition{}
	for i := 0; i < count; i++ {
		e := b[driveLayoutHeaderSize+i*partitionInformationExSize:]
		offset := int64(binary.LittleEndian.Uint64(e[8:]))
		length := int64(binary.LittleEndian.Uint64(e[16:]))
		number := int(binary.LittleEndian.Uint32(e[24:]))
		if length == 0 || number == 0 {
			continue
		}
		if style == partitionStyleMBR && (e[32] == 0 || e[32] == 0x05 || e[32] == 0x0F) {
			continue // unused or extended container
		}
		p := Partition{ID: fmt.Sprintf("Disk%dPartition%d", disk, number), Number: number, SizeBytes: length}
		for _, v := range vols {
			if v.disk == disk && v.offset == offset {
				p = partitionFromVolume(p, v)
				break
			}
		}
		parts = append(parts, p)
	}
	return parts
}

func partitionFromVolume(p Partition, v volume) Partition {
	if p.ID == "" {
		p.ID = v.guidPath
	}
	p.Path = v.guidPath
	p.FSType = v.fs
	p.Label = v.label
	p.MountPoints = v.paths
	p.Supported = windowsReadableFS[v.fs] && !v.multiDisk
	return p
}

// systemDiskNumbers returns the disks holding the Windows volume (C:) and
// the boot "system partition" (EFI/boot manager), which is often on another disk.
func systemDiskNumbers(vols []volume) map[uint32]bool {
	out := map[uint32]bool{}
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	mp, _ := windows.UTF16PtrFromString(drive + `\`)
	buf := make([]uint16, windows.MAX_PATH)
	if err := windows.GetVolumeNameForVolumeMountPoint(mp, &buf[0], uint32(len(buf))); err == nil {
		sysVols, _ := describeVolume(windows.UTF16ToString(buf))
		for _, v := range sysVols {
			out[v.disk] = true
		}
	}
	for d := range bootDiskNumbers(vols) {
		out[d] = true
	}
	return out
}

// bootDiskNumbers maps HKLM\SYSTEM\Setup\SystemPartition (an NT device name
// such as \Device\HarddiskVolume1) to the disks of that volume.
func bootDiskNumbers(vols []volume) map[uint32]bool {
	out := map[uint32]bool{}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\Setup`, registry.QUERY_VALUE)
	if err != nil {
		return out
	}
	defer k.Close()
	device, _, err := k.GetStringValue("SystemPartition")
	if err != nil || device == "" {
		return out
	}
	for _, v := range vols {
		if strings.EqualFold(ntDeviceName(v.guidPath), device) {
			out[v.disk] = true
		}
	}
	return out
}

// ntDeviceName returns the NT device (\Device\HarddiskVolumeN) of a volume GUID path.
func ntDeviceName(guidPath string) string {
	name := strings.TrimSuffix(strings.TrimPrefix(guidPath, `\\?\`), `\`)
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return ""
	}
	buf := make([]uint16, windows.MAX_PATH)
	if _, err := windows.QueryDosDevice(p, &buf[0], uint32(len(buf))); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf)
}
