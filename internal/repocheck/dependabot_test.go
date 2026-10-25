package repocheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var ciJobNames = map[string]string{
	"frontend":    "Frontend build",
	"lint":        "Go lint",
	"go":          "Go tests",
	"e2e":         "Frontend E2E",
	"wasm-demo":   "Wasm demo smoke",
	"docs":        "Docs build",
	"android":     "Android build",
	"android-lib": "Android Go library",
	"vulncheck":   "Go vulnerability scan",
}

var blockingCIJobs = map[string]bool{
	"frontend":    true,
	"lint":        true,
	"go":          true,
	"e2e":         true,
	"wasm-demo":   true,
	"docs":        true,
	"android":     false,
	"android-lib": false,
	"vulncheck":   true,
}

// versionUpdatedEcosystem keeps Dependabot version updates: security updates
// never cover it, as GitHub's advisory database has no Docker ecosystem.
const versionUpdatedEcosystem = "docker"

type dependabotConfig struct {
	Version int `yaml:"version"`
	Updates []struct {
		Ecosystem   string   `yaml:"package-ecosystem"`
		Directory   string   `yaml:"directory"`
		Directories []string `yaml:"directories"`
		Limit       *int     `yaml:"open-pull-requests-limit"`
	} `yaml:"updates"`
}

