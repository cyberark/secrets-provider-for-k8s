# Secrets Provider - Fetch All

## Existing Functionality

In the regular configuration of Secrets Provider, the application developer must
specify each secret that needs to be retrieved from Secrets Manager. This is done in one
of two ways, depending on how the secrets are being provided:

### Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: test-app-secrets-provider-k8s-secret
type: Opaque
stringData:
  conjur-map: |-
    DB_URL: test-secrets-provider-k8s-app-db/url
    DB_USERNAME: test-secrets-provider-k8s-app-db/username
    DB_PASSWORD: test-secrets-provider-k8s-app-db/password
```

The values in the K8s secret can now be used in the application pod like so:

```yaml
annotations:
  conjur.org/secrets-destination: k8s_secrets
  conjur.org/k8s-secrets: |
    - test-app-secrets-provider-k8s-secret
...
env:
  - name: DB_USERNAME
    valueFrom:
      secretKeyRef:
        name: test-app-secrets-provider-k8s-secret
        key: DB_USERNAME
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: test-app-secrets-provider-k8s-secret
        key: DB_PASSWORD
```

### Push to File

```yaml
annotations:
  conjur.org/secrets-destination: file
  conjur.org/conjur-secrets.test-app: |
    - db-url: test-secrets-provider-p2f-app-db/url
    - admin-username: test-secrets-provider-p2f-app-db/username
    - admin-password: test-secrets-provider-p2f-app-db/password
  conjur.org/secret-file-path.test-app: "./application.yaml"
  conjur.org/secret-file-format.test-app: "yaml"
```

## New Functionality

With the introduction of the Fetch All feature, the application developer can
now retrieve all secrets that the host has access to using the following new
syntax:

### Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: test-k8s-secret-fetch-all
type: Opaque
stringData:
  # Now choose ONE of the following two options:

  # For plaintext secrets
  conjur-map: |-
    "*": "*"
  
  # For secrets that should be decoded from Base64
  conjur-map: |-
    "*":
      id: "*"
      content-type: base64
```

