package repocheck

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

const androidResXML = "frontend/android/app/src/main/res/xml"

var tokenPrefExcludes = []backupRule{
	{Domain: "sharedpref", Path: "CapacitorStorage.xml"},
	{Domain: "sharedpref", Path: "plainshelf_secure_storage.xml"},
}

var localShelvesExclude = backupRule{Domain: "file", Path: "local-shelves/"}

type backupRule struct {
	Domain string `xml:"domain,attr"`
	Path   string `xml:"path,attr"`
}

type backupSection struct {
	Include []backupRule `xml:"include"`
	Exclude []backupRule `xml:"exclude"`
}

func readBackupXML(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), androidResXML, name))
	if err != nil {
		t.Fatalf("Failed to read %s: %v", name, err)
	}
	if err := xml.Unmarshal(data, v); err != nil {
		t.Fatalf("Failed to parse %s: %v", name, err)
	}
}

func TestAndroidBackupRules(t *testing.T) {
	var full backupSection
	readBackupXML(t, "backup_rules.xml", &full)
	var extraction struct {
		Cloud    backupSection `xml:"cloud-backup"`
		Transfer backupSection `xml:"device-transfer"`
	}
	readBackupXML(t, "data_extraction_rules.xml", &extraction)

	for name, s := range map[string]backupSection{
		"full-backup-content": full,
		"cloud-backup":        extraction.Cloud,
		"device-transfer":     extraction.Transfer,
	} {
		if len(s.Include) != 0 {
			t.Errorf("%s has include rules %v; an include list drops everything else", name, s.Include)
		}
		for _, r := range tokenPrefExcludes {
			if !slices.Contains(s.Exclude, r) {
				t.Errorf("%s does not exclude %+v", name, r)
			}
		}
	}
	if !slices.Contains(extraction.Cloud.Exclude, localShelvesExclude) {
		t.Errorf("cloud-backup does not exclude %+v", localShelvesExclude)
	}
	if slices.Contains(extraction.Transfer.Exclude, localShelvesExclude) {
		t.Errorf("device-transfer excludes %+v; transfer should carry local shelves", localShelvesExclude)
	}
	// Android 11 and lower only read full-backup-content, which goes to the cloud.
	if !slices.Equal(full.Exclude, extraction.Cloud.Exclude) {
		t.Errorf("full-backup-content excludes %v, cloud-backup %v; they must match", full.Exclude, extraction.Cloud.Exclude)
	}
}

func TestAndroidShelvesDirMatchesBackupRule(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t),
		"frontend/android/app/src/main/java/lab/voile/plainshelf/LocalShelf.java"))
	if err != nil {
		t.Fatalf("Failed to read LocalShelf.java: %v", err)
	}
	m := regexp.MustCompile(`new File\(context\.getFilesDir\(\), "([^"]+)"\)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("LocalShelf.java no longer builds its data dir from getFilesDir()")
	}
	if got := string(m[1]) + "/"; got != localShelvesExclude.Path {
		t.Errorf("LocalShelf.java uses %q, backup rules exclude %q", got, localShelvesExclude.Path)
	}
}