type ciWorkflow struct {
	Jobs map[string]struct {
		Name            string `yaml:"name"`
		ContinueOnError bool   `yaml:"continue-on-error"`
		Steps           []struct {
			Name string            `yaml:"name"`
			Run  string            `yaml:"run"`
			If   string            `yaml:"if"`
			Env  map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func TestDependabotCoversEveryManifest(t *testing.T) {
	root := repoRoot(t)

	covered := map[string]bool{}
	for _, update := range readDependabot(t, root).Updates {
		switch {
		case update.Limit == nil:
			t.Errorf("Dependabot entry for %s has no open-pull-requests-limit; unbounded updates flood dev",
				update.Ecosystem)
		case update.Ecosystem != versionUpdatedEcosystem && *update.Limit != 0:
			t.Errorf("Dependabot entry for %s must set open-pull-requests-limit: 0; only security updates open PRs",
				update.Ecosystem)
		}
		dirs := update.Directories
		if update.Directory != "" {
			dirs = append(dirs, update.Directory)
		}
		for _, dir := range dirs {
			covered[update.Ecosystem+" "+dir] = true
		}
	}

	for _, want := range discoverManifests(t, root) {
		if !covered[want.dependabotEcosystem+" "+want.dependabotDirectory] {
			t.Errorf("%s is not covered by .github/dependabot.yml; add a %q entry for %q",
				want.sourcePath, want.dependabotEcosystem, want.dependabotDirectory)
		}
	}
}

func TestCIJobNamesAreStable(t *testing.T) {
	jobs := readCIWorkflow(t, repoRoot(t)).Jobs

	for id, job := range jobs {
		want, known := ciJobNames[id]
		if !known {
			t.Errorf("ci.yml job %q is new; add it to ciJobNames and blockingCIJobs", id)
			continue
		}
		if job.Name != want {
			t.Errorf("ci.yml job %q is named %q, not %q; a rename drops it from branch protection silently",
				id, job.Name, want)
		}
		if blocking, ok := blockingCIJobs[id]; ok && blocking == job.ContinueOnError {
			t.Errorf("ci.yml job %q has continue-on-error=%v, which contradicts blockingCIJobs",
				id, job.ContinueOnError)
		}
	}
	for id := range ciJobNames {
		if _, ok := jobs[id]; !ok {
			t.Errorf("ci.yml no longer defines job %q; drop it from ciJobNames and blockingCIJobs", id)
		}
	}
}

// releasedGOOS are the operating systems .github/workflows/release.yml builds
// for: linux for the server binaries and the Docker image, darwin for those
// plus the two Wails apps. govulncheck resolves build tags for the GOOS it runs
// as, so scanning only one of them leaves the other's shipped binaries unread.
var releasedGOOS = []string{"linux", "darwin"}

func TestVulnerabilityScanCoversEveryGoModule(t *testing.T) {
	root := repoRoot(t)

	scanned := map[string]bool{}
	for _, step := range readCIWorkflow(t, root).Jobs["vulncheck"].Steps {
		run := strings.TrimSpace(step.Run)
		if !strings.Contains(run, "govulncheck ./...") {
			continue
		}
		dir := "/"
		if rest, ok := strings.CutPrefix(run, "cd "); ok {
			dir = "/" + strings.TrimSpace(strings.SplitN(rest, "&&", 2)[0])
		}
		goos := step.Env["GOOS"]
		if goos == "" {
			t.Errorf("ci.yml step %q does not set GOOS, so it silently scans only the runner's platform", step.Name)
		}
		scanned[dir+" "+goos] = true
	}

	for _, want := range discoverManifests(t, root) {
		if want.dependabotEcosystem != "gomod" {
			continue
		}
		for _, goos := range releasedGOOS {
			if !scanned[want.dependabotDirectory+" "+goos] {
				t.Errorf("ci.yml's vulncheck job does not scan %q (from %s) as GOOS=%s",
					want.dependabotDirectory, want.sourcePath, goos)
			}
		}
	}
}

// TestScanStepsSurviveAnEarlierFailure guards a trap the scan job sits in: a
// failed step skips the rest of its job by default, so the first vulnerable
// module would mask every one scanned after it. Job-level
// continue-on-error does not help — it keeps the failure off the workflow
// result without un-skipping anything.
func TestScanStepsSurviveAnEarlierFailure(t *testing.T) {
	wf := readCIWorkflow(t, repoRoot(t))

	for _, jobID := range []string{"vulncheck"} {
		seenFallible := false
		for _, step := range wf.Jobs[jobID].Steps {
			run := step.Run
			if !strings.Contains(run, "govulncheck") {
				continue
			}
			if seenFallible && !strings.Contains(step.If, "cancelled") {
				t.Errorf("ci.yml step %q (job %q) has no `if` surviving an earlier failure, so a finding before it hides it",
					step.Name, jobID)
			}
			seenFallible = true
		}
		if !seenFallible {
			t.Errorf("ci.yml job %q runs no scan step at all", jobID)
		}
	}
}

type manifest struct {
	sourcePath          string
	dependabotEcosystem string
	dependabotDirectory string
}

func discoverManifests(t *testing.T, root string) []manifest {
	t.Helper()

	var found []manifest
	add := func(rel, ecosystem, dir string) {
		found = append(found, manifest{sourcePath: rel, dependabotEcosystem: ecosystem, dependabotDirectory: dir})
	}

	err := filepath.WalkDir(root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, pth)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skippedDirs[d.Name()] || d.Name() == "android" {
				return filepath.SkipDir
			}
			return nil
		}
		dir := "/" + strings.TrimSuffix(filepath.ToSlash(filepath.Dir(rel)), ".")
		switch d.Name() {
		case "go.mod":
			add(rel, "gomod", dir)
		case "package-lock.json":
			add(rel, "npm", dir)
		case "Dockerfile":
			add(rel, "docker", dir)
		case "requirements.txt":
			add(rel, "pip", dir)
		}
		if strings.HasPrefix(rel, ".github/workflows/") {
			add(rel, "github-actions", "/")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk the repository: %v", err)
	}

	sort.Slice(found, func(i, j int) bool { return found[i].sourcePath < found[j].sourcePath })
	return found
}

func readDependabot(t *testing.T, root string) dependabotConfig {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatalf("Failed to read .github/dependabot.yml: %v", err)
	}
	var cfg dependabotConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("Failed to parse .github/dependabot.yml: %v", err)
	}
	if cfg.Version != 2 {
		t.Fatalf("Dependabot config version is %d; GitHub only reads version 2", cfg.Version)
	}
	return cfg
}

func readCIWorkflow(t *testing.T, root string) ciWorkflow {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("Failed to read .github/workflows/ci.yml: %v", err)
	}
	var wf ciWorkflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("Failed to parse .github/workflows/ci.yml: %v", err)
	}
	return wf
}
