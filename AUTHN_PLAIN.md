# Plain Authn (API Key) Authentication Mode

This document describes the addition of plain `authn` (host ID + API key)
authentication support to the CyberArk Secrets Provider for Kubernetes.

## Motivation

In environments where:
- `authn-k8s` suffers from certificate expiry issues (common on OpenShift)
- `authn-jwt` requires complex self-signed domain configurations
- The Conjur endpoint already has a trusted TLS certificate (e.g. DigiCert wildcard)

Plain `authn` provides a simpler, reliable alternative that uses the standard
Conjur REST authentication endpoint.

## What Changed

### New Files

| File | Description |
|------|-------------|
| `deploy/authn-plain/namespace.yaml` | Namespace manifest |
| `deploy/authn-plain/conjur-connect-configmap.yaml` | ConfigMap with Conjur URL, account, SSL cert |
| `deploy/authn-plain/conjur-credentials-secret.yaml` | K8s Secret holding login + API key |
| `deploy/authn-plain/rbac.yaml` | ServiceAccount, Role, RoleBinding |
| `deploy/authn-plain/cronjob.yaml` | CronJob manifest (every 5 min) |
| `deploy/authn-plain/target-secret.yaml` | Target K8s Secret with conjur-map |
| `AUTHN_PLAIN.md` | This document |

### Modified Files

| File | Change |
|------|--------|
| `pkg/secrets/clients/conjur/conjur_authenticator.go` | Added `PlainAuthenticator` struct + factory detection logic |
| `pkg/log/messages/info_messages.go` | Added `CSPFK042I` log message |
| `pkg/log/messages/debug_messages.go` | Added `CSPFK017D`, `CSPFK018D` log messages |
| `README.md` | Added "Plain Authn (API Key) Mode" section + TOC entry |

### No Changes To

- Secret retrieval logic (`conjur_secrets_retriever.go`)
- K8s Secrets patching (`k8s_secrets_storage/`)
- Push-to-file mode (`pushtofile/`)
- Retry/backoff logic (`provide_conjur_secrets.go`)
- Entrypoint orchestration (`entrypoint.go`)
- Config validation (`config/config.go`)
- Existing authenticators (authn-k8s, authn-jwt, authn-iam, authn-azure, authn-gcp)

## How It Works

### Selection Logic

In `NewAuthenticator()`, before checking `CONJUR_AUTHN_URL`:

```go
apiKey := os.Getenv("CONJUR_AUTHN_API_KEY")
if apiKey != "" {
    // Use plain authn mode
    return NewPlainAuthenticator(applianceURL, account, login, apiKey), nil
}
// Otherwise fall through to CONJUR_AUTHN_URL-based selection...
```

If `CONJUR_AUTHN_API_KEY` is set → plain authn. Otherwise → existing behavior unchanged.

### Authentication Flow

1. Build URL: `{CONJUR_APPLIANCE_URL}/authn/{account}/{url-encoded-login}/authenticate`
2. POST with body = raw API key, header: `Content-Type: text/plain`
3. Receive raw JSON access token (same format as all other authenticators)
4. Token is passed to `NewConjurClient()` for secret retrieval (same as all other authenticators)

### TLS / CA Certificate

The `PlainAuthenticator` respects `CONJUR_SSL_CERTIFICATE` and `CONJUR_CERT_FILE`
using the same resolution logic as `conjur-authn-k8s-client`'s `common.ReadSSLCert`:

1. `CONJUR_SSL_CERTIFICATE` — PEM cert content directly (takes precedence)
2. `CONJUR_CERT_FILE` — path to a PEM file (fallback), e.g. `/etc/ssl/certs/ca-certificates.crt`
3. If neither is set — error

For publicly-trusted certs (DigiCert, etc.), just use the Alpine system CA bundle:
```yaml
CONJUR_CERT_FILE: "/etc/ssl/certs/ca-certificates.crt"
```

For self-signed Conjur, put the PEM content directly in `CONJUR_SSL_CERTIFICATE`.