The values in the K8s secret can now be used in the application pod in the usual
way, referencing the secret path as the key in the secret map, while replacing
any `/` with `.` (see [Limitations](#aliases-and-key-names) for more details).

```yaml
annotations:
  conjur.org/secrets-destination: k8s_secrets
  conjur.org/k8s-secrets: |
    - test-k8s-secret-fetch-all
...
env:
  - name: FETCH_ALL_TEST_SECRETS
    valueFrom:
      secretKeyRef:
        name: test-k8s-secret-fetch-all
        key: test-secrets-provider-k8s-app-db.username
```

Alternatively, you can use the following syntax to push all the retrieved secrets into
environment variables:

```yaml
envFrom:
  - secretRef:
      name: test-k8s-secret-fetch-all
```

This will create an environment variable for each secret, for example:

```sh
test-secrets-provider-k8s-app-db.url="..."
test-secrets-provider-k8s-app-db.username="..."
test-secrets-provider-k8s-app-db.password="..."
```

### Push to File

```yaml
conjur.org/secrets-destination: file
conjur.org/secret-file-format.test-app: json # or "yaml", or "template"
# Add any other relevant annotations, such as "conjur.org/secret-file-path.test-app", here

# Now choose ONE of the following two options:
# For plaintext secrets
conjur.org/conjur-secrets.test-app: "*"
# For secrets that should be decoded from Base64
conjur.org/conjur-secrets.test-app: |
  - "*": "*"
    content-type: base64
```

With this configuration, the Secrets Provider will retrieve all secrets that the
host has access to and provide them to the application pod in the usual way.

## Limitations

There are several important things to note about this feature:

### Security Implications

- The Fetch All feature should be used with caution, as it can expose more
  secrets than intended. Ideally, Secrets Manager should be configured to only allow the
  host to access the secrets that it needs. Additionally, the host should only
  be used for a small unit, such as a single application, and therefore only
  need access to a small number of secrets. If the host has access to a large
  number of secrets, it may be a sign that the host is too permissive and should
  be restricted. In the case of a compromised host, the attacker would have
  access to all secrets that the host has access to, which could be a
  significant security risk. It is important to follow the principle of least
  privilege in general, and even more so when using the Fetch All feature.

### Performance and Reliability

- Using Fetch All will be slightly slower than specifying each secret
  individually, as it requires multiple requests to Secrets Manager - first to list all
  the available secrets, and then to fetch them. In cases where performance is
  critical, it may be better to specify the secrets individually.

- Using Fetch All introduces a certain amount of unpredictability into the
  application, as the set of secrets that the host has access to may change,
  which could cause the application to behave unexpectedly. When specifying the
  secrets individually, Secrets Provider will fail if any of the specified
  secrets are not available, which can be a useful signal that something is
  wrong. With Fetch All, the application will continue to run even if some
  secrets are missing, and the application may behave in unexpected ways as a
  result. If using Fetch All, ensure that the application will handle missing
  secrets gracefully.

- To prevent denial of service due to very large numbers of secrets, the maximum
  number of secrets supported is 500. Secrets Provider will cease fetching secrets
  once it reaches this limit and log an error with code `CSPFK010D`.

### Aliases and Key Names

- There is no way to use aliases for secrets when using the Fetch All feature.
  This means that the keys used for the secrets (both in K8s Secrets and P2F)
  will be the *full path* of the secret in Secrets Manager. At the same time, Kubernetes
  secrets do not allow keys to contain slashes (`/`) or most other special
  characters. Due to these limitations:
  
  - *In K8s secrets mode:* Any slashes, spaces or other special characters
    (besides `_`, `-`, and `.`) in the Secrets Manager secret path will be replaced with
    dots (`.`) in the key names when using K8s Secrets. For example, if the
    secret is stored at `host/my-app/secrets/db-password`, the key in the K8s
    Secret will be `host.my-app.secrets.db-password`.
  - *Duplicate keys:* If there are two or more secrets that, after this
    character replacement, have the same key, a warning will be logged with
    error code `CSPFK067E`. The first secret will be used, and the others will
    be ignored. For example, if there are two secrets at
    `host/my-app/secrets/db.password` and `host/my-app/secrets/db password`
    (with a space in place of a `.`), the key in the K8s Secret for each of them
    will be `host.my-app.secrets.db.password`. The order in which the secrets
    are processed is not guaranteed, so one of them will be used and the other
    will be ignored. This will cause non-deterministic behavior in the
    application and must be avoided.

  - *In P2F mode:* The key names will be the full path of the secret in Secrets Manager.
    For example, if the secret is stored at `host/my-app/secrets/db-password`,
    the key in the P2F file will be `host/my-app/secrets/db-password`.
    This poses no issues for YAML and JSON files, since those formats
    support special characters in key names. However, it is impossible to
    use the `bash`, `dotenv`, and `properties` formats with secrets that have
    `/` in their path, and they are therefore not supported for use with the
    Fetch All feature. Additionally, when using custom templates, it must be
    ensured that the key names are valid for the chosen format.
  - *Custom Templates:* When using custom templates, aside from the above
    restriction due to keys containing `/`, it is also not possible to directly
    reference the secret path in the template. This is because the secret path
    is not known at the time the template is validated, before the secrets are
    actually retrieved. Additionally, there may be different secrets returned
    for subsequent fetches of the same template (when using rotation).
    Therefore, custom templates cannot rely on specific secret paths, and must
    instead use the `SecretsArray` variable to iterate over all secrets. Here is
    an example template that prints all fetched secrets in Base64 encoding:

    ```yaml
    conjur.org/secret-file-template.test-app: |
      {{range .SecretsArray}}{{ .Alias }}: {{ .Value | b64enc }}{{ "\n" }}{{end}}
    ```

    The resulting file will look like this:

    ```txt
    host/my-app/secrets/db-username: YWRtaW4=
    host/my-app/secrets/db-password: cGFzc3dvcmQ=
    ```

### Summary

The Fetch All feature is a powerful tool that can simplify the configuration of
Secrets Provider by allowing the application developer to retrieve all secrets
that the host has access to with a single configuration. However, it should be
used with caution, as it can expose more secrets than intended, and has some
limitations that must be taken into account when using it.

**For these reasons, we recommend using the Fetch All feature only in cases where
it is impractical to specify each secret individually, and only after carefully
considering the security implications and limitations of the feature.**
