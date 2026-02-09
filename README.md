# Table of Contents

- [Table of Contents](#table-of-contents)
- [CyberArk Secrets Provider for Kubernetes](#cyberark-secrets-provider-for-kubernetes)
  - [Supported services](#supported-services)
  - [Using This Project With Conjur Open Source](#using-secrets-provider-for-k8s-with-conjur-open-source)
  - [Methods for Configuring Secrets Provider](#methods-for-configuring-cyberark-secrets-provider)
  - [Enabling Tracing](#enabling-tracing)
- [Releases](#releases)
  - [Stable release definition](#stable-release-definition)
- [Development](#development)
- [Documentation](#documentation)
- [Community](#community)
- [License](#license)

# CyberArk Secrets Provider for Kubernetes

The CyberArk Secrets Provider for Kubernetes provides Kubernetes-based
applications with access to secrets that are stored and managed in CyberArk Secrets Manager.

## Consuming Secrets from CyberArk Secrets Provider

Using the CyberArk Secrets Provider, your applications can easily consume
secrets that have been retrieved from Secrets Manager in one of two ways:

- **Using Kubernetes Secrets:** The Secrets Provider can populate Kubernetes
  Secrets with secrets stored in Secrets Manager. This is sometimes referred to as
  **"K8s Secrets"** mode.
- **Using Secrets files:** The Secrets Provider can generate initialization or
  credentials files for your application based on secrets retrieved from
  Secrets Manager, and it can write those files to a volume that is shared with your
  application container. This is referred to as the Secrets Provider
  **"Push to File"** mode. For more information, see the
  [Secrets Provider Push-to-File guide](PUSH_TO_FILE.md).

## Deployment Modes

The Secrets Provider can be deployed into your Kubernetes cluster in one
of two modes:

- **As an init container:** The Secrets Provider can be deployed as a
  Kubernetes init container for each of your application Pods that requires
  secrets to be retrieved from Secrets Manager. This configuration allows you to employ
  Secrets Manager policy that authorizes access to Secrets Manager secrets on a
  per-application-Pod basis.

- **As a standalone application container (Kubernetes Job):**
  The Secrets Provider can be deployed as a separate, application container
  that runs to completion as part of a Kubernetes Job. In this mode, the
  Secrets Provider can support delivery of Secrets Manager secrets to multiple
  application Pods. In this mode, you would use Secrets Manager policy that authorizes
  access to Secrets Manager secrets on a per-Secrets-Provider basis.

  The [Secrets Provider Helm chart](helm) can be used to deploy the
  Secrets Provider in standalone application mode.

- **As a sidecar to enable [secrets rotation.](ROTATION.md)**

__NOTE: If you are using the Secrets Provider "Push to file" mode, the
  Secrets Provider must be deployed as an init or sidecar container, since these modes
  makes use of shared volumes to deliver secrets to an application.__

## Supported Services
- Secrets Manager, Self-Hosted 11.1+

- Conjur Open Source v1.4.2+

## Supported Platforms
- GKE

- K8s 1.11+

- Openshift v4.6-v4.8 _*(Secrets Manager, Self-Hosted only)*_

## Using secrets-provider-for-k8s with Conjur Open Source 

Are you using this project with [Conjur Open Source](https://github.com/cyberark/conjur)? Then we 
**strongly** recommend choosing the version of this project to use from the latest [Conjur OSS 
suite release](https://docs.conjur.org/Latest/en/Content/Overview/Conjur-OSS-Suite-Overview.html). 
Conjur maintainers perform additional testing on the suite release versions to ensure 
compatibility. When possible, upgrade your Conjur version to match the 
[latest suite release](https://docs.conjur.org/Latest/en/Content/ReleaseNotes/ConjurOSS-suite-RN.htm); 
when using integrations, choose the latest suite release that matches your Conjur version. For any 
questions, please contact us on [Discourse](https://discuss.cyberarkcommons.org/c/conjur/5).

## Methods for Configuring CyberArk Secrets Provider

There are several methods available for configuring the  CyberArk Secrets
Provider:

- **Using Pod Environment Variables:** The Secrets Provider can be configured
  by setting environment variables in a Pod manifest. To see a description of
  the Secrets Provider environment variables, and an example manifest in the
  [Set up Secrets Provider as an Init Container](https://docs.conjur.org/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ic.htm#SetupSecretsProviderasaninitcontainer)
  section of the Secrets Provider documentation (expand the collapsible
  section in Step 6 of this guide to see details).

- **Using Pod Annotations:** The Secrets Provider can be configured by setting
  [Pod Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)
  in a Pod manifest. For details on how Annotations can be used to configure
  the Secrets Provider, see the
  [Secrets Provider Push-to-File guide](PUSH_TO_FILE.md).

- **Using the [Secrets Provider Helm chart](helm) (Standalone Application Mode Only)**
  If you are using the Secrets Provider in standalone application mode, then
  you can configure the Secrets Provider by setting Helm chart values and
  deploying Secrets Provider using the [Secrets Provider Helm chart](helm).

Some notes about the different configuration methods:

1. For a setting that can be configured either by Pod Annotation or by
   environment variable, a Pod Annotation configuration takes precedence
   over the corresponding environment variable configuration.
1. If you are using the Secrets Provider in Push-to-File mode, then the
   Secrets Provider must be configured via Pod Annotations.
1. If you are using the Secrets Provider in Kubernetes Secrets mode, it
   is recommended that you use environment variable settings to configure
   the Secrets Provider.

## Retry Behavior

By default, the Secrets Provider will retry failed Conjur retrieval attempts up to a limited number of times. The default retry count is defined in the code as `DefaultRetryCountLimit` (currently `5`).

- To change the retry behavior via environment variable, set `RETRY_COUNT_LIMIT` to an integer value.
- To change the retry behavior via Pod annotation, set `conjur.org/retry-count-limit` to an integer value.
- To configure via Helm, set `environment.conjur.retryCountLimit` in the chart values.

Note: set the retry count to `-1` to indicate "unlimited" retries. Retries will continue indefinitely until success or process termination.


## Label-based Secret Management

Beginning with v1.8.0, when running in Kubernetes Secrets mode, the Secrets
Provider can automatically detect which secrets it should manage by checking for
the label `conjur.org/managed-by-provider: true` on Kubernetes secrets in the
same namespace. If a secret has this label with the value `true`, the Secrets
Provider will automatically manage it.

This configuration only applies when both the `K8S_SECRETS` environment variable
and the `conjur.org/k8s-secrets` annotation are not set. It requires `list`
privileges on secrets within the namespace.

> **Note:** It is strongly recommended to use this feature only with Secrets Manager
> versions that support the V2 Batch Retrieval API:
>
> | Secrets Manager Flavor | Version |
> |------------------------|---------|
> | SaaS                   | Latest  |
> | Self-Hosted            | 14.0+   |
> | Conjur OSS             | 1.25.0+ |
> 
> Without this API, a Kubernetes secret which is improperly configured or lacking
> necessary Secrets Manager privileges could cause the Secrets Provider to fail
> to update all secrets until resolved.


When using label-based secrets, if the Secrets Provider is running continuously
(as a sidecar container) and has `watch` privileges on secrets within the
namespace, it will detect when a labeled Kubernetes secret is added or updated
and immediately provision the secret.

### Sanitization Behavior in Label-based Mode

The Secrets Provider's behavior when keys are removed from the `conjur-map` depends
on the `conjur.org/remove-deleted-secrets-enabled` annotation (or `REMOVE_DELETED_SECRETS`
environment variable), which defaults to `true`.

**When sanitization is enabled (`true`, default):**
- If a key is removed from the `conjur-map`, the corresponding application secret
  will be deleted from the Kubernetes Secret's data.
- If the entire `conjur-map` is cleared or removed, all Conjur-managed application
  secrets will be deleted from the Kubernetes Secret.

**When sanitization is disabled (`false`):**
- Keys removed from the `conjur-map` will remain in the Kubernetes Secret's data.
- The Secrets Provider will not automatically clean up stale secrets.

> **Warning:** When using label-based mode with sanitization enabled, if you move a
> secret from the `conjur-map` to the Kubernetes Secret's `data` field directly, the
> application secret will be removed on the first update after the key is removed from
> the `conjur-map`. This occurs because the Secrets Provider treats it as a stale
> Conjur-managed secret. This behavior only happens during the first update after the
> key is removed from the `conjur-map`.
>
> **To avoid this:**
> - Set `conjur.org/remove-deleted-secrets-enabled: "false"` before removing the key
>   from the `conjur-map`, or
> - If the secret has already been removed, reapply the Kubernetes Secret with the
>   correct data to restore the value.


## Enabling Tracing

Tracing of CyberArk Secrets Provider for Kubernetes is available using the [OpenTelemetry](https://opentelemetry.io/) standard.
Tracing is disabled by default. You can enable tracing using either Pod Annotations or environment variables.
To enable traces appended to the init container's logs, add the annoation `conjur.org/log-traces: true`
to the Pod manifest, or set the `LOG_TRACES` environment variable to `true`.
To instead export the traces to a Jaeger server, use the following annotation:
`conjur.org/jaeger-collector-url: http://<jaeger-collector-host>/api/traces` or use the `JAEGER_COLLECTOR_URL` environment variable.
Traces will include errors to assist in troubleshooting.

# Releases

The primary source of CyberArk Secrets Provider for Kubernetes releases is our [Dockerhub](https://hub.docker.com/repository/docker/cyberark/secrets-provider-for-k8s).

When we release a version, we push the following images to Dockerhub:
1. Latest
1. Major.Minor.Build
1. Major.Minor
1. Major

We also push the Major.Minor.Build image to our [Red Hat registry](https://catalog.redhat.com/software/containers/cyberark/secrets-provider-for-k8s/5ee814f0ac3db90370949cf0).

# Builds

We push the following tags to Dockerhub:

*Latest* - on every release the latest tag will be updated (_cyberark/secrets-provider-for-k8s:latest_). This tag means the Secrets Provider for Kubernetes meets the stability criteria detailed in the following section.
 
*Semver* - on every release a Semver tag will be pushed (_cyberark/secrets-provider-for-k8s:1.1.0_). This tag means the Secrets Provider for Kubernetes meets the stability criteria detailed in the following section.

## Stable release definition

The CyberArk Secrets Provider for Kubernetes is considered stable when it meets the core acceptance criteria:

- Documentation exists that clearly explains how to set up and use the provider and includes troubleshooting information to resolve common issues.
- A suite of tests exist that provides excellent code coverage and possible use cases.
- The CyberArk Secrets Provider for Kubernetes has had a security review and all known high and critical issues have been addressed.
Any low or medium issues that have not been addressed have been logged in the GitHub issue backlog with a label of the form `security/X`
- The CyberArk Secrets Provider for Kubernetes is easy to setup.
- The CyberArk Secrets Provider for Kubernetes is clear about known limitations and bugs, if they exist.

# Development

We welcome contributions of all kinds to CyberArk Secrets Provider for Kubernetes. For instructions on
how to get started and descriptions of our development workflows, see our [contributing guide](CONTRIBUTING.md).

# Documentation
You can find official documentation on [our site](https://docs.conjur.org/Latest/en/Content/Integrations/k8s-ocp/cjr-secrets-provider-lp.htm).

# Community

Interested in checking out more of our open source projects? See our [open source repository](https://github.com/cyberark/)!

# License

The CyberArk Secrets Provider for Kubernetes is licensed under the Apache License 2.0 - see [`LICENSE`](LICENSE.md) for more details.
