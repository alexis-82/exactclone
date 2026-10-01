package clone

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	DefaultBlockSize = 16 << 20 // 16 MiB
	DefaultHeadSize  = 1 << 20  // 1 MiB: covers MBR and primary GPT
	alignment        = 4096     // buffer alignment required by unbuffered I/O on Windows
	numBuffers       = 4
)

var (
	// ErrCanceled is returned when the context is canceled during an operation.
	ErrCanceled = errors.New("operation canceled")
	// ErrVerifyMismatch is returned when the destination hash differs from the source.
	ErrVerifyMismatch = errors.New("verification failed: destination differs from source")
)

// Options tunes a copy. Zero values select the defaults.
type Options struct {
	BlockSize int
	// HeadLast writes the first HeadSize bytes (partition table) after
	// everything else, so the OS does not discover and mount the new
	// partitions while the copy is still running.
	HeadLast bool
	HeadSize int
}

func (o Options) withDefaults() Options {
	if o.BlockSize <= 0 {
		o.BlockSize = DefaultBlockSize
	}
	if o.HeadSize <= 0 {
		o.HeadSize = DefaultHeadSize
	}
	if o.HeadSize > o.BlockSize {
		o.HeadSize = o.BlockSize
	}
	return o
}

// Verifiable is a destination that can be re-read from the physical medium.
type Verifiable interface {
	io.ReaderAt
	DropCache() error
}

type chunk struct {
	buf  []byte
	off  int64
	refs atomic.Int32
}

// alignedBuf returns a slice of length size whose first byte is aligned to alignment.
func alignedBuf(size int) []byte {
	raw := make([]byte, size+alignment)
	shift := 0
	if r := int(uintptr(unsafe.Pointer(&raw[0])) % alignment); r != 0 {
		shift = alignment - r
	}
	return raw[shift : shift+size : shift+size]
}

// pipeline runs a sequential reader that hands each chunk to the given consumers.
// Consumers are called in order from their own goroutine; the buffer is recycled
// once every consumer has returned.
type pipeline struct {
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
	err    error
	pool   chan []byte
}

func newPipeline(parent context.Context, blockSize int) *pipeline {
	ctx, cancel := context.WithCancel(parent)
	p := &pipeline{ctx: ctx, cancel: cancel, pool: make(chan []byte, numBuffers)}
	for i := 0; i < numBuffers; i++ {
		p.pool <- alignedBuf(blockSize)
	}
	return p
}

func (p *pipeline) fail(err error) {
	p.once.Do(func() { p.err = err; p.cancel() })
}

func (p *pipeline) release(c *chunk) {
	if c.refs.Add(-1) == 0 {
		p.pool <- c.buf[:cap(c.buf)]
	}
}

// run reads src[0:size) sequentially (first chunk of length first, then
// blockSize) and feeds every consumer. It returns when all chunks are consumed
// or the pipeline fails.
func (p *pipeline) run(src io.ReaderAt, size int64, first, blockSize int, consumers ...func(*chunk) error) {
	chans := make([]chan *chunk, len(consumers))
	var wg sync.WaitGroup
	for i, consume := range consumers {
		chans[i] = make(chan *chunk, numBuffers)
		wg.Add(1)
		go func(ch chan *chunk, consume func(*chunk) error) {
			defer wg.Done()
			for c := range ch {
				if p.ctx.Err() == nil {
					if err := consume(c); err != nil {
						p.fail(err)
					}
				}
				p.release(c)
			}
		}(chans[i], consume)
	}

	func() {
		defer func() {
			for _, ch := range chans {
				close(ch)
			}
		}()
		n := first
		for off := int64(0); off < size; off += int64(n) {
			var buf []byte
			select {
			case buf = <-p.pool:
			case <-p.ctx.Done():
				return
			}
			if p.ctx.Err() != nil {
				return
			}
			if off > 0 {
				n = blockSize
			}
			if rem := size - off; int64(n) > rem {
				n = int(rem)
			}
			buf = buf[:n]
			m, err := src.ReadAt(buf, off)
			if m < n {
				if err == nil || err == io.EOF {
					err = io.ErrUnexpectedEOF
				}
				p.fail(fmt.Errorf("read at offset %d: %w", off, err))
				return
			}
			c := &chunk{buf: buf, off: off}
			c.refs.Store(int32(len(chans)))
			for _, ch := range chans {
				select {
				case ch <- c:
				case <-p.ctx.Done():
					return
				}
			}
		}
	}()
	wg.Wait()
}

// result maps the pipeline outcome: parent cancellation wins over other errors.
func (p *pipeline) result(parent context.Context) error {
	if parent.Err() != nil {
		return ErrCanceled
	}
	return p.err
}

// Copy copies the first size bytes of src to dst and returns the SHA-256 of
// the source data. onProgress (optional) receives the number of bytes written.
func Copy(ctx context.Context, src io.ReaderAt, dst io.WriterAt, size int64, opts Options, onProgress func(int64)) ([]byte, error) {
	opts = opts.withDefaults()
	p := newPipeline(ctx, opts.BlockSize)
	defer p.cancel()

	first := opts.BlockSize
	if opts.HeadLast {
		first = opts.HeadSize
	}
	h := sha256.New()
	var head []byte

	write := func(c *chunk) error {
		if opts.HeadLast && c.off == 0 {
			head = alignedBuf(len(c.buf))
			copy(head, c.buf)
			return nil
		}
		if _, err := dst.WriteAt(c.buf, c.off); err != nil {
			return fmt.Errorf("write at offset %d: %w", c.off, err)
		}
		if onProgress != nil {
			onProgress(int64(len(c.buf)))
		}
		return nil
	}
	hash := func(c *chunk) error {
		h.Write(c.buf)
		return nil
	}
	p.run(src, size, first, opts.BlockSize, write, hash)

	if err := p.result(ctx); err != nil {
		return nil, err
	}
	if head != nil {
		if _, err := dst.WriteAt(head, 0); err != nil {
			return nil, fmt.Errorf("write at offset 0: %w", err)
		}
		if onProgress != nil {
			onProgress(int64(len(head)))
		}
	}
	return h.Sum(nil), nil
}

// Verify re-reads the first size bytes of dst from the physical medium and
// compares their SHA-256 with expected.
func Verify(ctx context.Context, dst Verifiable, size int64, expected []byte, onProgress func(int64)) error {
	if err := dst.DropCache(); err != nil {
		return fmt.Errorf("drop cache: %w", err)
	}
	p := newPipeline(ctx, DefaultBlockSize)
	defer p.cancel()
	h := sha256.New()
	p.run(dst, size, DefaultBlockSize, DefaultBlockSize, func(c *chunk) error {
		h.Write(c.buf)
		if onProgress != nil {
			onProgress(int64(len(c.buf)))
		}
		return nil
	})
	if err := p.result(ctx); err != nil {
		return err
	}
	if !bytes.Equal(h.Sum(nil), expected) {
		return ErrVerifyMismatch
	}
	return nil
}
