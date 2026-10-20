package jsonopt

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// diskIndent is the indentation every hand-editable file on the shelf is
// written with. It is two spaces because that is what the shelf already
// contains, and rewriting every book.json to change it would be a data-format
// change for no benefit.
const diskIndent = "  "

var (
	disk = json.JoinOptions(
		json.Deterministic(true),
		jsontext.WithIndent(diskIndent),
	)

	diskCompact = json.JoinOptions(
		json.Deterministic(true),
	)

	api = json.JoinOptions(
		json.Deterministic(true),
	)
)

func Disk() json.Options { return disk }

func DiskCompact() json.Options { return diskCompact }

func API() json.Options { return api }
