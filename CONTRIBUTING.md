# Contributing to the CyberArk Secrets Provider for Kubernetes

Thanks for your interest in the CyberArk Secrets Provider for Kubernetes. We welcome contributions!

## Table of Contents

- [Contributing to the CyberArk Secrets Provider for Kubernetes](#contributing-to-the-cyberark-secrets-provider-for-kubernetes)
  * [Prerequisites](#prerequisites)
    + [Go](#go)
  * [Documentation](#documentation)
    + [Get up and running](#get-up-and-running)
  * [Contributing](#contributing)
    + [Contributing workflow](#contributing-workflow)
    + [Testing](#testing)
      - [Unit testing](#unit-testing)
      - [Integration testing](#integration-testing)
  * [Releases](#releases)
    + [Update the version, changelog, and notices](#update-the-version--changelog--and-notices)
    + [Add a git tag](#add-a-git-tag)
    + [Publish the git release](#publish-the-git-release)

## Prerequisites

### Go

To work in this codebase, you will want to have Go version 1.12+ installed.

## Documentation

The full documentation for the Cyberark Secrets Provider for Kubernetes can be found [here](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm) for DAP and [here](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm) for OSS.

### Get up and running

Before you can start contributing to the CyberArk Secrets Provider for Kubernetes project, you must:

1. Setup your environment. 
    
    a. For detailed instructions on how to setup a DAP env, see [DAP Setup](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/HomeTilesLPs/LP-Tile1.htm).
    
    b. For detailed instructions on how to setup an OSS env, see [OSS Setup](https://docs.conjur.org/Latest/en/Content/HomeTilesLPs/LP-Tile1.htm).

2. Setup the CyberArk Secrets Provider for Kubernetes

    a. For detailed setup instructions for DAP, see [CyberArk Secrets Provider for Kubernetes for DAP](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm).
    
    b. For detailed setup instructions for OSS, see [CyberArk Secrets Provider for Kubernetes for OSS](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm).
    

## Contributing

### Contributing workflow

1. Search our [open issues](https://github.com/cyberark/secrets-provider-for-k8s/issues) in GitHub to see what features are planned.

1. Select an existing issue or open a new issue to propose changes or fixes.

1. Add the `implementing` label to the issue that you open or modify.

1. Run [existing tests](#testing) locally and ensure they pass.

1. Create a branch and add your changes. Include appropriate tests and ensure that they pass.

1. Submit a pull request, linking the issue in the description (e.g. Connected to #123).

1. Add the `implemented` label to the issue and request that a Cyberark engineer reviews and merges your code.

From here your pull request is reviewed. Once you have implemented all reviewer feedback, your code is merged into the project. Congratulations, you're a contributor!

### Testing

#### Unit testing
For our Go unit testing, we use the [GoConvey](http://goconvey.co/) testing tool.  

To run existing unit tests, run `./bin/test_unit`

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
  
#### Integration testing

You can run integrations in different environment- local, demo, docker with either OSS or DAP deployments. 
All you need to do is run `./bin/test_integration` with the proper flags.

Run on docker: `--docker`

Run demo: `--demo`

Run locally: no flag is supplied

Additionally, concatenate `--dap` to the command to deploy DAP. By default, the integration tests run OSS, so no flag is needed.

For example, to deploy DAP locally, run  `./bin/test_integration --dap` or on docker `./bin/test_integration --docker --dap`

When contributing new intregration tests, perform the following:
1. Navigate to the `test/test_case` folder

1. Create a new test file with filename prefix `TEST_ID_<HIGHEST_NUMBER>_<TEST_NAME>`

If your tests follow the above instructions, our scripts should grab your test additions and run it as our test suite. 

That's it!

## Releases

Releases should be created by maintainers only. To create a tag and release,
follow the instructions in this section.

### Update the version, changelog, and notices
1. Create a new branch for the version bump.
1. Based on the unreleased content, determine the new version number and update
   the [version](pkg/secrets/version.go) file.
1. Review the git log and ensure the [changelog](CHANGELOG.md) contains all
   relevant recent changes with references to GitHub issues or PRs, if possible.
1. Review the changes since the last tag, and if the dependencies have changed
   revise the [NOTICES](NOTICES.txt) to correctly capture the included
   dependencies and their licenses / copyrights.
1. Commit these changes - `Bump version to x.y.z` is an acceptable commit
   message - and open a PR for review.
   
### Add a git tag
1. Once your changes have been reviewed and merged into master, tag the version
   using `git tag -s v0.1.1`. Note this requires you to be able to sign releases.
   Consult the [github documentation on signing commits](https://help.github.com/articles/signing-commits-with-gpg/)
   on how to set this up. `vx.y.z` is an acceptable tag message.
1. Push the tag: `git push vx.y.z` (or `git push origin vx.y.z` if you are working
   from your local machine).

### Publish the git release
1. In the GitHub UI, create a release from the new tag and copy the change log
for the new version into the GitHub release description.
1. The Jenkins pipeline auto-publishes new images to DockerHub, but to publish the Red Hat certified image you will need 
to visit its [management page](https://connect.redhat.com/project/4381831/view) and manually publish the image.

### Publish the Red Hat image
1. Visit the [Red Hat project page](https://connect.redhat.com/project/4381831/view) once the images have been pushed 
and manually choose to publish the latest release.
