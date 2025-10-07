# Release Process

This document describes the release process for heimdall-go.

## Overview

Releases are fully automated using [GoReleaser](https://goreleaser.com/) and GitHub Actions. When you push a semantic version tag (e.g., `v0.2.0`), GitHub Actions automatically:

1. Runs all tests with race detection
2. Runs linter checks
3. Generates a changelog from commit history
4. Creates a GitHub Release with release notes
5. Uploads source archives and checksums

## Prerequisites

- Write access to the repository
- Local git configured with your GitHub credentials
- All CI checks passing on `main` branch

## Release Types

### Semantic Versioning

We follow [Semantic Versioning](https://semver.org/):

```
v0.2.3
 │ │ └─ PATCH: Bug fixes, no API changes
 │ └─── MINOR: New features, backward compatible
 └───── MAJOR: Breaking changes, incompatible API changes
```

### Version Guidelines

| Current Version | Change Type | New Version | When to Use |
|----------------|-------------|-------------|-------------|
| v0.1.0 | Bug fix | v0.1.1 | Fix issues, no new features |
| v0.1.0 | New feature | v0.2.0 | Add new functionality, backward compatible |
| v0.1.0 | Breaking change | v0.2.0 (or v1.0.0) | API changes that break existing code |
| v0.9.0 | Stability commitment | v1.0.0 | Ready for production, stable API |

**Note**: During v0.x.x development, breaking changes are allowed in MINOR versions (v0.1.0 → v0.2.0). Once we reach v1.0.0, breaking changes require a MAJOR version bump (v1.0.0 → v2.0.0).

## Creating a Release

### Step 1: Prepare the Release

1. **Ensure `main` is up to date:**
   ```bash
   git checkout main
   git pull origin main
   ```

2. **Verify all tests pass:**
   ```bash
   make test
   make lint
   ```

3. **Clean up dependencies:**
   ```bash
   go mod tidy
   go mod verify
   ```

4. **Commit any pending changes:**
   ```bash
   git add go.mod go.sum
   git commit -m "chore: prepare for vX.Y.Z release"
   git push origin main
   ```

### Step 2: Decide the Version Number

Based on the changes since the last release:

- **Patch (v0.1.0 → v0.1.1)**: Only bug fixes
  ```bash
  git log v0.1.0..HEAD --oneline | grep "^fix:"
  ```

- **Minor (v0.1.0 → v0.2.0)**: New features or breaking changes (during v0.x.x)
  ```bash
  git log v0.1.0..HEAD --oneline | grep -E "^(feat:|BREAKING CHANGE:)"
  ```

- **Major (v1.0.0 → v2.0.0)**: Breaking changes (after v1.0.0)

### Step 3: Create and Push the Tag

1. **Create an annotated tag:**
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0: Add topic management support"
   ```

   Use a descriptive message summarizing the key changes.

2. **Push the tag:**
   ```bash
   git push origin v0.2.0
   ```

3. **Monitor the release:**
   - Go to: https://github.com/ciaranRoche/heimdall-go/actions
   - Watch the "Release" workflow execute
   - Verify it completes successfully

### Step 4: Verify the Release

1. **Check the GitHub Release:**
   - Go to: https://github.com/ciaranRoche/heimdall-go/releases
   - Verify the release appears with correct version
   - Check that changelog is generated correctly
   - Verify source archives and checksums are attached

2. **Test installation:**
   ```bash
   # In a test project
   go get github.com/ciaranRoche/heimdall-go@v0.2.0
   ```

3. **Verify Go module proxy:**
   - Wait 5-10 minutes for proxy.golang.org to cache the module
   - Check: https://pkg.go.dev/github.com/ciaranRoche/heimdall-go@v0.2.0

## Release Workflow Details

### What Happens Automatically

When you push a tag, the `.github/workflows/release.yml` workflow:

1. ✅ Checks out the code with full git history
2. ✅ Sets up Go 1.23
3. ✅ Verifies the Go module
4. ✅ Runs all tests with race detection
5. ✅ Runs linter checks (if available)
6. ✅ Executes GoReleaser to create the release
7. ✅ Uploads release artifacts

### What GoReleaser Does

The `.goreleaser.yaml` configuration:

1. ✅ Generates changelog from git commit history
2. ✅ Groups changes by type (Features, Bug Fixes, etc.)
3. ✅ Creates source archives (`.tar.gz`)
4. ✅ Generates SHA256 checksums
5. ✅ Creates GitHub Release with formatted notes
6. ✅ Marks pre-release versions automatically (alpha, beta, rc)

### Commit Message Format

For best changelog generation, use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add support for Azure Service Bus provider
fix: resolve race condition in message delivery
docs: update installation instructions
chore: upgrade dependencies
refactor: improve error handling in routing engine
test: add integration tests for RabbitMQ

BREAKING CHANGE: rename Config.Bootstrap to Config.BootstrapServers
```

GoReleaser automatically groups these into sections:
- `feat:` → 🚀 Features
- `fix:` → 🐛 Bug Fixes
- `perf:` → ⚡ Performance Improvements
- `refactor:` → ♻️ Refactoring
- `docs:` → 📝 Documentation
- `test:` → 🧪 Tests

## Pre-releases

For testing releases before making them official:

### Alpha Release
```bash
git tag -a v0.2.0-alpha.1 -m "Alpha release for testing"
git push origin v0.2.0-alpha.1
```

### Beta Release
```bash
git tag -a v0.2.0-beta.1 -m "Beta release for wider testing"
git push origin v0.2.0-beta.1
```

### Release Candidate
```bash
git tag -a v0.2.0-rc.1 -m "Release candidate"
git push origin v0.2.0-rc.1
```

Pre-releases are automatically marked as "Pre-release" on GitHub.

## Hotfix Releases

For critical bug fixes that need immediate release:

1. **Create hotfix from tag:**
   ```bash
   git checkout v0.2.0
   git checkout -b hotfix/v0.2.1
   ```

2. **Make the fix and commit:**
   ```bash
   # Fix the bug
   git add .
   git commit -m "fix: critical bug in message routing"
   ```

3. **Create and push the patch tag:**
   ```bash
   git tag -a v0.2.1 -m "Hotfix v0.2.1: Fix critical routing bug"
   git push origin v0.2.1
   ```

4. **Merge back to main:**
   ```bash
   git checkout main
   git merge hotfix/v0.2.1
   git push origin main
   ```

## Troubleshooting

### Release Workflow Fails

**Check the Actions tab:** https://github.com/ciaranRoche/heimdall-go/actions

Common issues:

1. **Tests fail:**
   - Fix the tests on `main` branch
   - Delete the tag: `git tag -d v0.2.0 && git push origin :refs/tags/v0.2.0`
   - Recreate the tag after fixing

2. **Linter errors:**
   - Fix linting issues on `main` branch
   - Delete and recreate the tag

3. **Permission errors:**
   - Verify the workflow has `contents: write` permission (already configured)
   - Check GitHub token has not expired

### Tag Was Created Incorrectly

**Delete local and remote tag:**
```bash
git tag -d v0.2.0                          # Delete local tag
git push origin :refs/tags/v0.2.0          # Delete remote tag
```

**Recreate the tag:**
```bash
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

**Note**: Once a version is on proxy.golang.org, you cannot delete it. If you need to fix a bad release, create a new patch version (e.g., v0.2.1).

### GoReleaser Configuration Issues

**Test locally before pushing:**
```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Test without publishing
goreleaser release --snapshot --clean

# Check configuration
goreleaser check
```

## Major Version (v2+) Considerations

When eventually releasing v2.0.0, the Go module path must change:

### Current (v1):
```go
// go.mod
module github.com/ciaranRoche/heimdall-go
```

### After v2:
```go
// go.mod
module github.com/ciaranRoche/heimdall-go/v2
```

**Recommended approach:**
1. Create a `v2` subdirectory
2. Copy code to `v2/`
3. Update module path in `v2/go.mod`
4. Tag as `v2.0.0` from main branch

See [Go modules documentation](https://go.dev/blog/v2-go-modules) for details.

## Resources

- **GoReleaser Docs**: https://goreleaser.com/
- **Semantic Versioning**: https://semver.org/
- **Conventional Commits**: https://www.conventionalcommits.org/
- **Go Module Versioning**: https://go.dev/doc/modules/version-numbers
- **GitHub Actions**: https://docs.github.com/en/actions

## Quick Reference

```bash
# Create a new release
git checkout main
git pull origin main
make test && make lint
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0

# Delete a tag
git tag -d v0.2.0
git push origin :refs/tags/v0.2.0

# Create a pre-release
git tag -a v0.2.0-beta.1 -m "Beta release"
git push origin v0.2.0-beta.1

# Check what would be in next release
git log $(git describe --tags --abbrev=0)..HEAD --oneline
```