### Why Not Modify `conjur-authn-k8s-client`?

- It's an external CyberArk dependency pulled via `go.mod` — forking creates maintenance burden
- Its `Configuration` interface requires K8s-specific fields (`GetTokenFilePath`, `GetContainerMode`)
- Its `ReadSSLCert` **fails** if `CONJUR_SSL_CERTIFICATE` is empty — incompatible with publicly-trusted certs
- The IAM/Azure/GCP authenticators already bypass it — our pattern is consistent with those

## Environment Variables

| Variable | Description | Required for Plain Authn |
|----------|-------------|--------------------------|
| `CONJUR_AUTHN_API_KEY` | API key for the Conjur host. **Triggers plain authn mode.** | Yes |
| `CONJUR_APPLIANCE_URL` | Conjur server URL (e.g. `https://conjur.example.com`) | Yes |
| `CONJUR_ACCOUNT` | Conjur account (e.g. `BRI`) | Yes |
| `CONJUR_AUTHN_LOGIN` | Host identity (e.g. `host/apps/myapp`) | Yes |
| `CONJUR_AUTHN_URL` | Not required in plain authn mode | No |
| `CONJUR_SSL_CERTIFICATE` | PEM CA cert content (takes precedence over CONJUR_CERT_FILE) | One of these is required |
| `CONJUR_CERT_FILE` | Path to CA bundle file (e.g. `/etc/ssl/certs/ca-certificates.crt`) | One of these is required |

## Deployment

### As a CronJob (recommended for periodic sync)

```bash
kubectl apply -f deploy/authn-plain/namespace.yaml
kubectl apply -f deploy/authn-plain/rbac.yaml
kubectl apply -f deploy/authn-plain/conjur-connect-configmap.yaml
kubectl apply -f deploy/authn-plain/conjur-credentials-secret.yaml
kubectl apply -f deploy/authn-plain/target-secret.yaml
kubectl apply -f deploy/authn-plain/cronjob.yaml
```

The `conjur-connect` ConfigMap provides `CONJUR_APPLIANCE_URL`, `CONJUR_ACCOUNT`,
and optionally `CONJUR_SSL_CERTIFICATE` — consistent with existing authn-jwt
and authn-k8s deployments.

### As an Init Container

Set the same env vars in your Pod's init container spec. No `CONTAINER_MODE`
change needed — the default is `init`.

## Building and Testing Locally (Docker)

The image uses the same multi-stage build as the original project:
- **Build stage:** `golang:1.26`
- **Runtime stage:** `alpine:latest`

To build locally:

```bash
docker build --target secrets-provider -t secrets-provider-authn-plain:local .
```

To test the authentication against your Conjur (without K8s):

```bash
docker run --rm \
  -e CONJUR_APPLIANCE_URL=https://conjur.example.com \
  -e CONJUR_ACCOUNT=BRI \
  -e CONJUR_AUTHN_LOGIN=host/apps/myapp \
  -e CONJUR_AUTHN_API_KEY=your-api-key \
  -e CONJUR_SSL_CERTIFICATE="$(cat conjur-ca.pem)" \
  -e MY_POD_NAMESPACE=default \
  -e SECRETS_DESTINATION=k8s_secrets \
  -e K8S_SECRETS=app-secrets \
  secrets-provider-authn-plain:local
```

If your Conjur uses a publicly-trusted cert, omit `CONJUR_SSL_CERTIFICATE`.

Note: Running outside a K8s cluster will fail at the K8s client step (patching
secrets), but you will see the authentication succeed in the logs. For a full
local test, use Kind or Docker Desktop Kubernetes.

## Log Messages

| Code | Level | Message |
|------|-------|---------|
| `CSPFK042I` | INFO | "Using plain authn (API key) authentication mode" |
| `CSPFK017D` | DEBUG | "Plain authn: authenticating at {url}" |
| `CSPFK018D` | DEBUG | "Plain authn: authentication successful" |
