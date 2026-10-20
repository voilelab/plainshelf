package logutil

import (
	"bytes"
	"io"
	"os"

	"github.com/voilelab/plainshelf/internal/util"
)

const DefaultTailBytes int64 = 256 << 10

const maxLineScanBytes int64 = 64 << 10

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

func lineStartAfter(fp *os.File, start, size int64) int64 {
	limit := min(size-start, maxLineScanBytes)
	buf := make([]byte, limit)

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
