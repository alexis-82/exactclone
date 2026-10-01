package disk

import (
	"os/exec"
)

// List returns the physical disks of the machine.
func List() ([]Disk, error) {
	out, err := exec.Command("lsblk", "-J", "-b", "-o", lsblkColumns).Output()
	if err != nil {
		// util-linux < 2.37 does not know MOUNTPOINTS.
		out, err = exec.Command("lsblk", "-J", "-b", "-o", lsblkColumnsOld).Output()
		if err != nil {
			return nil, err
		}
	}
	return parseLsblk(out, DevSafe())
}
