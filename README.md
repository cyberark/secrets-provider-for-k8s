# Table of Contents

- [Table of Contents](#table-of-contents)
- [CyberArk Secrets Provider for Kubernetes](#cyberark-secrets-provider-for-kubernetes)
  - [Consuming Secrets from CyberArk Secrets Provider](#consuming-secrets-from-cyberark-secrets-provider)
  - [Deployment Modes](#deployment-modes)
  - [Supported Services](#supported-services)
  - [Supported Platforms](#supported-platforms)
  - [Using secrets-provider-for-k8s with Conjur Open Source](#using-secrets-provider-for-k8s-with-conjur-open-source)
  - [Methods for Configuring CyberArk Secrets Provider](#methods-for-configuring-cyberark-secrets-provider)
  - [Retry Behavior](#retry-behavior)
  - [Standalone Mode](#standalone-mode)
  - [Label-based Secret Management](#label-based-secret-management)
    - [Label-based Secret Groups and Templates](#label-based-secret-groups-and-templates)
    - [Sanitization Behavior in Label-based Mode](#sanitization-behavior-in-label-based-mode)
  - [Automatic Application Restart (Reloader)](#automatic-application-restart-reloader)
  - [Enabling Tracing](#enabling-tracing)
- [Releases](#releases)
  - [Stable release definition](#stable-release-definition)
- [Builds](#builds)
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
of four modes:

| Mode | Deploy with Helm | K8s Secrets | Push-to-file | Multiple Pods (provider scope) | Multiple Namespaces | Rotation |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Init container | No | Yes | Yes | Pod-scoped | No | No |
| Sidecar container | No | Yes | Yes | Pod-scoped | No | Yes |
| Kubernetes Job (application mode) | Yes | Yes | No | Namespace-scoped | No | No |
| Kubernetes Deployment (standalone mode) | Yes | Yes | No | Cluster-scoped | Yes | Yes |

__NOTE: Application and standalone modes are scoped more broadly, meaning multiple pods/namespaces may receive secrets from the same Secrets Provider instance, and therefore the same host identity in Secrets Manager policy. While convenient, this should be treated as a security tradeoff which makes auditing and RBAC more complex.__

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

- **Using the [Secrets Provider Helm chart](helm) (Standalone/Application Modes Only)**
  If you are using the Secrets Provider in standalone or application modes, then
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

The default retry count depends on container mode:

- **Standalone mode**: Unlimited retries (`-1`) by default. The provider keeps retrying until Conjur is available or configuration issues have been resolved. The readiness probe keeps the pod out of service until secrets are successfully provisioned.
- **Init, application, and sidecar modes**: Limited to 5 retries by default (`DefaultRetryCountLimit`).

- To change the retry behavior via environment variable, set `RETRY_COUNT_LIMIT` to an integer value.
- To change the retry behavior via Pod annotation, set `conjur.org/retry-count-limit` to an integer value.
- To configure via Helm, set `environment.conjur.retryCountLimit` in the chart values.

Note: set the retry count to `-1` to indicate "unlimited" retries. Retries will continue indefinitely until success or process termination.


## Standalone Mode

For full standalone Helm deployment guidance, including benefits, tradeoffs, install
examples, and key flags, see [STANDALONE.md](STANDALONE.md).

## Label-based Secret Management

Beginning with v1.8.0, when running in Kubernetes Secrets mode, the Secrets
Provider can automatically detect which secrets it should manage by checking for
the label `conjur.org/managed-by-provider: true` on Kubernetes secrets in the
configured namespace set. In a single namespace deployment this is the current
namespace; in standalone mode with a namespace allowlist this applies to those
allowed namespaces. If a secret has this label with the value `true`, the Secrets
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
> | Self-Hosted            | TBD     |
> | Conjur OSS             | TBD     |
> 
> Without this API, a Kubernetes secret which is improperly configured or lacking
> necessary Secrets Manager privileges could cause the Secrets Provider to fail
> to update all secrets until resolved.


When using label-based secrets, if the Secrets Provider is running continuously
as either a standalone deployment or a sidecar container, and has `watch` privileges
on secrets within the namespace, it will detect when a labeled Kubernetes secret
is added or updated and immediately provision the secret.

### Label-based Secret Groups and Templates

In label-based mode, a managed Kubernetes Secret can also include secret-group
annotations and custom templates (`conjur.org/conjur-secrets.<group>` and
`conjur.org/secret-file-template.<group>`) to render computed values directly into
the Kubernetes Secret data key named `<group>`.

Example:

```yaml
metadata:
  labels:
    conjur.org/managed-by-provider: "true"
  annotations:
    conjur.org/conjur-secrets.db: |
      - db-user: secrets/team/db/user
      - db-pass: secrets/team/db/password
      - db-host: secrets/team/db/host
      - db-port: secrets/team/db/port
      - db-name: secrets/team/db/name
    conjur.org/secret-file-template.db: |
      postgres://{{ secret "db-user" }}:{{ secret "db-pass" }}@{{ secret "db-host" }}:{{ secret "db-port" }}/{{ secret "db-name" }}
stringData:
  conjur-map: |
    DB_HOST: secrets/team/db/host
```

With this configuration, the provider updates both:

- `DB_HOST` (from `conjur-map`) as a normal managed secret entry, and
- `db` (from template rendering) as a computed entry in the same Kubernetes Secret.

This behavior is mirrored from Push-to-File mode, and includes the ability to use inline template functions
such as b64enc and b64dec, i.e. `{{ secret "<alias>" | b64enc }}` or `{{ secret "<alias>" | b64dec }}`.

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


## Automatic Application Restart (Reloader)

When using the Secrets Provider Helm chart, you can enable automatic rolling restarts of application
Deployments and StatefulSets whenever a Kubernetes Secret is updated by the Secrets Provider.
This is powered by [Reloader](https://github.com/stakater/Reloader), an open-source project by
[Stakater](https://github.com/stakater), included as an optional subchart.

Enable Reloader with `autoReload.enabled: true` in `values.yaml`, then add the Conjur restart
annotations on your application Deployments or StatefulSets (`conjur.org/automatic-restart` or
`conjur.org/restart-secrets`). By default **`reloader.reloader.watchGlobally` is `false`**, so
Reloader watches Secrets in the release namespace only; if `standalone.namespaceAllowlist` is `*`
or lists multiple namespaces, the chart requires **`watchGlobally: true`** so Reloader can cover
those namespaces. Standalone mode is the usual choice when you want the Secrets Provider to keep
syncing Conjur into Kubernetes Secrets so Reloader can react; copy-paste YAML, annotation behavior,
RBAC notes, digest pinning, and troubleshooting are all in the
[Reloader Integration Guide](RELOADER.md).


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
