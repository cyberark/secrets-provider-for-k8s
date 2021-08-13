# Contributing to the CyberArk Secrets Provider for Kubernetes

Thanks for your interest in the CyberArk Secrets Provider for Kubernetes. We welcome contributions!

## Table of Contents

- [Contributing to the CyberArk Secrets Provider for Kubernetes](#contributing-to-the-cyberark-secrets-provider-for-kubernetes)
  * [Prerequisites](#prerequisites)
    + [Go](#go)
  * [Documentation](#documentation)
    + [Get up and running](#get-up-and-running)
    + [Deploy a Local Dev Environment (K8s)](#deploy-a-local-dev-environment--k8s-)
      - [Prerequisites](#prerequisites-1)
      - [Deploy a local development environment](#deploy-a-local-development-environment)
      - [Clean-up](#clean-up)
      - [Limitations](#limitations)
  * [Contributing](#contributing)
    + [Contributing workflow](#contributing-workflow)
    + [Testing](#testing)
      - [Unit testing](#unit-testing)
      - [Integration testing](#integration-testing)
  * [Releases](#releases)
    + [Pre-requisites](#pre-requisites)
    + [Update the version, changelog, and notices](#update-the-version--changelog--and-notices)
    + [Add a git tag](#add-a-git-tag)
    + [Push Helm package](#push-helm-package)
    + [Publish the git release](#publish-the-git-release)
    + [Publish the Red Hat image](#publish-the-red-hat-image)

## Prerequisites

### Go

To work in this codebase, you will want to have Go version 1.12+ installed.

## Documentation

The full documentation for the Cyberark Secrets Provider for Kubernetes can be found [here](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm) for Conjur Enterprise and [here](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm) for Conjur Open Source.

### Get up and running

Before you can start contributing to the CyberArk Secrets Provider for Kubernetes project, you must:

1. Setup your environment. 
    
    a. For detailed instructions on how to setup a Conjur Enterprise env, see [Conjur Enterprise Setup](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/HomeTilesLPs/LP-Tile1.htm).
    
    b. For detailed instructions on how to setup a Conjur Open Source env, see [Conjur Open Source Setup](https://docs.conjur.org/Latest/en/Content/HomeTilesLPs/LP-Tile1.htm).

2. Setup the CyberArk Secrets Provider for Kubernetes

    a. For detailed setup instructions for Conjur Enterprise, see [CyberArk Secrets Provider for Kubernetes for Conjur Enterprise](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm).
    
    b. For detailed setup instructions for Conjur Open Source, see [CyberArk Secrets Provider for Kubernetes for Conjur Open Source](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm).
    
### Deploy a Local Dev Environment (K8s)

You can now deploy a local development environment for Kubernetes using [Docker Desktop](https://www.docker.com/products/docker-desktop). Docker Desktop provides a convenient way to deploy and develop from your machine against a locally deployed cluster. 

#### Prerequisites

1. [Docker Desktop](https://www.docker.com/products/docker-desktop) installed

1. Kubernetes enabled in Docker Desktop

    1. Navigate to Docker Preferences
    
    1. Click on the Kubernetes tab and "Enable Kubernetes"
    
1. The Secrets Provider for K8s uses the [Kubernetes Conjur deploy](https://github.com/cyberark/kubernetes-conjur-deploy/blob/master/CONTRIBUTING.md) repository to deploy Conjur Enterprise / Conjur Open Source on Kubernetes. 
   By default, 2.0 Gib of memory is allocated to Docker on your computer.
   
   To successfully deploy a Conjur Enterprise cluster (Leader + Followers + Standbys), you will need to increase the memory limit to 6 Gib. To do so, perform the following:
   
   1. Navigate to Docker preferences
   
   1. Click on "Resources" and slide the "Memory" bar to 6
  
1. If you intend to deploy the Secrets Provider via Helm, you will need to install the Helm CLI. See [here](https://helm.sh/docs/intro/install/) for instructions on how to do so.

#### Deploy a local development environment

To deploy a local development environment, perform the following:

1. Run `./bin/build` to build the Secrets Provider image locally 

1. Ensure you are in the proper local context. Otherwise, the deployment will not run successfully

Run ` kubectl config current-context` to verify which context you are currently in so if needed, you can switch back to it easily

Run `kubectl config use-context docker-desktop` to switch to a local context. This is the context you will need to run the development environment

1. Navigate to `bootstrap.env` and uncomment the `Local DEV Env` section, ensuring that `DEV=true`. Additionally, you can deploy the Secrets Provider locally using HELM. To do so, _also_ uncomment `DEV_HELM`

1. Run `./bin/start --dev`, appending `--oss` or `--dap` according to the environment that needs to be deployed

1. To view the pod(s) that were deployed and the Secrets Provider logs, run `kubectl get pods` and `kubectl logs <pod-name> -c cyberark-secrets-provider-for-k8s` respectively. 
You can also view Conjur Enterprise / Conjur Open Source pod logs by running `kubectl get pods -n local-conjur` and `kubectl logs <conjur-pod-name> -n local-conjur`

1. If a cluster is already locally deployed run `./bin/start --dev --reload` to build your local changes and redeploy them to the local Secrets Provider K8s cluster

#### Clean-up

To remove K8s resources from your local environment perform the following:

1. Run `kubectl get all --all-namespaces` to list all resources across all namespaces in your cluster

1. Run `kubectl delete <resource-type> <name-of-resource> --namespace <namespace>`

Note that for Deployments, you must first delete the Deployment and then the Pod. Otherwise the Pod will terminate and another will start it its place.

#### Limitations

- Currently, deploying a local dev environment only works against a local K8s cluster and not an Openshift one

- At current, we cannot run our integration tests locally, only against a remote cluster

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

Our integration tests can be run against either a GKE / Openshift remote cluster. To do so, run `./bin/start` and add the proper flags. 

To deploy Conjur Enterprise / Conjur Open Source, add the `--oss` / `--dap` flags to the above command. By default, the integration tests run Conjur Enterprise, so no flag is required.
To deploy on GKE, add `--gke`. For Openshift, use `--oldest` / `--current` / `--next`. By default, the integration tests run on a GKE cluster,
so no flag is required.

For example:

- Deploy Conjur Open Source on GKE, run  `./bin/start --oss --gke`
- Deploy Conjur Enterprise on Openshift, run  `./bin/start --dap --oc311`

It is also possible to run a single test instead of the full suite. This can be done by running `./bin/start` with the
flag `--test-prefix=<prefix>`. For example, running with the flag `--test-prefix=TEST_ID_18` will run only the test
`TEST_ID_18_helm_multiple_provider_multiple_secrets.sh`, or run with the flag `--test-prefix=*1[789]` to run tests 17,18,19.

When contributing new integration tests, perform the following:

1. Navigate to the `test/test_case` folder

1. Create a new test file with filename prefix `TEST_ID_<HIGHEST_NUMBER>_<TEST_NAME>`

If your tests follow the above instructions, our scripts should grab your test additions and run it as our test suite. 

That's it!

## Releases

Releases should be created by maintainers only. To create a tag and release,
follow the instructions in this section.

### Pre-requisites 

1. Review the git log and ensure the [changelog](CHANGELOG.md) contains all
   relevant recent changes with references to GitHub issues or PRs, if possible.
1. Review the changes since the last tag, and if the dependencies have changed
   revise the [NOTICES](NOTICES.txt) to correctly capture the included
   dependencies and their licenses / copyrights.
1. Ensure that all documentation that needs to be written has been 
   written by TW, approved by PO/Engineer, and pushed to the forward-facing documentation.
1. Scan the project for vulnerabilities

### Update the version, changelog, and notices
1. Create a new branch for the version bump.
1. Based on the unreleased content, determine the new version number.
1. Update this version in the following files:
    1. [Version file](pkg/secrets/version.go)
    1. [Chart version](helm/secrets-provider/Chart.yaml)
    1. [Default deployed version](helm/secrets-provider/values.yaml)
    1. [Helm unit test for chart defaults](helm/secrets-provider/tests/secrets_provider_test.yaml)
    1. [Test case hardcoded version](deploy/test/test_cases/TEST_ID_22_helm_rbac_defaults_taken_successfully.sh)
1. Commit these changes - `Bump version to x.y.z` is an acceptable commit
   message - and open a PR for review.
   
### Add a git tag
1. Once your changes have been reviewed and merged into main, tag the version
   using `git tag -s v0.1.1`. Note this requires you to be able to sign releases.
   Consult the [github documentation on signing commits](https://help.github.com/articles/signing-commits-with-gpg/)
   on how to set this up. `vx.y.z` is an acceptable tag message.
1. Push the tag: `git push vx.y.z` (or `git push origin vx.y.z` if you are working
   from your local machine).

### Push Helm package
1. Every build packages the Secrets Provider Helm chart for us. The package can be found under the 'Artifacts' tab of the Jenkins build and will resemble `secrets-provider-<version>.tgz`. 
Navigate to the 'Artifacts' tab of the _tagged version_ build and save this file. You will need it for the next step.
1. Clone the repo [helm-charts](https://github.com/cyberark/helm-charts) and do the following:
    1. Move the Helm package file created in the previous step to the *docs* folder in the `helm-charts` repo.
    1. Go to the `helm-charts` repo root folder and execute the `reindex.sh` script file located there.
    1. Create a PR with those changes.
    
### Publish the git release
1. In the GitHub UI, create a release from the new tag and copy the change log
for the new version into the GitHub release description. The Jenkins pipeline 
will auto-publish new images to DockerHub.
                                                            
### Publish the Red Hat image
1. Visit the [Red Hat project page](https://connect.redhat.com/project/4381831/view) once the images have been pushed 
and manually choose to publish the latest release.
