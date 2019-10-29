# Contributing to the CyberArk Secrets Provider for Kubernetes

Thanks for your interest in the CyberArk Secrets Provider for Kubernetes. Before contributing, please take a moment to read and sign our [Contributor Agreement](Contributing_OSS/CyberArk_Open_Source_Contributor_Agreement.pdf). This provides patent protection for all Secretless Broker users and allows CyberArk to enforce its license terms. Please email a signed copy to oss@cyberark.com.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Pull Request Workflow](#pullrequestworkflow)
- [Style guide](#styleguide)
- [Documentation](#documentation)
- [Releasing](#releasing)

## Prerequisites

### Go version

To work in this codebase, you will want to have Go installed.

### Gitleaks

We use [gitleaks](https://github.com/zricethezav/gitleaks) as a pre-push hook to ensure that all code pushed is checked for secrets.
This provides us with an extra safety net that alerts us if and when we accidentally attempt to push unencrypted secrets to our source code.

Normally git hooks are per-clone, which makes configuring them a burden, however `core.hooksPath` lets us set a single location git will search for hooks.

1. Install gitleaks

```terminal
brew install gitleaks
# OR docker pull zricethezav/gitleaks
# OR go get -u github.com/zricethezav/gitleaks
```

2. Configure hooksPath in user git configuration (~/.gitconfig)

```
[core]
hooksPath = ~/git-hooks
```

3. Create ~/git-hooks/pre-push, make it executable and add the following:

```terminal
#!/bin/bash -eu

set -o pipefail

if ! command -v gitleaks &> /dev/null; then
  echo "ERROR: Gitleaks not installed!"
  exit 1
fi

commits="$(git log origin/master..HEAD --oneline | wc -l | xargs)"

if [ "$commits" -eq 0 ]; then
  echo "WARN: No commits different from origin/master - skipping gitleaks..."
  exit 0
fi

branch=$(git rev-parse --abbrev-ref HEAD)

if grep -q whitelist .gitleaks.toml &> /dev/null; then
  gitleaks -v \
          --repo-path . \
          --config .gitleaks.toml \
          --branch "$branch" \
          --depth "$commits"
else
  echo "WARN: Gitleaks config not found - using defaults..."
  gitleaks -v \
          --repo-path . \
          --branch "$branch" \
          --depth "$commits"
  fi
```

## Pull Request Workflow

1. Search the open issues in GitHub to find out what has been planned
2. Select an existing issue or open an issue to propose changes or fixes
3. Add the implementing label to the issue as you begin to work on it
4. Run tests as described here, ensuring they pass
5. Submit a pull request, linking the issue in the description (e.g. Connected to #123)
6. Add the implemented label to the issue, and ask another contributor to review and merge your code

## Style guide

Use [this guide](STYLE.md) to maintain consistent style across the Cyberark Secrets Provider for Kubernetes project.

## Documentation

Cyberark Secrets Provider for Kubernetes documentation can be found in the Cyberark docs <TODO: insert link when doc site is published>

## Releasing

### Update the version and changelog

1. Create a new branch from `master` for the version bump.
2. Update the `version.go` file to the new version number.
3. Add to the already existing `changelog.md` a description of the new changes that will be included in the release (Fixed, Added, Changed).
4. Commit these changes - Bump version to x.y.z is an acceptable commit message - and open a PR for review.
5. Once the PR has been reviewed and merged by a Cyberark engineer, create a tag in Github.
    
    a. Go to "Release" -> "Draft a new release"
    b. Add a tag version and a release title (both should be `v<number_of_version>`)
    c. Add the contents of the changelog in the description
    d. Publish the release