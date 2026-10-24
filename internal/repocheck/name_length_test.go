package repocheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const (
	maxTestNameLen = 50
	maxNameLen     = 28
)

var longNamesBeforeTheCap = map[string]bool{
	"originAllowedForTokenExemptRequest": true,
	"backdateSourcesForFingerprintCache": true,
	"collectBookIDsNeedingContentStats":  true,
	"scheduleBookCacheRefreshIfNeeded":   true,
	"refreshCurrentSourceContentStats":   true,
	"readConformanceCaseThroughReadFS":   true,
	"originAllowedForProtectedRequest":   true,
	"scheduleBookCacheExportIfNeeded":    true,
	"forgetAppliedExportDigestLocked":    true,
	"normalizeDesktopShelfDirectory":     true,
	"mirrorSourceFormatIntoBookMeta":     true,
	"writeDownloadsUnderDownloadMu":      true,
	"collectExportFolderMetaLocked":      true,
	"ValidateSecurityForListenAddr":      true,
	"readerUnsupportedPlatformCode":      true,
	"desktopLegacyDefaultShelfName":      true,
	"cjkSymbolsAndPunctuationStart":      true,
	"halfwidthAndFullwidthFormsStart":    true,
	"halfwidthAndFullwidthFormsEnd":      true,
	"resultRecursiveListingUnsupported":  true,
	"ErrUnsupportedBookSchemaVersion":    true,
	"ErrUnsupportedSourceSchemaVersion":  true,
	"ErrUnsupportedTrashSchemaVersion":   true,
}

func TestDeclaredNamesStayShort(t *testing.T) {
	root := repoRoot(t)

	fset := token.NewFileSet()
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(pth, ".go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, pth, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		for _, ident := range declaredNames(file) {
			name := ident.Name
			limit := maxNameLen
			if isTestEntryPoint(name) {
				limit = maxTestNameLen
			}
			if len(name) <= limit {
				continue
			}
			seen[name] = true
			if !longNamesBeforeTheCap[name] {
				t.Errorf("%s: %s is %d characters, over %d; name the subject and the case, and leave the reason to a test", fset.Position(ident.Pos()), name, len(name), limit)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the repository: %v", err)
	}
	for name := range longNamesBeforeTheCap {
		if !seen[name] {
			t.Errorf("%s is gone or now short enough; delete it from longNamesBeforeTheCap", name)
		}
	}
}

func declaredNames(file *ast.File) []*ast.Ident {
	var names []*ast.Ident
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			names = append(names, decl.Name)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, spec.Name)
				case *ast.ValueSpec:
					names = append(names, spec.Names...)
				}
			}
		}
	}
	return names
}

func isTestEntryPoint(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
