package jsonopt

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

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
