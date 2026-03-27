# Kubernetes Secrets Provider Reloader Integration

This document describes how the Kubernetes Reloader subchart integrates with the Secrets Provider and the supported deployment modes.

## Table of Contents

- [Open Source Attribution](#open-source-attribution)
- [Overview](#overview)
- [Quick Start](#quick-start)
- [Supported Deployment Modes](#supported-deployment-modes)
  - [Application Mode (Job-based)](#application-mode-job-based)
  - [Standalone Mode (Deployment-based)](#standalone-mode-deployment-based)
- [Annotating Workloads](#annotating-workloads)
- [Complete Workload Example](#complete-workload-example)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Open Source Attribution

The automatic restart functionality in this chart is powered by
[Reloader](https://github.com/stakater/Reloader), an open-source project maintained
by [Stakater](https://github.com/stakater). Reloader is licensed under the
[Apache License 2.0](https://github.com/stakater/Reloader/blob/master/LICENSE).

CyberArk includes Reloader as an **optional subchart** in the Secrets Provider Helm chart. The
Reloader source code and its own documentation can be found at
[https://github.com/stakater/Reloader](https://github.com/stakater/Reloader). We are grateful to
the Stakater team and the open-source community for this component.

## Overview

[Kubernetes Reloader](https://github.com/stakater/Reloader) is an optional component that watches for changes in Kubernetes Secrets and ConfigMaps, and automatically restarts pods when their referenced secrets change. This is useful for applications that read secret values at startup and need to pick up changes without manual intervention.

The Secrets Provider Helm chart includes Reloader as a **subchart** that can be optionally enabled via the `autoReload.enabled` value. When enabled, Reloader watches for changes in K8s Secrets that applications consume and triggers rolling restarts.

## Quick Start

Reloader is most often paired with **Standalone** mode: the Secrets Provider runs as a Deployment,
periodically syncs Conjur into Kubernetes Secrets, and Reloader restarts annotated workloads when
those secrets change. The steps below use that path. ([Application mode](#application-mode-job-based)
is an alternative when the Secrets Provider runs as a one-shot Job.)

To enable automatic application restarts when secrets change:

**Step 1:** In your `values.yaml`, set **Standalone** mode and enable Reloader:

```yaml
containerMode: standalone

standalone:
  secretsRefreshEnabled: true
  secretsRefreshInterval: "5m"  # how often Conjur is checked; tune as needed

autoReload:
  enabled: true
```

**Step 2:** Label each Kubernetes Secret that Standalone mode should manage so the provider can
discover it (see [K8s Secret Labels](#k8s-secret-labels) for details):

```yaml
metadata:
  name: db-credentials
  labels:
    conjur.org/managed-by-provider: "true"
```

**Step 3:** Annotate your application Deployment (or StatefulSet) with Reloader's auto-restart annotation:

```yaml
metadata:
  annotations:
    conjur.org/automatic-restart: "true"
```

**Step 4:** Ensure your application's Deployment references that Kubernetes Secret by name in `envFrom`, `env.valueFrom`, or a `volume`:

```yaml
envFrom:
  - secretRef:
      name: db-credentials
```

That's it. On each refresh interval, the Secrets Provider updates `db-credentials` when Conjur
values change; Reloader detects the Kubernetes Secret update and performs a rolling restart of the
annotated Deployment.

## Supported Deployment Modes

### Application Mode (Job-based)

In Application mode, the Secrets Provider runs as a **Kubernetes Job** that completes after fetching secrets once. This is the default mode.

> **Important:** In Application mode, the Secrets Provider Job runs once and exits. Reloader will
> only trigger a restart when the K8s Secret is updated — which requires the Job to run again
> (e.g. via a CronJob). If you need continuous automatic restarts on secret rotation, use
> [Standalone mode](#standalone-mode-deployment-based) instead.

#### How It Works

1. Secrets Provider Job runs and fetches secrets from Conjur
2. Job populates configured Kubernetes Secrets with the fetched values
3. Job completes (runs to completion)
4. Applications consume the K8s Secrets
5. **External systems** must update the K8s Secrets when Conjur secrets change
   - This could be another Secrets Provider Job running on a schedule (via CronJob)
   - Or a monitoring/sync system external to this deployment
6. Reloader detects the K8s Secret changes and restarts application pods

> **Note:** Reloader restarts Deployments and StatefulSets only — it does not re-trigger
> completed Jobs. Developers using Application mode should not expect their Job to be
> automatically re-run when a secret changes.

#### Configuration

```yaml
# values.yaml
containerMode: application  # Default

autoReload:
  enabled: true
```

#### Application Annotation

Applications must be annotated for Reloader to restart them when secrets change. For the
annotation reference table and copy-paste examples, see **[Workload Annotations](#workload-annotations)**
under [Configuration](#configuration). [Annotating Workloads](#annotating-workloads) (later in this
document) explains how Reloader matches secrets to pods.

```yaml
metadata:
  annotations:
    conjur.org/automatic-restart: "true"
```

### Standalone Mode (Deployment-based)

In Standalone mode, the Secrets Provider runs as a **Kubernetes Deployment** and continuously monitors Conjur for secret changes. This mode is suited for environments that require automatic secret rotation.

#### How It Works

1. Secrets Provider Deployment starts and uses **label-based discovery** to find K8s Secrets
   - Secrets must have labels matching the secrets-provider configuration
   - The provider discovers secrets across the configured namespaces (via `namespaceAllowlist`)
2. Provider automatically **refreshes secrets periodically** (default 5 minutes)
   - Interval is configurable via `standalone.secretsRefreshInterval`
3. When Conjur secrets change, the provider automatically updates the K8s Secrets
4. Reloader detects the K8s Secret changes and restarts application pods
5. Applications pick up the new secret values from the restarted pods

#### Configuration

```yaml
# values.yaml
containerMode: standalone

standalone:
  # Namespace allowlist for label-based discovery
  # Empty string = current namespace only
  # "*" = all namespaces (requires ClusterRole)
  # "ns1,ns2,ns3" = specific namespaces (requires ClusterRole if multiple)
  namespaceAllowlist: ""
  
  secretsRefreshEnabled: true
  secretsRefreshInterval: "5m"  # Configurable interval
  removeDeletedSecretsEnabled: true

autoReload:
  enabled: true
```

#### K8s Secret Labels

In Standalone mode, secrets are discovered via labels rather than explicit configuration. Secrets should be labeled:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  labels:
    conjur.org/managed-by-provider: "true"
stringData:
  conjur-map: |
    username: path/to/username
    password: path/to/password
```

#### Multi-Namespace Support

For multi-namespace scenarios, configure the `namespaceAllowlist`:

```yaml
standalone:
  namespaceAllowlist: "default,app-namespace,service-namespace"
```

**RBAC Requirement:** When using multiple namespaces, ensure the RBAC configuration includes a **ClusterRole** with cluster-wide secret access:

```yaml
rbac:
  create: true
  clusterRoleName: secrets-provider-cluster-role
  clusterRoleBindingName: secrets-provider-cluster-role-binding
```

> ⚠️ **Security note:** The allowlist defines how widely the Secrets Provider may read Kubernetes
> Secrets (typically through a ClusterRole). If the ServiceAccount is compromised, an attacker can
> read every Secret within that scope.
>
> Setting **`namespaceAllowlist: "*"`** expands that scope to **all namespaces**—roughly equivalent
> to cluster-wide Secret read for that ServiceAccount. Use it only when a narrower allowlist cannot
> meet your requirements, and treat the decision as a significant security tradeoff.
>
> Prefer the smallest allowlist that fits your policy. Where appropriate, add defense in depth
> (for example NetworkPolicy, Pod Security admission, and audit logging for Secret access). Review
> RBAC periodically, and obtain security sign-off before deploying `*` to production environments
> that require strict separation between teams or data classes.
>
> **Reloader:** With **`reloader.reloader.watchGlobally: false`** (the chart default), Reloader only
> watches Secrets in the release namespace. If you use a multi-namespace allowlist with Reloader,
> set **`watchGlobally: true`** as required by the chart so Reloader’s scope matches the provider’s;
> expect a Reloader **ClusterRole** in that configuration. See [Reloader RBAC](#reloader-rbac).

## Annotating Workloads

The **[Workload Annotations](#workload-annotations)** subsection under [Configuration](#configuration)
lists the two Conjur annotation keys and example YAML. This section describes how Reloader uses
those annotations in practice.

For Reloader to trigger a rolling restart, the workload (Deployment or StatefulSet) must:

1. Be annotated with one of the Conjur restart annotations (see [Application Annotation](#application-annotation) above).
2. Reference the Kubernetes Secret that the Secrets Provider populates — via `envFrom`, `env.valueFrom`, or a mounted `volume`.

Reloader discovers the relationship between a workload and a Kubernetes Secret by inspecting the
workload's spec. If the Pod spec does not directly reference the secret, Reloader will not restart
it even when the secret changes. Ensure that the secret name in the workload spec matches exactly
the secret name managed by the Secrets Provider.

## Complete Workload Example

The following example shows a full end-to-end configuration for Standalone mode with Reloader
enabled. The same pattern applies to Application mode, with the differences noted in the comments.

### `values.yaml` (Secrets Provider Helm chart)

```yaml
containerMode: standalone  # Use "application" for Job-based mode

standalone:
  secretsRefreshEnabled: true
  secretsRefreshInterval: "5m"

autoReload:
  enabled: true

environment:
  conjur:
    account: myConjurAccount
    applianceUrl: https://conjur.example.com
    authnLogin: host/conjur/authn-jwt/k8s/my-app
    sslCertificate:
      name: conjur-cert-config-map
```

### `k8s-secret.yaml` (Kubernetes Secret to be populated by Secrets Provider)

In Standalone mode, secrets are discovered via labels rather than explicit `K8S_SECRETS` config:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  labels:
    conjur.org/managed-by-provider: "true"
type: Opaque
stringData:
  conjur-map: |
    username: secrets/db/username
    password: secrets/db/password
```

### `app-deployment.yaml` (Application Deployment consuming the secret)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  annotations:
    # Restart this Deployment when any referenced Kubernetes Secret changes.
    # Remove this if you want to use conjur.org/restart-secrets for targeted restarts.
    conjur.org/automatic-restart: "true"
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: my-app
          image: my-app:latest
          # Consume secrets via environment variables sourced from the K8s Secret.
          # Reloader uses this reference to associate the Deployment with db-credentials.
          envFrom:
            - secretRef:
                name: db-credentials
```

**Alternative: targeted restart on a specific secret**

If your Deployment references multiple secrets but should only restart when `db-credentials`
changes, use `conjur.org/restart-secrets` instead:

```yaml
metadata:
  annotations:
    conjur.org/restart-secrets: "db-credentials"
```

You can also specify a comma-separated list of secret names:

```yaml
metadata:
  annotations:
    conjur.org/restart-secrets: "db-credentials,api-keys"
```

## Configuration

### Enabling Reloader

Reloader is **disabled by default**. To enable it:

```yaml
autoReload:
  enabled: true
```

### Reloader RBAC

When Reloader is enabled, the subchart installs RBAC for Reloader’s ServiceAccount (controlled by
`reloader.reloader.rbac.enabled` in the Reloader chart values, `true` by default). Reloader must
**watch** Kubernetes Secrets and **patch** workloads (Deployments, DaemonSets, StatefulSets) to
perform rolling restarts. The exact rules are defined in the
[Stakater Reloader Helm chart](https://github.com/stakater/Reloader); this chart sets
`reloader.reloader.ignoreConfigMaps`, `ignoreJobs`, and `ignoreCronJobs` so the controller
focuses on Secrets and core `apps` workloads.

The Stakater chart may also create an additional namespace-scoped **Role** for ConfigMap access
(e.g. `<release>-reloader-metadata-role`); see your rendered manifests for the full set.

#### Watch scope (`watchGlobally`) and the namespace allowlist

In the CyberArk Secrets Provider chart, **`reloader.reloader.watchGlobally` defaults to `false`**
(namespace-scoped Reloader, least privilege). Reloader then uses a namespace **Role** and watches
Secrets **in the Helm release namespace only**. That default differs from consuming the Stakater
chart alone, where cluster-wide Secret watch is common—use this chart’s values when documenting
customer installs.

If **`standalone.namespaceAllowlist`** is **`*`** or a **comma-separated list** (more than one
namespace) **and** `autoReload.enabled` is true, you **must** set **`reloader.reloader.watchGlobally: true`**.
Otherwise Helm fails with a remediation message: Reloader would not be able to see Secrets or
workloads in allowlisted namespaces outside the release namespace. The same rule applies in
Application or Standalone mode whenever that allowlist shape is used.

When **`watchGlobally` is `true`**, Stakater typically creates a **ClusterRole** with cluster-wide
list/watch on Secrets (and matching bindings). When it is **`false`**, Reloader stays on a
namespace **Role**; scope follows the Stakater templates.

At a high level, the main Reloader rules include:

| API area | Purpose |
|----------|---------|
| Core `secrets` | `get`, `list`, `watch` — detect secret changes (within Reloader’s watch scope) |
| `apps` | `get`, `list`, `update`, `patch` on Deployments, DaemonSets, StatefulSets — trigger reloads |
| Core `events` | `create`, `patch` — emit events |
| `coordination.k8s.io` `leases` | When `enableHA` is `true` — leader election between replicas |

To inspect the installed rules after deployment (names depend on your Helm release and any
`fullnameOverride`):

```bash
kubectl get clusterrole,role -n <namespace> | grep -i reloader
kubectl describe clusterrole <helm-release-name>-reloader-role
```

Use `kubectl describe role …` instead when Reloader is running with namespace-scoped RBAC only.

The chart also pins the Reloader image with **`reloader.image.digest`** (and records
**`reloader.imageDigestReference.pullReference`**) for provenance. When you bump the Reloader
subchart or image tag, run **`./bin/reloader-image-digest.sh sync`** as described in
[CONTRIBUTING.md](CONTRIBUTING.md).

### Replicas and High Availability (HA)

By default, Reloader runs as a single replica. For production deployments requiring redundancy,
you can run multiple replicas with leadership election enabled:

The top-level key **`reloader:`** matches the dependency name in the parent `Chart.yaml`. Stakater’s
chart reads its settings under **`reloader:`** in its own `values.yaml`, so values are written as
**`reloader.reloader.*`**: the outer key selects the subchart; the inner key is Stakater’s values
root.

```yaml
reloader:
  reloader:        # Stakater values root (see above)
    deployment:
      replicas: 2
    enableHA: true  # REQUIRED — without this, replicas is silently capped to 1
```

> ⚠️ **Important:** The Reloader subchart silently caps replicas to 1 when `enableHA` is false,
> even if a higher value is specified. Always set `enableHA: true` when running more than one
> replica, or your HA configuration will be ignored without any error or warning.

With **`enableHA: true`**, Reloader uses Kubernetes **Lease** objects for leader election. The
Stakater chart adds the corresponding **`coordination.k8s.io` / `leases`** permissions to Reloader’s
RBAC when HA is enabled. If you customize or trim Reloader’s ClusterRole or Role after install,
dropping Lease access can break HA replicas silently or leave election stuck—keep those rules or
restore them from the chart defaults.

### Workload Annotations

The Secrets Provider Helm chart supports two annotations for controlling automatic restarts:

| Annotation | Behavior |
|---|---|
| `conjur.org/automatic-restart: "true"` | Restart the workload when **any** Kubernetes Secret it references is updated |
| `conjur.org/restart-secrets: "<name>"` | Restart the workload only when the **named** Kubernetes Secret is updated |

Use `conjur.org/automatic-restart` when your Deployment references a single secret or you want any
secret change to trigger a restart. Use `conjur.org/restart-secrets` when you want to scope
restarts to a specific secret (for example, if the Deployment uses multiple secrets but only one is
managed by the Secrets Provider).

```yaml
# Option A: restart on ANY referenced secret change
metadata:
  annotations:
    conjur.org/automatic-restart: "true"

# Option B: restart only when a specific secret changes
metadata:
  annotations:
    conjur.org/restart-secrets: "db-credentials"

# Option C: restart on changes to any of several specific secrets
metadata:
  annotations:
    conjur.org/restart-secrets: "db-credentials,api-keys"
```

## Troubleshooting

### Secrets Provider workload names and kubectl labels

The Secrets Provider **Deployment** or **Job** is named after your Helm release (see `helm list -n
<namespace>`) unless you set `secretsProvider.workloadName` or `secretsProvider.jobName`. Pod
templates use the label **`app=<that same name>`** — not `app.kubernetes.io/name=secrets-provider`.
The examples below use **`<helm-release-name>`** for that workload name. The provider container
name defaults to **`cyberark-secrets-provider-for-k8s`** (`secretsProvider.name` in chart values).

### Access and RBAC

The commands below assume an operator with permission to read workloads,
Secrets, Events, and pod logs in the relevant namespaces. Many of them reveal metadata about secret
rotation, timing, or application configuration; provider logs may include paths or errors that
should stay within a trusted break-glass or support workflow. Grant `kubectl` access to these
resources only to accounts that need it, and treat shared command output like any other sensitive
diagnostic material.

### Reloader is not running or is unhealthy

#### Problem

You do not see Reloader pods, or they are not ready, so nothing will watch Secrets and trigger reloads.

#### Likely causes

Reloader disabled in chart values, install in a different namespace than you are checking, pods crashing on startup, or RBAC stripped after install (for example missing `coordination.k8s.io` Lease rules when using HA—see [Replicas and High Availability (HA)](#replicas-and-high-availability-ha)).

#### How to confirm

The Reloader subchart (Stakater) labels pods with `app.kubernetes.io/name=reloader`.

```bash
kubectl get pods -n <namespace> -l app.kubernetes.io/name=reloader
# If the label is unfamiliar, list by name:
kubectl get pods -n <namespace> | grep reloader
kubectl logs -n <namespace> -l app.kubernetes.io/name=reloader --all-containers
```

#### How to fix

Enable Reloader in Helm values and upgrade the release; resolve errors from Reloader logs; restore Reloader RBAC from chart defaults if the ClusterRole or Role was customized.

### Secret changes but the workload never restarts (annotations)

#### Problem

The Kubernetes Secret is updated (or you expect Reloader to act) but the Deployment or StatefulSet does not roll out.

#### Likely causes

Missing `conjur.org/automatic-restart` or `conjur.org/restart-secrets`, a typo in the secret name in `restart-secrets`, or annotations placed on the wrong object (for example only on the Pod template instead of the workload).

#### How to confirm

Inspect the workload’s metadata (use `statefulset` instead of `deployment` for StatefulSets):

```bash
kubectl get deployment <app-name> -n <namespace> -o yaml | grep -E 'conjur\.org/(automatic-restart|restart-secrets)'
# Optional: full annotations
kubectl get deployment <app-name> -n <namespace> -o jsonpath='{.metadata.annotations}'
```

You should see either `conjur.org/automatic-restart` (for example `"true"`) or `conjur.org/restart-secrets` with the intended secret name(s). If neither is present, Reloader will not restart the workload regardless of how often the Secret changes.

#### How to fix

Add or correct annotations per [Annotating Workloads](#annotating-workloads) and [Workload Annotations](#workload-annotations) above. Ensure `conjur.org/restart-secrets` lists the exact Secret name(s), comma-separated if there are several.

### Workload never restarts: Secret not updating, or Reloader does not see the dependency

#### Problem

Nothing restarts when you expect it to. Reloader only reacts when the Kubernetes **Secret object** changes, and it only ties that Secret to workloads whose Pod spec declares the dependency (`envFrom`, `env.valueFrom`, or volumes). If the Secret’s `resourceVersion` never moves, or the Secret updates but Reloader still does not reload, the failure is usually upstream sync or how the workload references the Secret.

#### Likely causes

- The Secrets Provider is not updating the Secret (often a Standalone provider or connectivity issue), the wrong Secret name or namespace is referenced, or you are comparing against a stale baseline.
- Reloader discovers dependencies from `envFrom`, `env.valueFrom`, and volume mounts. If the app consumes the Secret only indirectly (for example a path not mounted from that Secret), Reloader may not treat the workload as dependent—see [Best Practices](#best-practices).

#### How to confirm

> ⚠️ **Caution:** `kubectl get secret -o yaml` outputs base64-encoded secret values. Avoid sharing this output in support tickets or logs.

```bash
kubectl get secret <secret-name> -o jsonpath='{.metadata.resourceVersion}'
kubectl get events --field-selector involvedObject.name=<secret-name>
kubectl get events --field-selector reason=Reloaded
```

#### How to fix

If the Secret is not updating, follow [Kubernetes Secrets not updating in Standalone mode](#kubernetes-secrets-not-updating-in-standalone-mode) below. If the Secret updates but there is still no `Reloaded` event, check Reloader logs and chart scope (for example namespace vs cluster-wide watches), and ensure the Pod spec declares the Secret in a form Reloader recognizes.

### Kubernetes Secrets not updating in Standalone mode

#### Problem

The provider Deployment runs, but Secrets in the cluster stay stale or missing.

#### Likely causes

Deployment not ready or pods not scheduling, provider errors (Conjur, auth, or config), namespace not on the allowlist, or Secrets missing expected labels.

#### How to confirm

```bash
kubectl get deployment <helm-release-name> -n <namespace> -o wide
kubectl get pods -n <namespace> -l app=<helm-release-name>
kubectl logs -n <namespace> -l app=<helm-release-name> -c cyberark-secrets-provider-for-k8s --tail=100
kubectl get secrets -L conjur.org/managed-by-provider
```

The chart injects the namespace allowlist as **`NAMESPACE_ALLOWLIST`** (from `standalone.namespaceAllowlist` in values):

```bash
kubectl get deployment <helm-release-name> -n <namespace> -o yaml | grep -A 2 NAMESPACE_ALLOWLIST
```

#### How to fix

Fix Deployment readiness (resources, image pull, crashes); correct allowlist and Conjur/auth settings in values; use provider logs to resolve connection and permission errors—for example:

```bash
kubectl logs -n <namespace> -l app=<helm-release-name> -c cyberark-secrets-provider-for-k8s --tail=50 | grep -i "error\|failed\|connection"
```

### Kubernetes Secrets not updating in Application mode

#### Problem

Secrets are never refreshed after the Job completes.

#### Likely causes

In Application mode the Secrets Provider runs **once** and exits; it does not continuously poll Conjur. Stale Secrets are expected unless something else triggers a new run.

#### How to confirm

Check that you are using Application mode (Job-based install) and that no CronJob or other automation re-runs the provider on an interval.

#### How to fix

Run the Secrets Provider Job on a schedule (for example via CronJob), use a separate reconciliation process, or switch to Standalone mode for continuous sync from Conjur into Kubernetes Secrets.

## Best Practices

1. **Only enable Reloader if applications need automatic restarts:**
   Reloader runs a controller that watches Secrets within the scope set in the Reloader chart. In
   this Helm chart **`watchGlobally` defaults to `false`** (release namespace only); set it to
   `true` only when your allowlist or workload layout requires cluster-wide Secret watches, and
   only then together with the chart’s validation for multi-namespace allowlists. Reloader restarts
   **annotated** workloads when a referenced Secret changes. Those watches still add API traffic
   and controller memory. If your applications can tolerate reading stale values until the next
   manual restart, you don't need Reloader.

   In clusters with very large numbers of Secrets or high churn, watch traffic and controller memory
   can become noticeable. See the [Reloader documentation](https://github.com/stakater/Reloader)
   for tuning; align **`watchGlobally`** with your allowlist as described in
   [Reloader RBAC](#reloader-rbac).

2. **Prefer `conjur.org/restart-secrets` over `conjur.org/automatic-restart` when a Deployment
   references multiple secrets:**
   This avoids unnecessary restarts when an unrelated secret changes.

3. **Ensure the workload's Pod spec directly references the Kubernetes Secret:**
   Reloader inspects `envFrom`, `env.valueFrom`, and `volume` mounts to discover which secrets a
   workload depends on. If the secret is consumed indirectly (e.g. read by an init script from a
   path not declared as a volume), Reloader will not detect the dependency.
