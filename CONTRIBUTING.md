# Contributing to the CyberArk Secrets Provider for Kubernetes

Thanks for your interest in the CyberArk Secrets Provider for Kubernetes. We welcome contributions!

## Table of Contents

- [Prerequisites](#prerequisites)
- [Documentation](#documentation)
    - [Get up and running](#get-up-and-running)
- [Contributing](#contributing)
    - [Style guide](#style-guide)
    - [Pull Request Workflow](#pull-request-workflow)
- [Releases](#releasing)
    - [Update the version and changelog](#update-the-version-and-changelog)

## Prerequisites

### Go

To work in this codebase, you will want to have Go installed.

## Documentation

Cyberark Secrets Provider for Kubernetes documentation can be found [here](https://www.docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm)

### Get up and running

Before you can start contributing to the CyberArk Secrets Provider for Kubernetes project, you must first setup your environment. 
See [here](https://www.docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm) for detailed directions on how to do so.

## Contributing

1. [Fork the project](https://help.github.com/en/github/getting-started-with-github/fork-a-repo)

2. [Clone your fork](https://help.github.com/en/github/creating-cloning-and-archiving-repositories/cloning-a-repository)

3. Make local changes to your fork by editing files

4. [Commit your changes](https://help.github.com/en/github/managing-files-in-a-repository/adding-a-file-to-a-repository-using-the-command-line)

5. [Push your local changes to the remote server](https://help.github.com/en/github/using-git/pushing-commits-to-a-remote-repository)

6. [Create new Pull Request](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request-from-a-fork)

### Style guide

Use [this guide](STYLE.md) to maintain consistent style across the Cyberark Secrets Provider for Kubernetes project.

### Pull Request Workflow

1. Search the open issues in GitHub to find out what has been planned
2. Select an existing issue or open an issue to propose changes or fixes
3. Add the implementing label to the issue as you begin to work on it
4. Run tests as described here, ensuring they pass
5. Submit a pull request, linking the issue in the description (e.g. Connected to #123)
6. Add the implemented label to the issue, and ask another contributor to review and merge your code

From here your pull request will be reviewed and once you've responded to all feedback it will be merged into the project. Congratulations, you're a contributor!

## Releases

### Update the version and changelog

1. Create a new branch from `master` for the version bump.
2. Update the [`version`](VERSION) file to the new version number.
3. Add to the already existing `changelog.md` a description of the new changes that will be included in the release (Fixed, Added, Changed).
4. Commit these changes - Bump version to x.y.z is an acceptable commit message - and open a PR for review.
5. Once the PR has been reviewed and merged by a Cyberark engineer, create a tag in Github.
    
    a. Go to "Release" -> "Draft a new release"
    
    b. Add a tag version and a release title (both should be `v<number_of_version>`)
    
    c. Add the contents of the changelog in the description
    
    d. Publish the release