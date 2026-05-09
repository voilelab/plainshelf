package txtlibsrv

import (
	"log"

	"github.com/voilelab/plainshelf/internal/hashutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/txtlib"
	"github.com/voilelab/plainshelf/txtlib-srv/bookindex"
)

func initIndexDBFromLib(indexDB *bookindex.DB, lib *txtlib.Txtlib) error {
	books, err := lib.ListBooks()
	if err != nil {
		return util.Errorf("%w", err)
	}

	for _, book := range books {
		if indexDB.Has(book.ID()) {
			continue
		}

		err = addBookToIndexDB(indexDB, book)
		if err != nil {
			log.Printf("failed to add book %s to index: %v", book.ID(), err)
			continue
		}
	}

	return nil
}

func addBookToIndexDB(indexDB *bookindex.DB, book *txtlib.Book) error {
	snapShot, err := book.GetSnapshot(book.CurrentSnapshot())
	if err != nil {
		return util.Errorf("%w", err)
	}

	reader, err := snapShot.OpenSource()
	if err != nil {
		return util.Errorf("%w", err)
	}

	hash, err := hashutil.MD5Hash(reader)
	if err != nil {
		return util.Errorf("%w", err)
	}

	indexDB.Add(
		book.ID(),
		bookindex.MetaMap{
			"content_hash": hash,
		})
	return nil
}
