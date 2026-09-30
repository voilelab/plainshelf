# Releasing

A release is a pull request into `release/<major>.x`: `release/0.x` for every
`v0.*` tag, `release/1.x` from `v1.0.0-rc1` on. Two workflows drive it, and the
tag is created on the PR's merge commit, never pushed by hand.

| Step | Who | What happens |
|---|---|---|
| 1 | you | Fill `[Unreleased]` in `CHANGELOG.md` on `dev` (the `update-changelog` skill) |
| 2 | you | Run **Prepare release** from the Actions tab with the tag, e.g. `v0.11.0` |
| 3 | `prepare-release.yml` | Opens the release PR `release-prep/<tag>` → `release/<major>.x` |
| 4 | you | Review and merge the release PR with a merge commit |
| 5 | `publish-release.yml` | Builds, tags the merge commit, publishes the Release and Docker image, opens a back-merge PR to `dev` |
| 6 | you | Merge the back-merge PR with a merge commit |
| 7 | you | For a stable release, update the Homebrew casks with `scripts/update-cask.sh <tag>` |

## What Prepare release checks

It stops before creating anything when:

- the tag is not `vMAJOR.MINOR.PATCH[-prerelease]`, already exists, or is not
  newer than the latest release tag (`scripts/resolve-version.sh validate-next`);
- `ci.yml` on `dev`'s current commit, or the latest completed nightly on `dev`,
  is not green;
- `[Unreleased]` has no entries, or a `release-prep/<tag>` branch already exists.

The release PR is `dev`'s current commit plus one commit that moves
`[Unreleased]` under `## [<tag>] - <date>` (`scripts/release-changelog.sh cut`).
Because a bot opens it, no CI runs on the PR itself; the gates above are what
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
2. creates the tag and the GitHub Release on that commit, with the tag's
   CHANGELOG section as the notes and `SHA256SUMS` attached; a tag with a `-`
   is marked as a prerelease;
3. pushes the Docker image as `ghcr.io/voilelab/plainshelf:<tag>`, plus
   `latest` for a stable tag.

Last, it pushes `release-sync/<tag>` at the merge commit and opens a PR from it
into `dev`, which brings the CHANGELOG cut back so the next release PR does not
conflict. If `CHANGELOG.md` changed on `dev` while the release PR was open, the
PR says so: Git can merge those new entries into the released section without a
conflict, so check they stayed under `[Unreleased]`. That PR is also opened by a
bot and starts no CI; close and reopen it if `dev` requires checks.

A manual run of **Release** (`release.yml`) builds every artifact from the chosen
ref and publishes nothing.

## One-time setup

- **Organization and repository settings → Actions → General:** enable *Allow
  GitHub Actions to create and approve pull requests*. The organization setting
  must be on before the repository one can be. This also lets any workflow that
  is granted `pull-requests: write` approve a PR.
- **Ruleset on `release/*`:** require a pull request, allow only the *merge*
  merge method, and block deletion. Do not restrict creation: Prepare release
  creates a new `release/<major>.x` itself.

## When something fails

- **Prepare release** pushes nothing until every check has passed. If it fails
  after pushing `release-prep/<tag>`, delete that branch and run it again.
- **Publish release** can be re-run from the failed job. The Release step updates
  an existing Release rather than failing on it, but a re-run of the whole
  workflow stops at the tag check once the tag exists; re-run failed jobs only.
