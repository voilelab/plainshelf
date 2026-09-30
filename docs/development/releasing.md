# Releasing

A release is a pull request into `release/<major>.x`: `release/0.x` for every
`v0.*` tag, `release/1.x` from `v1.0.0-rc1` on. Two workflows drive it, and the
tag is created on the PR's merge commit, never pushed by hand.

| Step | Who | What happens |
|---|---|---|
| 1 | you | Run **Prepare release** from the Actions tab with the tag, e.g. `v0.11.0` |
| 2 | `prepare-release.yml` | Opens the release PR `release-prep/<tag>` → `release/<major>.x` |
| 3 | you | Review and merge the release PR with a merge commit |
| 4 | `publish-release.yml` | Builds, tags the merge commit, publishes the Release and Docker image |
| 5 | you | For a stable release, update the Homebrew casks with `scripts/update-cask.sh <tag>` |

## What Prepare release checks

It stops before creating anything when:

- the tag is not `vMAJOR.MINOR.PATCH[-prerelease]`, already exists, or is not
  newer than the latest release tag (`scripts/resolve-version.sh validate-next`);
- `ci.yml` on `dev`'s current commit, or the latest completed nightly on `dev`,
  is not green (a CI run still in progress, as right after a merge, is waited
  for);
- a `release-prep/<tag>` branch already exists at another commit.

The release PR's head is `dev`'s current commit, under a branch name that carries
the version. Because a bot opens it, no CI runs on the PR itself; the gates above are what
stand in for it. If `release/<major>.x` does not exist yet, it is created from
the latest release tag, so the first PR into a new branch shows everything since
the previous release.

## What Publish release does

It runs only for a merged PR whose head is `release-prep/*` in this repository,
and fails before building if the PR was not merged with a merge commit, the tag
belongs on a different `release/*` branch, or the tag already exists. It then
calls `release.yml`, which:

1. builds the server tarballs, the macOS desktop and reader apps, and the Docker
   image from the merge commit;
2. creates the tag and the GitHub Release on that commit, with `SHA256SUMS`
   attached; a tag with a `-` is marked as a prerelease. The notes are
   GitHub's list of pull requests merged since the previous release tag by
   SemVer (`scripts/release-notes.sh`), which the release PR's description
   also shows;
3. pushes the Docker image as `ghcr.io/voilelab/plainshelf:<tag>`, plus
   `latest` for a stable tag.

The merge into `release/<major>.x` also deploys the documentation site
(`docs.yml`), so the published docs follow the latest release.

Nothing needs to go back to `dev`: `release/<major>.x` only ever gains merge
commits, so the next release PR merges cleanly as long as each one is merged
with a merge commit.

A manual run of **Release** (`release.yml`) builds every artifact from the chosen
ref and publishes nothing.

## Release notes labels

The notes list pull request titles, grouped by label (`.github/release.yml`), so
a PR's title should state its user-visible effect. Every PR into `dev` needs at
least one of these labels, which the `Release notes label` check enforces:

| Label | Section | Use for |
|---|---|---|
| `breaking` | Breaking changes | anything a user must act on when upgrading: format, API, config, default; say what in the title |
| `security` | Security | security fixes and hardening |
| `feature` | Features | new user-visible capability |
| `fix` | Fixes | bug fixes |
| `dependencies` | Dependencies | dependency updates; Dependabot sets it |
| `internal` | *(left out)* | CI, tests, refactors and docs with no user-visible effect |

A PR in several sections is listed once, under the first one in this table, so
a breaking feature carries both `breaking` and `feature`.

## One-time setup

- **Organization and repository settings → Actions → General:** enable *Allow
  GitHub Actions to create and approve pull requests*. The organization setting
  must be on before the repository one can be. This also lets any workflow that
  is granted `pull-requests: write` approve a PR.
- **Ruleset on `release/*`:** require a pull request, allow only the *merge*
  merge method, and block deletion. Do not restrict creation: Prepare release
  creates a new `release/<major>.x` itself.
- **Labels:** create the six labels above; the check and the release notes
  match their names exactly.
- **Branch protection on `dev`:** make `Release notes label` a required check.
- **Repository settings → Environments → `github-pages`:** if its deployment
  branches are limited, allow `release/*`; the docs site deploys from there.

## When something fails

- **Prepare release** pushes nothing until every check has passed. If it fails
  after pushing `release-prep/<tag>`, delete that branch and run it again.
- **Publish release** can be re-run from the failed job. The Release step updates
  an existing Release rather than failing on it, but a re-run of the whole
  workflow stops at the tag check once the tag exists; re-run failed jobs only.
