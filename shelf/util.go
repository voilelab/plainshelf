package shelf

import (
	"fmt"
	"io"
	"path"
	"strings"
	"time"
	"uuid"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
	"go.rtnl.ai/x/slugify"
)

const MaxTempDirCreationAttempts = 10

const MaxBookIDCreationAttempts = 10

func createTempDir(root fsutil.FS, prefix string) (string, error) {
	for range MaxTempDirCreationAttempts {
		tmpDirName := fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102-150405"), fsutil.RandomString(6))
		err := root.Mkdir(tmpDirName)
		if err == nil {
			return tmpDirName, nil
		}
	}

	return "", util.NewError("failed to create temp directory after multiple attempts")
}

func copyTreeAcross(srcRoot fsutil.ReadFS, src string, dstRoot fsutil.FS, dst string) error {
	info, err := srcRoot.Stat(src)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if !info.IsDir() {
		return copyFileAcross(srcRoot, src, dstRoot, dst)
	}

	if err := dstRoot.MkdirAll(dst); err != nil {
		return util.Errorf("%w", err)
	}

	entries, err := srcRoot.ReadDir(src)
	if err != nil {
		return util.Errorf("%w", err)
	}

	for _, entry := range entries {
		if err := copyTreeAcross(srcRoot, path.Join(src, entry.Name()), dstRoot, path.Join(dst, entry.Name())); err != nil {
			return util.Errorf("%w", err)
		}
	}

	return nil
}

func copyFileAcross(srcRoot fsutil.ReadFS, src string, dstRoot fsutil.FS, dst string) error {
	in, err := srcRoot.Open(src)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer in.Close()

	out, err := dstRoot.OpenWriter(dst)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return util.Errorf("%w", err)
	}

	if err := out.Close(); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

var ErrInvalidFolder = util.NewError("invalid folder name")

var ErrIgnoredFolderName = util.Errorf("%w: directory name the shelf scanners skip", ErrInvalidFolder)

type IgnoredFolderNameError struct {
	Folder string
	Reason string
}

func (e *IgnoredFolderNameError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("%s %q", ErrIgnoredFolderName, e.Folder)
	}
	return fmt.Sprintf("%s %q: %s", ErrIgnoredFolderName, e.Folder, e.Reason)
}

func (e *IgnoredFolderNameError) Unwrap() error { return ErrIgnoredFolderName }

func validateFolderPath(folders FolderPath) error {
	for _, folder := range folders {
		if err := shelfutil.ValidateFolderSegment(folder); err != nil {
			return util.Errorf("%w %q: %w", ErrInvalidFolder, folder, err)
		}
		if strings.Contains(folder, bookExtension) {
			return util.Errorf("%w %q: must not contain %q", ErrInvalidFolder, folder, bookExtension)
		}
	}
	return nil
}

func newBookID() string {
	return uuid.NewV4().String()
}

func validateBookID(bookID string) error {
	if err := shelfutil.ValidatePathSegment(bookID); err != nil {
		return util.Errorf("invalid book id %q: %w", bookID, err)
	}
	return nil
}

func titleToFolderName(title string) string {
	folderName := strings.ReplaceAll(title, " ", "-")
	return slugify.Slugify(folderName) + bookExtension
}
