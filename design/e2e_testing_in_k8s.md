# Solution Design - E2E Testing in Kubernetes

## Table of Contents
- [Solution Design - E2E Testing in Kubernetes](#solution-design---e2e-testing-in-kubernetes)
  * [Table of Contents](#table-of-contents)
  * [Useful links](#useful-links)
  * [Background](#background)
  * [Issue description](#issue-description)
  * [Solution](#solution)
    + [Design](#design)
    + [Creating or Interacting with a Kubernetes Cluster Test Environment](#creating-or-interacting-with-a-kubernetes-cluster-test-environment)
    + [Creating or Interacting with Kubernetes Config](#creating-or-interacting-with-kubernetes-config)
    + [Using the Config Object](#using-the-config-object)
    + [Helpers and Utils](#helpers-and-utils)
    + [Managing Kubernetes Resources](#managing-kubernetes-resources)
    + [Backwards compatibility](#backwards-compatibility)
    + [Affected Components](#affected-components)
  * [Documentation](#documentation)
  * [Open questions](#open-questions)
  * [Implementation plan](#implementation-plan)

## Useful links
[//]: # (Add links that may be useful for the reader)
- [Kubernetes e2e-framework](https://github.com/kubernetes-sigs/e2e-framework)
- [Kubernetes e2e-framework examples](https://github.com/kubernetes-sigs/e2e-framework/tree/main/examples)
- [Secrets Provider POC branch](https://github.com/cyberark/secrets-provider-for-k8s/pull/522)
- [Secrets Provider Bash Test Folder](https://github.com/cyberark/secrets-provider-for-k8s/tree/main/deploy/test/test_cases)


## Background
[//]: # (Give relevant background for the designed feature. What is the motivation for this solution?)
Our current scheme for doing end-to-end (E2E) testing of Kubernetes projects involves complex bash scripts which run in CI environments (mainly GKE and Openshift). This utilizes a lot of resources on the infrastructure side and can be subject to long provisioning times of clusters, and occasionally flaky builds. It also ties up pipelines for prolonged periods while the multiple platforms and configurations are tested.

There has been discussion of only running these lengthy, often platform-specific E2E tests on nightly or weekly builds so that developers can implement and test changes more effectively on branch. However it is still important to be able to run tests against an actual cluster to augment our unit testing strategy.

## Issue description
[//]: # (Elaborate on the issue you are writing a solution for)
For the most part we are running our current E2E tests run on every commit. This results pipelines that can take anywhere from 30 minutes to 2 hours depending on the project and which platforms are tested. There is also not currently a supported way to run these same E2E tests locally, so a small change that passes unit testing can take a long time to debug if it is resulting in one or my E2E test failures.

Another motivator for this improvement is the ability to move away from bash-driven testing which can be more difficult to debug and implement. Instead we should favor using Golang tests where possible. This will make it simpler to toggle which tests run on branch and provide better visibility into failing tests.

## Solution
Most of our projects contain a fully working development environment running in either Docker Desktop or a KinD cluster, both of which can be leveraged to run basic E2E tests on vanilla Kubernetes. While there is likely room for extensive refactoring in terms of how we standup and configure these Kubernetes test environments both locally and in the cloud, we first want to address the actual testing. To that end, we will continue to use available bash scripts and repos which facilitate standing up a local Conjur cluster with the necessary tooling. In the case of Secrets Provider, it relies heavily on [kubernetes-conjur-deploy](https://github.com/cyberark/kubernetes-conjur-deploy), along with a series of bash scripts ([with the entrypoint: ./start](../bin/start)) in order to create a dev/test environment with Secrets Provider configured to load Conjur secrets.

The first wave of improvements should focus on using an available Golang testing framework to interact with Kubernetes resources in a running cluster and allow us to handle commands and output so that we may implement our test assertions. The [e2e-framework](https://github.com/kubernetes-sigs/e2e-framework) library is designed to provide tools for doing basic cluster configuration, inspecting Kubernetes resources, and running kubectl commands. This should be sufficient for most of our e2e tests, which primarily rely on the ability to run `kubectl exec` commands against the appropriate pods and containers in order to verify expected values and conditions. In the case of e2e-framework, such a command in Go may look like:
```
client.Resources().ExecInPod(context.TODO(), namespaceName, podName, containerName, command, stdout, stderr)
```

This makes it simple enough to compare the expected output of a kubectl command (stdout) with some value that we setup in our test case. You can view an example [POC branch for Secrets Provider here.](https://github.com/cyberark/secrets-provider-for-k8s/pull/522)

### Design
[//]: # (Add any diagrams, charts and explanations about the design aspect of the solution. Elaborate also about the expected user experience for the feature)

As mentioned, the e2e-framework this solution is based on provides a series of helpful functions for interacting directly with the Kubernetes API server in Golang. This should allow us to write tests which are agnostic of differences in CLI functionality i.e. between `kubectl` and `oc` (Openshift). Below is a list of potentially helpful functions and examples provided by the e2e-framework:

### Creating or Interacting with a Kubernetes Cluster Test Environment
For full details, see: https://pkg.go.dev/sigs.k8s.io/e2e-framework@v0.2.0/pkg/envfuncs

[The POC](https://github.com/cyberark/secrets-provider-for-k8s/pull/522)  relies on the existing start script, which deploys the local cluster on Kubernetes in Docker Desktop, but I could see us making heavier use of these capabilities down the road as we continue to develop our Kubernetes testing strategy. The e2e-framework provides fairly robust support for KinD, and there is a follow-up story to investigate using KinD more in E2E/integration testing so a handful of these functions may prove useful down the road:
- CreateKindCluster()
- CreateKindClusterWithConfig()
- CreateNamespace()
- DeleteNamespace()
- DestroyKindCluster()
- ExportKindClusterLogs()
- GetKindClusterFromContext()
- LoadDockerImageToCluster()
- LoadImageArchiveToCluster()
- SetupCRDs()
- TeardownCRDs()

Example:
Deleting namespaces in the test environment to cleanup between or after test executions.
```
testenv.Finish(
  envfuncs.DeleteNamespace("local-secrets-provider"),
  envfuncs.DeleteNamespace("local-conjur"),
)
```

### Creating or Interacting with Kubernetes Config
For full details, see: https://pkg.go.dev/sigs.k8s.io/e2e-framework@v0.2.0/klient/conf

Example:
Creating a config object based on a local KubeConfig file.
```
path := conf.ResolveKubeConfigFile()
cfg := envconf.NewWithKubeConfig(path)
```

### Using the Config Object
For full details, see: https://pkg.go.dev/sigs.k8s.io/e2e-framework@v0.2.0/pkg/envconf

Example:
Creating a test environment and client object to perform future operations and testing against the Kubernetes API.
```
cfg := envconf.NewWithKubeConfig(path)
testenv := env.NewWithConfig(cfg)
client := cfg.Client()
```

### Helpers and Utils
For full details, see: https://pkg.go.dev/sigs.k8s.io/e2e-framework@v0.2.0/klient/wait

Example:
Waiting for the Secrets Provider Pod to be ready before beginning test execution.
```
pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName}}
wait.For(conditions.New(k8sClient.Resources(SecretsProviderNamespace)).PodReady(k8s.Object(&pod)), wait.WithTimeout(time.Minute*1))
```

### Managing Kubernetes Resources
For full details, see: https://pkg.go.dev/sigs.k8s.io/e2e-framework@v0.2.0/klient/k8s/resources#Resources

These functions will likely be the most used in test writing, assuming we have a working cluster to test against. Below is subset of the available CRUD Operations against Kubernetes Resources which are invoked via `client.Resources(namespace).FunctionCall(...)`:
| Function                   | Description                                                            |
|----------------------------|------------------------------------------------------------------------|
| Create                     | Creates a new resource                                                 |
| Delete                     | Deletes an existing resource                                           |
| Get                        | Retrieves information about a specific resource                        |
| GetConfig                  | Retrieves the configuration for a specific resource                    |
| Update                     | Updates an existing resource                                           |
| List                       | Lists resources based on specific criteria                             |
| ExecInPod                  | Executes a command in a pod                                            |
| Patch                      | Applies a patch to modify an existing resource                         |
| Watch                      | Watches for changes to resources                                       |
| GetControllerRuntimeClient | Retrieves the Controller Runtime client for interacting with resources |
| GetScheme                  | Retrieves the scheme for the resources                                 |
| Label                      | Labels a resource with specified labels                                |

Examples:
Populates a list of pods in the K8s namespace
```
var pods v1.PodList
client.Resources(SecretsProviderNamespace).List(context.TODO(), &pods)
```
Exec a command in a pod
```
command := []string{"conjur", "variable", "set", "-i", varId, "-v", value}
client.Resources(ConjurNamespace).ExecInPod(context.TODO(), ConjurNamespace, podName, containerName, command, stdout, stderr)
```

### Backwards compatibility
An important aspect of our E2E testing is that it allows us to validate the behavior of our product components on different Kubernetes platforms. The e2e-framework library provides a Kubernetes client which should be platform-agnostic, so long as the platform adheres to the same API specifications and conventions as the vanilla Kubernetes API. This proposed solution includes a follow-up task to investigate and implement running the Golang-based e2e tests in CI, which will allow us to validate this assumption.

### Affected Components
While this document and research has focused mainly on Secrets Provider, [conjur-authn-k8s-client](https://github.com/cyberark/conjur-authn-k8s-client) also relies heavily on bash scripts to run E2E tests.

There may be impacts to the pipelines of both components when these testing changes are implemented, although if anything it should speed up developer feedback by allowing E2E tests to run locally and time-consuming platform-specific tests can be updated to run only on occasion in CI.


## Documentation
This change will not include any user-facing changes, and will therefore only require updates to the CONTRIBUTING guidelines and other developer focused docs in the repo.

## Open questions
- Assuming we can run an E2E test suite locally, how do we want to update our pipelines and timing of the platform-specific E2E tests?
- Is it a primary goal to move all E2E tests to this scheme, and have them be runnable both locally and in CI across different Kubernetes platforms?

## Implementation plan
Secrets Provider:
1. Create a test structure for implementing Golang based E2E tests (3 pts)
    1. Decide on a location within the repo separate from our other tests or use Go tags to differentiate e2e from regular tests
    1. Implement a basic [TestMain](https://pkg.go.dev/sigs.k8s.io/e2e-framework@v0.2.0/klient/k8s/resources#Resources) function to define package-wide testing steps and configuration.
    1. If the TestMain `Setup` is using existing start scripts, validate that it can be run locally without any major modifications needed to the code/configuration by developers.
    1. Validate cleanup automatically happens after running a dummy test.
1. Migrate [TEST_IDs 1.1 - 1.7](../deploy/test/test_cases/) to use the strategy. (3 pts)
    1. Define features (see: [e2e-framework examples](https://github.com/kubernetes-sigs/e2e-framework#define-a-test-function) or the [POC branch]())
    1. Create util/helper functions as needed for recurring tasks, i.e. updating Conjur secrets, running commands in Secrets Provider pod, etc.
    1. Ensure that they run successfully, either when run directly via `go test` or produce a wrapper script if necessary
1. Migrate [TEST_IDs 2 - 16](../deploy/test/test_cases/) to use the strategy. (2 pts)
1. Migrate helm tests [TEST_IDs 17 - 26](../deploy/test/test_cases/) to use the strategy.
    1. Make any needed changes to the dev environment to allow helm deployments
1. Migrate [TEST_IDs 27 - 29](../deploy/test/test_cases/) to use the strategy. (2 pts)
    1. Ensure that switching between deployment manifests is possible during for each test feature
1. Optional spike - Investigate compatibility with conjur-authn-k8s-client E2E tests (2 pts)
1. Optional spike - Investigate migrating local K8s testing to KinD over docker-desktop (which is the default in Secrets Provider) (3 pts)
1. Optional spike - Investigate running these Golang based E2E tests in CI as a full replacement of existing E2E tests (3 pts)
