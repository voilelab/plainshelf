package bookpkg

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/util"
)

const (
	PageDirectionLTR = "ltr"
	PageDirectionRTL = "rtl"
)

const maxPageListBytes = 1 << 20

var ErrNotImageSource = util.NewError("source is not an images source")

var ErrInvalidPageList = util.NewError("invalid page list")

var ErrPageListConflict = util.NewError("page list changed since it was read")

var ErrInvalidPagePosition = util.NewError("page position is outside the page list")

var ErrInvalidPageDirection = util.NewError(`page_direction must be "ltr" or "rtl", and "rtl" only on an images source`)

type Page struct {
	Name    string `json:"name"`
	Missing bool   `json:"missing"`
}

type PageList struct {
	Pages    []Page   `json:"pages"`
	Unlisted []string `json:"unlisted"`
	MD5Hash  string   `json:"md5_hash"`
}

func (r *Source) HoldsText() bool {
	return r.meta.Format != BookFormatImages
}

func (r *Source) Pages() (*PageList, error) {
	if r.meta.Format != BookFormatImages {
		return nil, util.Errorf("%w", ErrNotImageSource)
	}

	content, err := r.readPageList()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	md5Hash, err := hashutil.MD5Hash(bytes.NewReader(content))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	assets, err := r.ListAssets()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	present := make(map[string]bool, len(assets))
	for _, name := range assets {
		present[name] = true
	}

	names := parsePageNames(content)
	listed := make(map[string]bool, len(names))
	list := &PageList{Pages: make([]Page, 0, len(names)), Unlisted: []string{}, MD5Hash: md5Hash}
	for _, name := range names {
		list.Pages = append(list.Pages, Page{Name: name, Missing: !present[name]})
		listed[name] = true
	}
	for _, name := range assets {
		if !listed[name] {
			list.Unlisted = append(list.Unlisted, name)
		}
	}
	return list, nil
}

func (r *Source) readPageList() ([]byte, error) {
	content, err := r.ReadContent()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, util.Errorf("%w", err)
	}
	return content, nil
}

func (r *Source) editablePageNames(baseMD5 string) ([]string, error) {
	if r.meta.Format != BookFormatImages {
		return nil, util.Errorf("%w", ErrNotImageSource)
	}
	if err := r.EnsureWritable(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	content, err := r.readPageList()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	md5Hash, err := hashutil.MD5Hash(bytes.NewReader(content))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if md5Hash != baseMD5 {
		return nil, util.Errorf("%w: source.txt is %s, the edit was based on %s", ErrPageListConflict, md5Hash, baseMD5)
	}
	return parsePageNames(content), nil
}

func (r *Source) writePageNames(names []string) (*PageList, error) {
	content := ""
	if len(names) > 0 {
		content = strings.Join(names, "\n") + "\n"
	}
	if err := r.UpdateContent(strings.NewReader(content)); err != nil {
		return nil, util.Errorf("%w", err)
	}
	return r.Pages()
}

func (r *Source) ReplacePages(baseMD5 string, names []string) (*PageList, error) {
	if _, err := r.editablePageNames(baseMD5); err != nil {
		return nil, util.Errorf("%w", err)
	}
	for i, name := range names {
		if err := validateAssetName(name); err != nil {
			return nil, util.Errorf("%w: page %d: %w", ErrInvalidPageList, i+1, err)
		}
		if strings.TrimSpace(name) != name || strings.ContainsAny(name, "\r\n") {
			return nil, util.Errorf("%w: page %d: %q would not read back as one line", ErrInvalidPageList, i+1, name)
		}
	}
	return r.writePageNames(names)
}

func (r *Source) InsertPage(baseMD5 string, position int, data []byte, ext string) (*PageList, error) {
	names, err := r.editablePageNames(baseMD5)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if position < 0 || position > len(names) {
		return nil, util.Errorf("%w: %d of %d pages", ErrInvalidPagePosition, position, len(names))
	}

	name := rand.Text() + strings.ToLower(ext)
	if err := r.WriteAsset(name, data); err != nil {
		return nil, util.Errorf("%w", err)
	}
	list, err := r.writePageNames(slices.Insert(names, position, name))
	if err != nil {
		if cleanupErr := r.removeUnlistedAsset(name); cleanupErr != nil {
			return nil, util.Errorf("%w (and the new page %s was not cleaned up: %w)", err, name, cleanupErr)
		}
		return nil, util.Errorf("%w", err)
	}
	return list, nil
}

func (r *Source) removeUnlistedAsset(name string) error {
	content, err := r.readPageList()
	if err != nil {
		return util.Errorf("%w", err)
	}
	if slices.Contains(parsePageNames(content), name) {
		return nil
	}
	return r.DeleteAsset(name)
}

func (r *Source) DeletePage(baseMD5, name string) (*PageList, error) {
	names, err := r.editablePageNames(baseMD5)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if err := validateAssetName(name); err != nil {
		return nil, util.Errorf("%w", err)
	}

	kept := slices.DeleteFunc(slices.Clone(names), func(listed string) bool { return listed == name })
	if len(kept) != len(names) {
		if _, err := r.writePageNames(kept); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}
	if err := r.DeleteAsset(name); err != nil {
		if !errors.Is(err, ErrAssetNotFound) || len(kept) == len(names) {
			return nil, util.Errorf("%w", err)
		}
	}
	return r.Pages()
}

func (r *Source) SetPageDirection(direction string) error {
	normalized, err := normalizePageDirection(r.meta.Format, direction)
	if err != nil {
		return util.Errorf("%w", err)
	}
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}
	meta := r.GetMeta()
	meta.PageDirection = normalized
	return r.writebackMeta(root, meta)
}

func parsePageNames(content []byte) []string {
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))
	var names []string
	for line := range strings.SplitSeq(string(content), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func validatePageList(content []byte) error {
	for i, name := range parsePageNames(content) {
		if err := validateAssetName(name); err != nil {
			return util.Errorf("%w: page %d: %w", ErrInvalidPageList, i+1, err)
		}
	}
	return nil
}

func checkedContent(format string, content io.Reader) (io.Reader, error) {
	if format != BookFormatImages {
		return content, nil
	}

	data, err := io.ReadAll(io.LimitReader(content, maxPageListBytes+1))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	if len(data) > maxPageListBytes {
		return nil, util.Errorf("%w: larger than %d bytes", ErrInvalidPageList, maxPageListBytes)
	}
	if err := validatePageList(data); err != nil {
		return nil, util.Errorf("%w", err)
	}
	return bytes.NewReader(data), nil
}

func normalizePageDirection(format, direction string) (string, error) {
	switch direction {
	case "", PageDirectionLTR:
		return "", nil
	case PageDirectionRTL:
		if format == BookFormatImages {
			return PageDirectionRTL, nil
		}
	}
	return "", util.Errorf("%w: got %q for a %q source", ErrInvalidPageDirection, direction, format)
}
