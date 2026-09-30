package logutil

import (
	"bytes"
	"io"
	"os"

	"github.com/voilelab/plainshelf/internal/util"
)

// DefaultTailBytes is how much of a log file is returned when the caller does
// not say. A log file grows without bound between rotations, so reading one
// has to be bounded by default rather than on request.
const DefaultTailBytes int64 = 256 << 10

// maxLineScanBytes bounds the search for the line break that the tail is
// aligned to. A log line longer than this is a malformed line, not a reason to
// read the rest of the file looking for its end.
const maxLineScanBytes int64 = 64 << 10

// SeekTail positions fp at the start of its last maxBytes bytes and returns
// that offset. The offset is moved forward to just past the next line break so
// the read never opens mid-line, which means slightly fewer than maxBytes are
// returned.
//
// maxBytes <= 0 reads the whole file.
func SeekTail(fp *os.File, maxBytes int64) (int64, error) {
	info, err := fp.Stat()
	if err != nil {
		return 0, util.Errorf("%w", err)
	}

	size := info.Size()
	if maxBytes <= 0 || size <= maxBytes {
		return 0, nil
	}

	start := lineStartAfter(fp, size-maxBytes, size)
	if _, err := fp.Seek(start, io.SeekStart); err != nil {
		return 0, util.Errorf("%w", err)
	}
	return start, nil
}

// lineStartAfter returns the offset of the first whole line at or after start.
//
// It falls back to start itself when there is no whole line to move to, which
// is the case for a line longer than the requested tail: returning the end of
// the file would answer a request for the last N bytes with nothing at all.
func lineStartAfter(fp *os.File, start, size int64) int64 {
	limit := min(size-start, maxLineScanBytes)
	buf := make([]byte, limit)

	// A short read is not a failure here: whatever was read is enough to look
	// for a break in, and finding none leaves the unaligned offset, which is
	// still a correct tail.
	n, _ := fp.ReadAt(buf, start)
	i := bytes.IndexByte(buf[:n], '\n')
	if i < 0 {
		return start
	}

	aligned := start + int64(i) + 1
	if aligned >= size {
		return start
	}
	return aligned
}
