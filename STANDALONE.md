# Standalone Mode

This page covers deployment of the CyberArk Secrets Provider in **standalone mode** (added in v1.10.0).

## Why this mode exists

Standalone mode is designed for teams that want the provider to stay running and keep
Kubernetes secrets synchronized continuously. In contrast to application mode (one-off
Kubernetes Job), standalone mode is appropriate when you need:

- ongoing secret rotation from Secrets Manager to Kubernetes Secrets,
- management of secrets across multiple namespaces,
- automatic application rollouts when a secret changes (via [Reloader][RELOADER.md]).

## Benefits

- `Deployment` lifecycle: the provider runs as a long-lived Pod and can recover from
  transient failures without being relaunched by an external controller.
- Continuous reconciliation: secrets are refreshed on a configurable interval,
  which means secret updates are detected and applied repeatedly.
- Multi-namespace support: namespace discovery can be controlled through
  `standalone.namespaceAllowlist`.
- Shared control plane: one provider instance can service many Kubernetes Secrets,
  which is a good fit when most applications should follow common secret-provider
  behavior.
- Native compatibility with automatic app restarts: when combined with
  `autoReload.enabled`, updated Kubernetes Secrets can trigger workload rollouts.

## Tradeoffs

- Additional operational surface: standalone mode introduces a continuously running
  security-critical controller that must be monitored, backed by alerts and lifecycle
  management.
- RBAC scope expands as namespace reach expands: using `standalone.namespaceAllowlist=*`
  or multi-namespace lists usually requires cluster-wide read permissions and careful
  IAM review.
- Polling behavior introduces bounded lag: rotation timing is linked to refresh interval
  and any API/network delays.
- More coupling in platform policy design: if your policy model expects strict
  per-pod authorization, a per-workload pattern (init/sidecar) can be easier to reason
  about.
- Operational blast radius can grow if a single provider deployment manages many
  namespaces; failures can affect multiple teams’ secrets if not segmented.

Use this mode when continuous reconciliation and central orchestration are higher
priority than strict per-Pod minimization.

## Example helm install/upgrade command

```bash
helm upgrade --install my-secrets-provider ./helm/secrets-provider \
  --namespace secrets-provider \
  --create-namespace \
  --set containerMode=standalone \
  --set standalone.secretsRefreshEnabled=true \
  --set standalone.secretsRefreshInterval=5m \
  --set standalone.removeDeletedSecretsEnabled=true \
  --set standalone.namespaceAllowlist="" \
  --set environment.conjur.conjurConnConfigMap=conjur-connect
```

## Key standalone chart values

- `containerMode=standalone`: deploys a long-running Deployment.
- `standalone.namespaceAllowlist`: controls which namespaces the provider scans for
  managed Kubernetes Secrets.
  - Empty string means current namespace.
  - `ns-a,ns-b,ns-c` enables multi-namespace discovery.
  - `*` enables cluster-wide discovery (not recommended).
- `standalone.secretsRefreshEnabled`: whether to refresh secrets periodically (default true).
- `standalone.secretsRefreshInterval`: poll interval for Conjur secret checks (default 5 minutes).
- `standalone.removeDeletedSecretsEnabled`: controls removal behavior for keys that
  disappear from managed secret definitions (default true).
- `autoReload.enabled=true`: installs the Reloader subchart to automate rollout of
  workloads consuming updated Kubernetes Secrets (default false).
- `reloader.reloader.watchGlobally=true`: required when allowlist includes multiple
  namespaces, so Reloader can watch all relevant workload targets.

When not using `conjurConnConfigMap`, set these explicit values:
`environment.conjur.account`, `environment.conjur.applianceUrl`,
`environment.conjur.authnUrl`, `environment.conjur.authnLogin`,
`environment.conjur.sslCertificate.name`.

## RBAC and multi-namespace alignment

When using multi-namespace discovery, keep the provider and any optional
Reloader deployment in the same authorization scope. At minimum, align:

- the provider namespace access (`standalone.namespaceAllowlist`),
- the chart RBAC names in `rbac.clusterRoleName` and `rbac.clusterRoleBindingName`,
- and `reloader.reloader.watchGlobally`.

Misalignment can cause partial discovery, failed reconciliations, or missed app rollouts.

## Configuring Kubernetes probes

When running in **standalone mode** the Secrets Provider exposes the following probe endpoints:

- `GET /healthz` (liveness)
- `GET /readyz` (readiness)

In all other deployment modes (init, application, or sidecar), probe endpoints are
not started.

### Configure server bind address

To let Kubernetes reach the probe endpoints from outside the container process,
set `SERVER_ADDRESS` (or annotation `conjur.org/server-address`) to a bindable
address such as `:8080` or `0.0.0.0:8080`.

> Avoid `127.0.0.1:8080` for Kubernetes probes, because kubelet probes the Pod
> IP and may not be able to reach a localhost-only bind.

### Pod spec example

```yaml
containers:
  - name: cyberark-secrets-provider-for-k8s
    image: cyberark/secrets-provider-for-k8s:latest
    env:
      - name: CONTAINER_MODE
        value: "standalone"
      - name: SECRETS_DESTINATION
        value: "k8s_secrets"
      - name: SERVER_ADDRESS
        value: ":8080"
    ports:
      - name: probes
        containerPort: 8080
    livenessProbe:
      httpGet:
        path: /healthz
        port: probes
      initialDelaySeconds: 5
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /readyz
        port: probes
      initialDelaySeconds: 5
      periodSeconds: 10
```

`/healthz` reports process health. `/readyz` reports provider readiness based on
the latest secret provisioning results.

### Helm probe configuration

If you deploy Secrets Provider in standalone mode with the chart, probe configuration is
customizable from `standalone.probes` in `helm/secrets-provider/values.yaml`:

- Default probe port: `8080`
- Default container port name: `probes`
- Default liveness probe: `GET /healthz` with `initialDelaySeconds: 5`, `periodSeconds: 10`
- Default readiness probe: `GET /readyz` with `initialDelaySeconds: 5`, `periodSeconds: 5`
- You can also tune `timeoutSeconds`, `failureThreshold`, and `successThreshold`.

Example:

```yaml
standalone:
  probes:
    port: 8080
    portName: probes
    livenessPath: /healthz
    readinessPath: /readyz
    livenessProbe:
      initialDelaySeconds: 10
      periodSeconds: 12
      timeoutSeconds: 2
    readinessProbe:
      initialDelaySeconds: 8
      periodSeconds: 6
      timeoutSeconds: 2
```

You can apply this from Helm with `--set` or a values file.

## Related docs

- [Automatic Application Restart (Reloader)](README.md#automatic-application-restart-reloader)
- [Reloader Integration Guide](RELOADER.md)
