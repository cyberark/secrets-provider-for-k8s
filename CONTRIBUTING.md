# Contributing to the CyberArk Secrets Provider for Kubernetes

Thanks for your interest in the CyberArk Secrets Provider for Kubernetes. We welcome contributions!

## Table of Contents

- [Contributing to the CyberArk Secrets Provider for Kubernetes](#contributing-to-the-cyberark-secrets-provider-for-kubernetes)
  - [Table of Contents](#table-of-contents)
  - [Prerequisites](#prerequisites)
    - [Go](#go)
  - [Documentation](#documentation)
    - [Get up and running](#get-up-and-running)
    - [Deploy a Local Dev Environment (K8s)](#deploy-a-local-dev-environment-k8s)
      - [Prerequisites](#prerequisites-1)
      - [Deploy a local development environment](#deploy-a-local-development-environment)
      - [Run integration tests against local development environment](#run-integration-tests-against-local-development-environment)
      - [Clean-up](#clean-up)
      - [Limitations](#limitations)
  - [Contributing](#contributing)
    - [Contributing workflow](#contributing-workflow)
    - [Testing](#testing)
      - [Unit testing](#unit-testing)
      - [Integration testing](#integration-testing)
      - [Tracing](#tracing)
  - [Releases](#releases)
    - [Pre-requisites](#pre-requisites)
    - [Update the version, changelog, and notices](#update-the-version-changelog-and-notices)
    - [Push Helm package](#push-helm-package)
    - [Release and Promote](#release-and-promote)
    - [Publish the Red Hat image](#publish-the-red-hat-image)

## Prerequisites

### Go

To work in this codebase, you will want to have Go version 1.12+ installed.

## Documentation

The full documentation for the Cyberark Secrets Provider for Kubernetes can be found [here](https://docs.cyberark.com/conjur-enterprise/latest/en/content/resources/_topnav/cc_home.htm) for Secrets Manager, Self-Hosted and [here](https://docs.cyberark.com/conjur-open-source/latest/en/content/overview/conjur-oss-suite-overview.html) for Conjur Open Source.

### Get up and running

Before you can start contributing to the CyberArk Secrets Provider for Kubernetes project, you must:

1. Setup your environment.

    a. For detailed instructions on how to setup a Secrets Manager, Self-Hosted env, see [Secrets Manager, Self-Hosted Setup](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/HomeTilesLPs/LP-Tile1.htm).

    b. For detailed instructions on how to setup a Conjur Open Source env, see [Conjur Open Source Setup](https://docs.conjur.org/Latest/en/Content/HomeTilesLPs/LP-Tile1.htm).

2. Setup the CyberArk Secrets Provider for Kubernetes

    a. For detailed setup instructions for Secrets Manager, Self-Hosted, see [CyberArk Secrets Provider for Kubernetes for Secrets Manager, Self-Hosted](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm).

    b. For detailed setup instructions for Conjur Open Source, see [CyberArk Secrets Provider for Kubernetes for Conjur Open Source](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm).

### Deploy a Local Dev Environment (K8s)

You can now deploy a local development environment for Kubernetes using [Docker Desktop](https://www.docker.com/products/docker-desktop). Docker Desktop provides a convenient way to deploy and develop from your machine against a locally deployed cluster.

#### Prerequisites

1. [Docker Desktop](https://www.docker.com/products/docker-desktop) installed

1. Kubernetes enabled in Docker Desktop

    1. Navigate to Docker Preferences

    1. Click on the Kubernetes tab and "Enable Kubernetes"

1. The Secrets Provider for K8s uses the [Kubernetes Conjur deploy](https://github.com/cyberark/kubernetes-conjur-deploy/blob/master/CONTRIBUTING.md) repository to deploy Secrets Manager, Self-Hosted / Conjur Open Source on Kubernetes.
   By default, 2.0 Gib of memory is allocated to Docker on your computer.

   To successfully deploy a Secrets Manager, Self-Hosted cluster (Leader + Followers + Standbys), you will need to increase the memory limit to 6 Gib. To do so, perform the following:

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
You can also view Secrets Manager, Self-Hosted / Conjur Open Source pod logs by running `kubectl get pods -n local-conjur` and `kubectl logs <conjur-pod-name> -n local-conjur`

1. If a cluster is already locally deployed run `./bin/start --dev --reload` to build your local changes and redeploy them to the local Secrets Provider K8s cluster

#### Run integration tests against local development environment

Assuming you have a running local cluster per the previous section and a valid kubeconfig, you can run the Golang-based integration tests by simply running `go test -timeout 0 -v ./e2e --tags=e2e` from the project root folder. This will execute the tests against the local cluster.

NOTE: This assumes that the cluster is running with the default values in `bootstrap.env` for local DEV clusters.

#### Clean-up

To remove K8s resources from your local environment perform the following:

1. Run `kubectl get all --all-namespaces` to list all resources across all namespaces in your cluster

1. Run `kubectl delete <resource-type> <name-of-resource> --namespace <namespace>`

Note that for Deployments, you must first delete the Deployment and then the Pod. Otherwise the Pod will terminate and another will start it its place.

#### Limitations

- Currently, deploying a local dev environment only works against a local K8s cluster and not an Openshift one

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

To run existing unit tests, run `./bin/test_unit`

When contributing to the CyberArk Secrets Provider for Kubernetes project, be sure to add the appropriate unit tests to either
already existing test files or create new ones.

To follow [Go testing conventions](https://golang.org/pkg/cmd/go/internal/test/) when creating a new test file, perform the following:

1. Create a new test file that matches the file naming pattern "*_test.go" in the proper `pkg` folder, close to the source code.

1. Add the following to the import statement at the beginning of the file

    ```go
    import (
        "testing"
    )
    ```

1. Create tests.

1. Run test suite, `./bin/test_unit`

#### Integration testing

Our integration tests can be run against either a GKE / Openshift remote cluster. To do so, run `./bin/start` and add the proper flags.

To deploy Secrets Manager, Self-Hosted / Conjur Open Source, add the `--oss` / `--dap` flags to the above command. By default, the integration tests run Secrets Manager, Self-Hosted, so no flag is required.
To deploy on GKE, add `--gke`. For Openshift, use `--oldest` / `--current` / `--next`. By default, the integration tests run on a GKE cluster,
so no flag is required.

For example:

- Deploy Conjur Open Source on GKE, run  `./bin/start --oss --gke`
- Deploy Secrets Manager, Self-Hosted on Openshift, run  `./bin/start --dap --current`

#### Tracing

To view tracing output from Secrets Provider, you can enable Jaeger tracing.

1. Create a local Jaeger instance.

    ```bash
    kubectl create namespace jaeger
    kubectl config set-context --current --namespace=jaeger
    helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
    helm install jaeger jaegertracing/jaeger
    ```

    Wait about 5 minutes for the Jaeger instance to be ready (you can check the status with `kubectl get pods -n jaeger`).
    Then run the following commands to expose the Jaeger UI on [localhost:8080](http://localhost:8080):

    ```bash
    pod_name=$(kubectl get pods --namespace jaeger -l "app.kubernetes.io/instance=jaeger,app.kubernetes.io/component=query" -o jsonpath="{.items[0].metadata.name}")
    kubectl port-forward --namespace jaeger $pod_name 8080:16686
    ```

2. Uncomment the following line in the template for the environment you are using in `bootstrap.env`,
    for example, if using `SECRETS_MODE="p2f-rotation"` then you'll need to update
    [deploy/dev/config/k8s/secrets-provider-p2f-rotation.sh.yml](deploy/dev/config/k8s/secrets-provider-p2f-rotation.sh.yml):

    ```yaml
    conjur.org/jaeger-collector-url: http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces
    ```

    Now reload the dev environment with `./bin/start --dev --reload` and optionally run
    the e2e tests with `go test -timeout 0 -v ./e2e --tags=e2e`.

## Releases

Releases should be created by maintainers only. To create a tag and release,
follow the instructions in this section.

### Pre-requisites

1. Review the git log and ensure the [changelog](CHANGELOG.md) contains all
   relevant recent changes with references to GitHub issues or PRs, if possible.
1. Ensure that all documentation that needs to be written has been
   written by TW, approved by PO/Engineer, and pushed to the forward-facing documentation.
1. Scan the project for vulnerabilities

### Update the changelog and notices (if necessary)

1. Update the `CHANGELOG.md` file with the new version and the changes that are included in the release.

1. Update `NOTICES.txt`

    ```sh-session
    go install github.com/google/go-licenses@latest

    # Verify that dependencies fit into supported licenses types.
    # If there is new dependency having unsupported license, that license should be
    # included to notices.tpl file in order to get generated in NOTICES.txt.
    $(go env GOPATH)/bin/go-licenses check ./... \
      --allowed_licenses="MIT,ISC,Apache-2.0,BSD-3-Clause,BSD-2-Clause" \
      --ignore $(go list std | awk 'NR > 1 { printf(",") } { printf("%s",$0) } END { print "" }')

    # If no errors occur, proceed to generate updated NOTICES.txt
    $(go env GOPATH)/bin/go-licenses report ./... \
      --template notices.tpl \
      --ignore github.com/cyberark/secrets-provider-for-k8s \
      --ignore $(go list std | awk 'NR > 1 { printf(",") } { printf("%s",$0) } END { print "" }') \
      > NOTICES.txt
    ```

### Update the version, changelog, and notices

1. Create a new branch for the version bump.
1. Based on the changelog content, determine the new version number.
1. Update this version in the following files:
    1. [version.go](pkg/secrets/version.go)
    1. [Chart version](helm/secrets-provider/Chart.yaml)
    2. [Helm unit test for chart defaults](helm/secrets-provider/tests/secrets_provider_test.yaml)
2. Commit these changes - `Bump version to x.y.z` is an acceptable commit
   message - and open a PR for review.

### Push Helm package

1. Every build packages the Secrets Provider Helm chart for us. The package can be found under the 'Artifacts' tab of the Jenkins build and will resemble `secrets-provider-<version>.tgz`.
Navigate to the 'Artifacts' tab of the _tagged version_ build and save this file. You will need it for the next step.
1. Clone the repo [helm-charts](https://github.com/cyberark/helm-charts) and do the following:
    1. Move the Helm package file created in the previous step to the *docs* folder in the `helm-charts` repo.
    1. Go to the `helm-charts` repo root folder and execute the `reindex.sh` script file located there.
    1. Create a PR with those changes.

### Release and Promote

1. Merging into main/master branches will automatically trigger a release. If successful, this release can be promoted at a later time.
1. Jenkins build parameters can be utilized to promote a successful release or manually trigger aditional releases as needed.
1. Reference the [internal automated release doc](https://github.com/conjurinc/docs/blob/master/reference/infrastructure/automated_releases.md#release-and-promotion-process) for releasing and promoting.

### Publish the Red Hat image

1. Visit the [Red Hat project page](https://connect.redhat.com/project/4381831/view) once the images have been pushed
and manually choose to publish the latest release.
