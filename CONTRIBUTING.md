# Contributing to the CyberArk Secrets Provider for Kubernetes

Thanks for your interest in the CyberArk Secrets Provider for Kubernetes. We welcome contributions!

## Table of Contents

- [Prerequisites](#prerequisites)
- [Documentation](#documentation)
    - [Get up and running](#get-up-and-running)
- [Contributing](#contributing)
    - [Contributing workflow](#contributing-workflow)
    - [Testing](#testing)
- [Releases](#releases)
    - [Update the version and changelog](#update-the-version-and-changelog)

## Prerequisites

### Go

To work in this codebase, you will want to have Go installed.

## Documentation

The full documentation for the Cyberark Secrets Provider for Kubernetes can be found [here](https://www.docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm)

### Get up and running

Before you can start contributing to the CyberArk Secrets Provider for Kubernetes project, you must first setup your environment. 

For detailed setup instructions, see [CyberArk Secrets Provider for Kubernetes Secrets](https://www.docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm).

## Contributing

### Contributing workflow

1. Search our [open issues](https://github.com/cyberark/cyberark-secrets-provider-for-k8s/issues) in GitHub to see what features are planned.
1. Select an existing issue or open a new issue to propose changes or fixes.
1. Add the `implementing` label to the issue that you open or modify.
1. Run [existing tests](#testing) locally and ensure they pass.
1. Create a branch and add your changes. Include appropriate tests and ensure that they pass.
1. Submit a pull request, linking the issue in the description (e.g. Connected to #123).
1. Add the `implemented` label to the issue and request that a Cyberark engineer reviews and merges your code.

From here your pull request is reviewed. Once you have implemented all reviewer feedback, your code is merged into the project. Congratulations, you're a contributor!

### Testing

For our Go testing, we use the [GoConvey](http://goconvey.co/) testing tool.  

In order to run existing unit tests, run `./bin/test_unit`

When contributing to the CyberArk Secrets Provider for Kubernetes project, be sure to add the appropriate unit tests to either
already existing test files or create new ones.

To follow [Go testing conventions](https://golang.org/pkg/cmd/go/internal/test/) when creating a new test file, perform the following:
1. Create a new test file that matches the file naming pattern "*_test.go" in the proper `pkg` folder, close to the source code.
1. Add the following to the import statement at the beginning of the file
    ```go
    import (
        "testing"
        . "github.com/smartystreets/goconvey/convey"
    )
    ```
1. Create tests according to the [GoConvey](https://github.com/smartystreets/goconvey/wiki) formatting and styling guidelines 
1. Run test suite, `./bin/test_unit`
  
## Releases

### Update the version and changelog

1. Create a new branch from `master` for the version bump.
1. Update the [`version`](pkg/secrets/version.go) file to the new version number.
1. Add a description to the already existing `CHANGELOG.md`of the new changes included in the release (Fixed, Added, Changed).
1. Commit these changes - "Bump version to x.y.z" is an acceptable commit message - and open a PR for review.
1. Once the PR has been reviewed and merged by a Cyberark engineer, create a tag in Github.
    
    a. Go to "Release" -> "Draft a new release"
    
    b. Add a tag version and a release title (both should be `v<number_of_version>`, i.e `v1.2.3`)
    
    c. Add the contents of the changelog in the description
    
    d. Publish the release