package shelf

import (
	"slices"
	"testing"
)

func TestFolderSortKeyParsingAndPreservingMembers(t *testing.T) {
	for _, raw := range []string{`0`, `-2.5`, `4`} {
		meta, err := folderMetaFromData([]byte(`{"sort_key":` + raw + `}`))
		if err != nil || !meta.HasSortKey {
			t.Fatalf("%s: %+v, %v", raw, meta, err)
		}
	}
	for _, raw := range []string{`null`, `"1"`, `true`, `[]`, `{}`} {
		meta, err := folderMetaFromData([]byte(`{"sort_key":` + raw + `}`))
		if err != nil || meta.HasSortKey {
			t.Fatalf("%s: %+v, %v", raw, meta, err)
		}
	}
	s, root := newFolderMetaShelf(t, FolderPath{"A"})
	writeFolderMeta(t, root, `{"icon":"moon","future":true,"sort_key":"bad"}`, "A")
	meta := mustReadFolderMeta(t, s, FolderPath{"A"})
	if err := s.SetFolderSortKey(FolderPath{"A"}, 0, meta.Version); err != nil {
		t.Fatal(err)
	}
	assertFolderMetaMembers(t, root, map[string]string{"schema_version": "1", "icon": `"moon"`, "future": "true", "sort_key": "0"}, "A")
	if err := s.SetFolderSortKey(FolderPath{"A"}, 1, meta.Version); err == nil {
		t.Fatal("stale write accepted")
	}
}

func TestFolderSortPreservesSubtreesAndFallback(t *testing.T) {
	paths := []FolderPath{{}, {"a"}, {"a", "child"}, {"b"}, {"c"}, {"d"}}
	original := slices.Clone(paths)
	sortFolderSiblings(paths, nil)
	if !slices.EqualFunc(paths, original, func(a, b FolderPath) bool { return a.Equal(b) }) {
		t.Fatal(paths)
	}
	sortFolderSiblings(paths, map[string]FolderMeta{"b": {SortKey: 0, HasSortKey: true}, "c": {SortKey: -1, HasSortKey: true}, "d": {SortKey: 0, HasSortKey: true}, "a/child": {SortKey: -100, HasSortKey: true}})
	got := []string{}
	for _, p := range paths {
		got = append(got, p.String())
	}
	if !slices.Equal(got, []string{"", "c", "b", "d", "a", "a/child"}) {
		t.Fatal(got)
	}
}

func TestFolderSortKeySurvivesMoveAndCacheExport(t *testing.T) {
	s, root := newCachedFolderMetaShelf(t, ShelfConf{BookCacheWriterID: testWriterID}, FolderPath{"A"}, FolderPath{"B"})
	if err := s.SetFolderSortKey(FolderPath{"A"}, -1, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveFolder(FolderPath{"A"}, FolderPath{"B"}); err != nil {
		t.Fatal(err)
	}
	meta := mustReadFolderMeta(t, s, FolderPath{"B", "A"})
	if !meta.HasSortKey || meta.SortKey != -1 {
		t.Fatal(meta)
	}
	if _, err := s.ExportBookCache(); err != nil {
		t.Fatal(err)
	}
	remote, _ := openRemoteShelf(t, root, ShelfConf{})
	metas := mustListFolderMeta(t, remote)
	if !metas["B/A"].HasSortKey || metas["B/A"].SortKey != -1 {
		t.Fatal(metas)
	}
}
