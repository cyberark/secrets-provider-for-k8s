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
      - [Flags for local dev and E2E](#flags-for-local-dev-and-e2e)
      - [Common examples](#common-examples)
      - [Logs, reload, and running E2E from the repo](#logs-reload-and-running-e2e-from-the-repo)
      - [Clean-up](#clean-up)
      - [Limitations](#limitations)
  - [Contributing](#contributing)
    - [Contributing workflow](#contributing-workflow)
    - [Testing](#testing)
      - [Unit testing](#unit-testing)
      - [Integration testing](#integration-testing)
      - [Tracing](#tracing)
    - [Helm chart: Reloader image digest](#helm-chart-reloader-image-digest)
  - [Releases](#releases)
    - [Pre-requisites](#pre-requisites)
    - [Update the version, changelog, and notices](#update-the-version-changelog-and-notices)
    - [Push Helm package](#push-helm-package)
    - [Release and Promote](#release-and-promote)
    - [Publish the Red Hat image](#publish-the-red-hat-image)

## Prerequisites

### Go

To work in this codebase, you will want to have Go version 1.26.1+ installed.

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

Local Conjur and the test app are driven by `./bin/start` (see `./bin/start --help` for the full option list). The script uses [kubernetes-conjur-deploy](https://github.com/cyberark/kubernetes-conjur-deploy/blob/master/CONTRIBUTING.md) to bring up Conjur on Kubernetes.

#### Prerequisites

1. [Docker](https://www.docker.com/products/docker-desktop) (or another engine KinD can use). Heavier stacks (for example DAP) often need more RAM allocated to Docker; follow the memory guidance in the kubernetes-conjur-deploy contributing doc.

1. If you run `./bin/start --dev` against a desktop Kubernetes cluster (for example [Docker Desktop Kubernetes](https://docs.docker.com/desktop/kubernetes/)), enable Kubernetes in Docker Desktop, then point `kubectl` at that cluster (for example `kubectl config use-context docker-desktop`). You can use `kubectl config current-context` to confirm before and after.

1. Install the [Helm CLI](https://helm.sh/docs/intro/install/) if you plan to use `--dev-helm` outside of a containerized KinD cluster.

#### Flags for local dev and E2E

These `./bin/start` flags are the ones most often used for local development or running E2E tests locally:

- `--kind` — Use a local KinD cluster instead of the default GKE-oriented summon environment (recommended).

- `--docker` — Run the flow fully in Docker. This is recommended if running E2E tests for more predictable results, and is also useful when `kubectl`,  `oc`, or `helm` is not installed on the host.

- `--dev` — Start the **development** environment without running E2E tests.

- `--oss` / `--dap` — Conjur Open Source vs Secrets Manager or Self-Hosted (Enterprise/DAP).

- `--secrets-mode=MODE` — Sets `SECRETS_MODE`: `k8s` (default), `k8s-rotation`, `p2f`, `p2f-rotation`, or `standalone` (must be used with `--dev-helm`).

- `--labeled-secrets` / `--no-labeled-secrets` — Toggle labeled secrets in dev manifests (default is on when using `--dev`, off for CI-style runs).

- `--secrets-refresh-interval=DURATION` — Optional interval (for example `10m`, default is `5s`) for rotation/informer-style flows.

- `--dev-helm` — Deploy the Secrets Provider with Helm instead of K8s manifests, which is suitable for `application` or `standalone` modes (for the latter, also apply `--secrets-mode=standalone`).

- `--with-reloader` — Enable the Reloader subchart with chart defaults (requires `--dev-helm`).

- `--reload` — Rebuild and redeploy only the Secrets Provider against an **existing** local dev cluster. Must be combined with `--dev`, plus other deployment flags should be the same as those used to start the dev environment. This is mainly useful for testing new code changes in an already running dev environment, or when switching between secrets modes. 

#### Common examples

- `./bin/start --kind --docker` — Run E2E tests in a containerized KinD cluster.

- `./bin/start --kind --docker --dev` — Start the dev environment in a containerized KinD cluster.

- `./bin/start --kind --dev` — Start the dev environment on a **local** KinD cluster. This is fine for interactive dev, but if you are running E2E tests as well, prefer the containerized KinD flow (`--kind --docker`) for isolation and consistency.

- `./bin/start --kind --docker --dev --secrets-mode=k8s-rotation` — Containerized KinD cluster with Kubernetes secret rotation.

- `./bin/start --kind --docker --dev --dev-helm --secrets-mode=standalone --with-reloader` — Containerized KinD cluster with Helm standalone deployment and Reloader enabled.

After deploy, inspect workloads with `kubectl get pods`. 
Secrets Provider logs: `kubectl logs -n local-secrets-provider -l app=test-env -c cyberark-secrets-provider-for-k8s --tail=100`
Conjur OSS logs: `kubectl logs -l app=conjur-oss -n local-conjur -c conjur --tail=100`. 
If the cluster is containerized, kubectl commands should be run from within the E2E test container.

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

2. Uncomment the following line in the template for the environment you are using;
    for example, if using `SECRETS_MODE="p2f-rotation"` then you'll need to update
    [deploy/dev/config/k8s/secrets-provider-p2f-rotation.sh.yml](deploy/dev/config/k8s/secrets-provider-p2f-rotation.sh.yml):

    ```yaml
    conjur.org/jaeger-collector-url: http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces
    ```

    Now reload the dev environment with `./bin/start --dev --reload` and optionally run
    the e2e tests with `go test -timeout 0 -v ./e2e --tags=e2e`.

### Helm chart: Reloader image digest

To update the Reloader image used by this chart, maintainers must keep two fields in sync in `helm/secrets-provider/values.yaml`:

| Field                                      | Purpose                                                        |
|---------------------------------------------|----------------------------------------------------------------|
| `reloader.image.digest`                     | Image digest (`sha256:...`) for the desired image.             |
| `reloader.imageDigestReference.pullReference` | Complete source reference (`registry/repository:tag`) matching the digest. |

#### How to update

1. Ensure Docker with `buildx` and the Helm CLI are installed.
2. After editing `helm/secrets-provider/Chart.yaml`, run from the repo root:

   ```bash
   helm repo add stakater https://stakater.github.io/stakater-charts 2>/dev/null || true
   ( cd helm/secrets-provider && helm dependency update )
   ./bin/reloader-image-digest.sh sync
   ```

   This will automatically update `.digest` and `.pullReference` in `values.yaml` using the locked chart and current image settings.  
   If you’re using a private registry, set `reloader.image.repository` before running the sync command.

#### Manual/inspect mode

- To inspect or verify the configured digest and reference:
  - Run `./bin/reloader-image-digest.sh print` or `verify` after `helm dependency build`.
  - The `verify` command exits 2 if Docker cannot access the registry.

#### CI and tests

- Jenkins runs `verify` after Helm chart tests. Release builds fail if verification fails; other builds may mark only the digest-check stage as unstable.
- Test overall integration with `helm/secrets-provider/tests/test-schema`, `tests/test-unit`, or `bin/test_helm`.

#### Pinning overview

- `Chart.lock` pins the exact Helm sub-chart and version.
- `values.yaml` pins the precise container image (by digest and full reference), so tag changes upstream don’t affect which image is deployed.

In short: Helm sub-chart versions are locked in `Chart.lock`, while the container image is locked in `values.yaml`.

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

1. Ensure the relevant `CHANGELOG.md` changes are included under the correct version and update the version release date.

1. Update the version in the [Chart version](helm/secrets-provider/Chart.yaml) file.

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

1. Ensure the internal libraries in the [go.mod](go.mod#L114-L118) still `replace` to `latest`. This may have been auto-updated by a `go mod tidy`.

1. Commit these changes - `Bump version to x.y.z` is an acceptable commit
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

1. Visit the [Red Hat project page](https://connect.redhat.com/component/view/5ec6353078e79e6a879fa296) once the images have been pushed
and manually choose to publish the latest release.
