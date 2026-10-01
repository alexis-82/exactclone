// Package rawdev opens physical disks for raw sector-level reading and writing.
package rawdev

import "io"

// Device is a raw block device (or, in tests, a regular file).
type Device interface {
	io.ReaderAt
	io.WriterAt
	// Size returns the device size in bytes.
	Size() int64
	// DropCache flushes pending writes and evicts cached pages so that the
	// next read hits the physical medium (used before verification).
	DropCache() error
	io.Closer
}
