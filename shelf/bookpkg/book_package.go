package bookpkg

import (
	"errors"
	"io/fs"
	"os"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

var ErrNotBookPackage = util.NewError("directory is not a book package")

type BookPackage struct {
	root *os.Root
	book *Book
}

func OpenBookPackage(dir string, logger *logutil.Logger) (*BookPackage, error) {
	if logger == nil {
		discard, err := logutil.NewLogger(&logutil.LogConf{
			LogFile: logutil.LogFileConf{Type: logutil.LogFileTypeNone},
		})
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		logger = discard
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	book, err := Open(fsutil.ReadOnly(fsutil.NewRootFS(root)), *logger, ".")
	if err != nil {
		root.Close()
		if errors.Is(err, fs.ErrNotExist) {
			return nil, util.Errorf("%w: %s", ErrNotBookPackage, dir)
		}
		return nil, util.Errorf("%w", err)
	}

	return &BookPackage{root: root, book: book}, nil
}

func (p *BookPackage) Book() *Book {
	return p.book
}

func (p *BookPackage) Close() error {
	if p == nil || p.root == nil {
		return nil
	}

	root := p.root
	p.root = nil
	if err := root.Close(); err != nil {
		return util.Errorf("%w", err)
	}
	return nil
}
