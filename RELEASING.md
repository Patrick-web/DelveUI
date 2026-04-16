# Releasing DelveUI

## Prerequisites

- Push access to `Patrick-web/DelveUI`
- All changes committed and pushed to `main`

## Release process

### 1. Decide the version

Follow [semver](https://semver.org/):
- **Patch** (v0.1.1): bug fixes, minor tweaks
- **Minor** (v0.2.0): new features, non-breaking changes
- **Major** (v1.0.0): breaking changes, major milestones

### 2. Tag and push

```bash
git tag v0.2.0
git push origin v0.2.0
```

This triggers the GitHub Actions release workflow (`.github/workflows/release.yml`).

### 3. What happens automatically

The CI pipeline:

1. **Builds on 3 platforms in parallel:**
   - **macOS** (`macos-latest`): compiles Go binary → creates `.app` bundle → signs → packages as `.zip` + `.dmg`
   - **Linux** (`ubuntu-latest`): compiles Go binary → packages as `.tar.gz`
   - **Windows** (`windows-latest`): compiles Go binary → packages as `.zip`

2. **Sets the version** via `-ldflags "-X main.version=v0.2.0"` so the app knows its version at runtime.

3. **Creates a GitHub Release** with:
   - Auto-generated release notes (categorized by PR labels)
   - All 4 artifacts uploaded: `.dmg`, `.zip` (macOS), `.tar.gz` (Linux), `.zip` (Windows)

### 4. Verify

- Check the Actions tab: https://github.com/Patrick-web/DelveUI/actions
- Check the release: https://github.com/Patrick-web/DelveUI/releases
- Download and test at least one artifact

### 5. Auto-updates

Users on older versions will see the new release automatically — the app checks GitHub Releases 30 seconds after launch via `go-github-selfupdate`.

## Release notes categorization

PRs are grouped in release notes by label (configured in `.github/release.yml`):

| Label | Category |
|---|---|
| `enhancement`, `feature` | 🚀 Features |
| `bug`, `fix` | 🐛 Bug Fixes |
| `ui`, `ux`, `design` | 🎨 UI/UX |
| `performance` | ⚡ Performance |
| `ci`, `build`, `dependencies` | 📦 Build & CI |
| `skip-changelog` | (excluded) |
| everything else | 📝 Other Changes |

## Hotfix process

For urgent fixes on a released version:

```bash
# Create a branch from the tag
git checkout -b hotfix/v0.2.1 v0.2.0

# Fix, commit, push
git push origin hotfix/v0.2.1

# Tag the hotfix
git tag v0.2.1
git push origin v0.2.1

# Merge back to main
git checkout main
git merge hotfix/v0.2.1
git push origin main
```
