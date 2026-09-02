# Security Policy

PlainShelf is a local-first personal reading library. It is designed for single-user use on trusted devices and networks, and its default development configuration binds the web server to localhost.

## Supported Versions

PlainShelf is currently in pre-alpha development. Security fixes are provided for the current `major.minor` release line and the `main` branch. Maintainers do not backport security fixes to earlier `major.minor` release lines.

| Version | Supported |
| ------- | --------- |
| Current `major.minor` release line | Yes |
| `main`  | Yes       |
| Earlier `major.minor` release lines, commits, forks, or experimental builds | No guarantee |

## Reporting a Vulnerability

Please do not open a public issue with exploit details, private library contents, paths from your machine, or any proof-of-concept that could be used directly against other users.

Preferred reporting flow:

1. Use GitHub's private vulnerability reporting or open a draft security advisory for this repository: <https://github.com/voilelab/plainshelf/security/advisories/new>.
2. If private reporting is not available, open a public issue that only says you have a security concern and ask the maintainers to arrange a private contact channel. Do not include sensitive details in that issue.

When reporting, please include as much of the following as you can safely share:

- Affected commit, branch, or build method.
- Operating system and whether you used the server, or Docker image.
- Configuration details relevant to the issue, especially bind address, reverse proxy, mounted directories, and shelf/store paths.
- Steps to reproduce with minimal test data.
- Expected impact, such as local file disclosure, data loss, cross-site scripting, request forgery, or unintended network exposure.

## What to Expect

PlainShelf is maintained on a best-effort basis. For credible reports, maintainers aim to:

- Acknowledge the report within 7 days.
- Confirm scope and severity once reproduction is understood.
- Prioritize fixes that protect local user data, prevent remote access in common deployments, or avoid irreversible data loss.
- Credit reporters in release notes or commit messages if they want public credit.

Please allow maintainers reasonable time to investigate and prepare a fix before public disclosure.

## Security Scope

In scope:

- Vulnerabilities in PlainShelf server routes, file handling, import logic, metadata parsing, frontend rendering, Docker defaults, or configuration examples.
- Issues that can expose, overwrite, corrupt, or delete files in the configured shelf/store directories.
- Issues that make the local web UI reachable or usable in ways that contradict documented defaults.
- Cross-site scripting or browser-side issues that can affect a PlainShelf user from untrusted book metadata or imported content.

Out of scope unless they demonstrate a concrete PlainShelf impact:

- Vulnerabilities that require full local account compromise before PlainShelf runs.
- Findings against unsupported forks or heavily modified builds.
- Denial-of-service reports based only on very large user-supplied libraries or files.
- Reports about missing multi-user authentication for deployments intentionally exposed to untrusted networks. PlainShelf is not currently designed as a public, multi-user service.

## Secure Usage Guidance

How to bind and configure PlainShelf for each deployment — loopback single user, home LAN / NAS, and why the public internet is not yet supported — is documented in one place so the guidance does not drift: [Deployment and threat model](docs/deployment-and-threat-model.md). That page also states two standing boundaries: `local_token` is a CSRF mitigation, not access control, and no PlainShelf server holds your credentials (a pCloud token you grant stays on your own device).

The essentials, at every tier:

- Follow [Deployment and threat model](docs/deployment-and-threat-model.md) for the bind address, `app_conf.security.mode`, and origin allowlist your deployment needs. Do not expose PlainShelf directly to the public internet: it has no login yet.
- Keep shelf and store directories backed up before testing new builds.
- Treat imported books, metadata, and covers as untrusted input.
- Review Docker volume mounts and custom configuration files before sharing them.

## Dependency Updates

Security updates for Go, npm, Docker base images, and other dependencies should be handled promptly when they affect PlainShelf. Two mechanisms back that up rather than leaving it to a maintainer remembering to look:

- **Detection.** Every pull request runs `govulncheck` over all three Go modules. It reports an advisory only when a vulnerable symbol is actually reachable from PlainShelf's code, so it is a merge gate. A second job audits the `frontend` and `e2e` npm lockfiles with `npm audit --audit-level=high`; that job is informational and does not block a merge, because `npm audit` flags advisories anywhere in the dependency tree without checking reachability, and most of that tree is build-time tooling that never ships.
- **Updates.** [Dependabot](.github/dependabot.yml) opens weekly grouped pull requests for the three Go modules, both npm lockfiles, the Dockerfile base images, the documentation build requirements, and the GitHub Actions workflows.

Neither mechanism covers everything: the Go and Node versions pinned as build arguments in the `Dockerfile` are not visible to Dependabot and are still updated by hand.

Reports that identify vulnerable dependencies are most helpful when they include the affected package, installed version, fixed version, and whether the vulnerable code path is reachable in PlainShelf.
