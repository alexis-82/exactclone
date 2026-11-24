package format

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"exactclone/internal/disk"
)

// partitionWait is how long to wait for the new partition device node.
const partitionWait = 10 * time.Second

// Run writes a new partition table (scheme) with one partition over the
// whole disk d and formats it with the file system fsID. The partitions of d
// must already be unmounted.
func Run(ctx context.Context, d disk.Disk, fsID, scheme, label string) error {
	s, ok := lookup(goos, fsID)
	if !ok {
		return fmt.Errorf("unknown file system %q", fsID)
	}
	// --wipe removes old signatures from the disk and from the new partition
	// area, so blkid does not find the previous file system afterwards.
	sf := exec.CommandContext(ctx, "sfdisk", "--wipe", "always", "--wipe-partitions", "always", d.Path)
	sf.Stdin = strings.NewReader(sfdiskScript(s, scheme))
	if err := run(sf); err != nil {
		return err
	}
	part := partitionPath(d.Path)
	if err := waitDevice(ctx, part); err != nil {
		return err
	}
	args := mkfsArgs(s, part, label)
	return run(exec.CommandContext(ctx, args[0], args[1:]...))
}

func run(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v\n%s", strings.Join(cmd.Args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// waitDevice waits until udev has created the device node of the new partition.
func waitDevice(ctx context.Context, path string) error {
	exec.CommandContext(ctx, "udevadm", "settle").Run() // optional: not every system has udev
	deadline := time.Now().Add(partitionWait)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("partition %s did not appear", path)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
