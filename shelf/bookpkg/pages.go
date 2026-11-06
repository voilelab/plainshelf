package bookpkg

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

const (
	PageDirectionLTR = "ltr"
	PageDirectionRTL = "rtl"
)

const maxPageListBytes = 1 << 20

var ErrNotImageSource = util.NewError("source is not an images source")

var ErrInvalidPageList = util.NewError("invalid page list")

var ErrInvalidPageDirection = util.NewError(`page_direction must be "ltr" or "rtl", and "rtl" only on an images source`)

type Page struct {
	Name    string `json:"name"`
	Missing bool   `json:"missing"`
}

type PageList struct {
	Pages    []Page   `json:"pages"`
	Unlisted []string `json:"unlisted"`
}

func (r *Source) HoldsText() bool {
	return r.meta.Format != BookFormatImages
}

func (r *Source) Pages() (*PageList, error) {
	if r.meta.Format != BookFormatImages {
		return nil, util.Errorf("%w", ErrNotImageSource)
	}

	content, err := r.ReadContent()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
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
	list := &PageList{Pages: make([]Page, 0, len(names)), Unlisted: []string{}}
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
