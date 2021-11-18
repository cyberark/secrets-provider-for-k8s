# Secrets Provider - Push to File Mode

# Table of Contents

- [Table of Contents](#table-of-contents)
- [Overview](#overview)
- [How Push to File Works](#how-push-to-file-works)
- [Certification Level](#certification-level)
- [Set up Secrets Provider for Push to File](#set-up-secrets-provider-for-push-to-file)
- [Reference Table of Configuration Annotations](#reference-table-of-configuration-annotations)
- [Example Common Policy Path](#example-common-policy-path)
- [Example Secret File Formats](#example-secret-file-formats)
- [Custom Templates for Secret Files](#custom-templates-for-secret-files)
- [Secret File Attributes](#secret-file-attributes)
- [Upgrading Existing Secrets Provider Deployments](#upgrading-existing-secrets-provider-deployments)

## Overview

The push to file feature detailed below allows Kubernetes applications to
consume Conjur secrets directly through one or more files accessed through
a shared, mounted volume. Providing secrets in this way should require zero
application changes, as reading local files is a common, platform agnostic
delivery method.

The Secrets Provider can be configured to create and write multiple files
containing Conjur secrets. Each file is configured independently as a group of
[Kubernetes Pod Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/),
collectively referred to as a "secret group".

Using annotations for configuration is new to Secrets Provider with this
feature and provides a more idiomatic deployment experience.

## How Push to File Works

![how push to file works](./assets/how-push-to-file-works.png)

1. The Secrets Provider, deployed as a
   [Kubernetes init container](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/)
   in the same Pod as your application container, starts up and parses Pod
   annotations from a
   [Kubernetes Downward API volume](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/).
   The Pod annotations are organized in secret groups, with each secret group
   indicating to the Secrets Provider:
   - The policy paths from which Conjur secrets should be retrieved.
   - The format of the secret file to be rendered for that group.
   - How retrieved Conjur secret values should be mapped to fields
     in the rendered secret file.

1. The Secrets Provider authenticates to the Conjur server using the
   Kubernetes Authenticator (`authn-k8s`).

1. The Secrets Provider reads all Conjur secrets required across all
   secret groups.

1. The Secrets Provider renders secret files for each secret group, and
   writes the resulting files to a volume that is shared with your application
   container.

1. The Secrets Provider init container runs to completion.

1. Your application container starts and consumes the secret files from
   the shared volume.

## Certification Level
![](https://img.shields.io/badge/Certification%20Level-Community-28A745?link=https://github.com/cyberark/community/blob/master/Conjur/conventions/certification-levels.md)

The Secrets Provider push to File feature is a **Community** level project. It's a community contributed project that **is not reviewed or supported
by CyberArk**. For more detailed information on our certification levels, see [our community guidelines](https://github.com/cyberark/community/blob/master/Conjur/conventions/certification-levels.md#community).

## Set up Secrets Provider for Push to File

This section describes how to set up the Secrets Provider for Kubernetes for
Push to File operation.

![push to file workflow](./assets/p2f-workflow.png)

1. <details><summary>Before you begin</summary>

   - This procedure assumes you have a configured Kubernetes namespace, with
     a service account for your application. It also assumes that you are
     familiar with loading manifests into your workspace.

     In this procedure, we will use `test-app-namespace` for the namespace,
     and `test-app-sa` for the service account.

   - Make sure that a Kubernetes Authenticator has been configured and enabled.
     For more information, contact your Conjur admin, or see
     [Enable Authenticators for Applications](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationCluster.htm).

   - You must configure Kubernetes namespace with the
     [Namespace Preparation Helm chart](https://github.com/cyberark/conjur-authn-k8s-client/tree/master/helm/conjur-config-namespace-prep#conjur-namespace-preparation-helm-chart).

   </details>

1. <details><summary>Define the application as a Conjur host in policy</summary>


   **Conjur admin:** To enable the Secrets Provider for Kubernetes
   (`cyberark-secrets-provider-for-k8s init container`) to retrieve Conjur
   secrets, it first needs to authenticate to Conjur.

   - In this step, you define a Conjur host used to authenticate the
     `cyberark-secrets-provider-for-k8s` container with the Kubernetes
     Authenticator.

     The Secrets Provider for Kubernetes must be defined by its **namespace**
     and **authentication container name**, and can also be defined by its
     **service account**. These definitions are defined in the host
     annotations in the policy. For guidelines on how to define annotations, see
     [Application Identity in Kubernetes](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_AppIdentity.htm).

     The following policy:

     - Defines a Conjur identity for `test-app` by its namespace,
       `test-app-namespace`, authentication container name,
       `cyberark-secrets-provider-for-k8s`, as well as by its service account,
       `test-app-sa`.

     - Gives `test-app` permissions to authenticate to Conjur using the
       `dev-cluster` Kubernetes Authenticator.

     Save the policy as **apps.yml**:

     ```
     - !host
       id: test-app
       annotations:
         authn-k8s/namespace: test-app-namespace
         authn-k8s/service-account: test-app-sa
         authn-k8s/authentication-container-name: cyberark-secrets-provider-for-k8s

     - !grant
       roles:
       - !group conjur/authn-k8s/dev-cluster/consumers
       members:
       - !host test-app
     ```

     __**NOTE:** The value of the host's authn-k8s/authentication-container-name
       annotation states the container name from which it authenticates to
       Conjur. When you create the application deployment manifest below,
       verify that the CyberArk Secrets Provider for Kubernetes init container
       has the same name.__

   - Load the policy file to root.

     ```
     $ conjur policy load -f apps.yml -b root
     ```

   </details>

1. <details><summary>Define variables to hold the secrets for your application,
   and grant the access to the variables</summary>

   **Conjur admin:** In this step, you define variables (secrets) to which
   the Secrets Provider for Kubernetes needs access.


   - Save the following policy as **app-secrets.yml**.

     This policy defines Conjur variables and a group that has permissions on
     the variables.

     In the following example, all members of the `consumers` group are
     granted permissions on the `username` and `password` variables:

     ```
     - !policy
       id: secrets
       body:
         - !group consumers
         - &variables
           - !variable username
           - !variable password
         - !permit
           role: !group consumers
           privilege: [ read, execute ]
           resource: *variables
     - !grant
       role: !group secrets/consumers
       member: !host test-app
     ```

   - Load the policy file to root.

     ```
     $ conjur policy load -f app-secrets.yml -b root
     ```

   - Populate the variables with secrets, for example `myUser` and `MyP@ssw0rd!`:

     ```
     $ conjur variable set -i secrets/username -v myUser
     $ conjur variable set -i secrets/password -v MyP@ssw0rd!
     ```

1. <details><summary>Set up the application deployment manifest</summary>


   **Application developer:** In this step you set up an application
   deployment manifest that includes includes an application container,
   `myorg/test-app`, and an init container that uses the
   `cyberark/secrets-provider-for-k8s` image. The deployment manifest also
   includes Pod Annotations to configure the Secrets Provider for Kubernetes
   Push to File feature. The annotations direct the Secrets Provider to
   generate and write a secret file containing YAML key/value settings
   into a volume that  is shared with the application container.

   Copy the following manifest and load it to the application namespace,
   `test-app-namespace`.

   __NOTE:__ The `mountPath` values used for the
   `cyberark-secrets-provider-for-k8s` container must appear exactly as
   shown in the manifest below, i.e.:

   - `/conjur/podinfo` for the `podinfo` volume.
   - `/conjur/secrets` for the `conjur-secrets` volume.


   ```
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     labels:
       app: test-app
     name: test-app
     namespace: test-app-namespace
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: test-app
     template:
       metadata:
         labels:
           app: test-app
         annotations:
           conjur.org/authn-identity: host/conjur/authn-k8s/dev-cluster/test-app
           conjur.org/container-mode: init
           conjur.org/secret-destination: file
           conjur.org/conjur-secrets-policy-path.first: secrets/
           conjur.org/conjur-secrets.test-app: |
           - admin-username: username
           - admin-password: password
           conjur.org/secret-file-path.test-app: "./credentials.yaml"
           conjur.org/secret-file-format.test-app: "yaml"
       spec:
         serviceAccountName: test-app-sa
         containers:
         - name: test-app
           image: myorg/test-app
           volumeMounts:
             - name: conjur-secrets
               mountPath: /opt/secrets/conjur
               readOnly: true
         initContainers:
         - name: cyberark-secrets-provider-for-k8s
           image: 'cyberark/secrets-provider-for-k8s:latest'
           imagePullPolicy: Never
            volumeMounts:
             - name: podinfo
               mountPath: /conjur/podinfo
             - name: conjur-secrets
               mountPath: /conjur/secrets
         volumes:
           - name: podinfo
             downwardAPI:
               items:
                 - path: "annotations"
                   fieldRef:
                     fieldPath: metadata.annotations
           - name: conjur-secrets
             emptyDir:
               medium: Memory
   ```

   The Secrets Provider will create a secret file in the `conjur-secrets`
   shared volume that will appear in the `test-app` container at location
   `/opt/secrets/conjur/credentials.yaml`, with contents as follows:

   ```
   "admin-username": "myUser"
   "admin-password": "myP@ssw0rd!"
   ```

## Reference Table of Configuration Annotations

Below is a list of Annotations that are used for basic Secrets Provider configurationv 
and to write the secrets to file.
All annotations begin with `conjur.org/` so they remain unique.
Push to File Annotations are organized by "secret groups". A secrets group is a logical grouping of application secrets, typically belonging to a particular component of an application deployment (e.g. all secrets related to a backend database). Each group of secrets is associated with a specific destination file.

Please refer to the
[Secrets Provider documentation](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ic.htm)
for a description of each environment variable setting:

| K8s Annotation  | Equivalent<br>Environment Variable | Description, Notes |
|-----------------------------------------|---------------------|----------------------------------|
| `conjur.org/authn-identity`         | `CONJUR_AUTHN_LOGIN`  | Required value. Example: `host/conjur/authn-k8s/cluster/apps/inventory-api` |
| `conjur.org/container-mode`         | `CONTAINER_MODE`      | Allowed values: <ul><li>`init`</li><li>`application`</li></ul>Defaults to `init`.<br>Must be set (or default) to `init` for Push to File mode.|
| `conjur.org/secrets-destination`    | `SECRETS_DESTINATION` | Allowed values: <ul><li>`file`</li><li>`k8s_secret`</li></ul> |
| `conjur.org/k8s-secrets`            | `K8S_SECRETS`         | This list is ignored when `conjur.org/secrets-destination` annotation is set to **`file`** |
| `conjur.org/retry-count-limit`      | `RETRY_COUNT_LIMIT`   | Defaults to 5
| `conjur.org/retry-interval-sec`     | `RETRY_INTERVAL_SEC`  | Defaults to 1 (sec)              |
| `conjur.org/debug-logging`          | `DEBUG`               | Defaults to `false`              |
| `conjur.org/conjur-secrets.{secret-group}`      | Note\* | List of secrets to be retrieved from Conjur. Each entry can be either:<ul><li>A Conjur variable path</li><li> A key/value pairs of the form `<alias>:<Conjur variable path>` where the `alias` represents the name of the secret to be written to the secrets file |
| `conjur.org/conjur-secrets-policy-path.{secret-group}` | Note\* | Defines a common Conjur policy path, assumed to be relative to the root policy.<br><br>When this annotation is set, the policy paths defined by `conjur.org/conjur-secrets.{secret-group}` are relative to this common path.<br><br>When this annotation is not set, the policy paths defined by `conjur.org/conjur-secrets.{secret-group}` are themselves relative to the root policy.<br><br>(See [Example Common Policy Path](#example-common-policy-path) for an explicit example of this relationship.)|
| `conjur.org/secret-file-path.{secret-group}`    | Note\* | Relative path for secret file or directory to be written. This path is assumed to be relative to the respective mount path for the shared secrets volume for each container.<br><br>If the `conjur.org/secret-file-template.{secret-group}` is set, then this secret file path must also be set, and it must include a file name (i.e. must not end in `/`).<br><br>If the `conjur.org/secret-file-template.{secret-group}` is not set, then this secret file path defaults to `{secret-group}.{secret-group-file-format}`. For example, if the secret group name is `my-app`, and the secret file format is set for YAML, the the secret file path defaults to `my-app.yaml`.
| `conjur.org/secret-file-format.{secret-group}`  | Note\* | Allowed values:<ul><li>yaml (default)</li><li>json</li><li>dotenv</li><li>bash</li></ul>(See [Example Secret File Formats](#example-secret-file-formats) for example output files.) |
| `conjur.org/secret-file-template.{secret-group}`| Note\* | Defines a custom template in Golang text template format with which to render secret file content. See dedicated [Custom Templates for Secret Files](#custom-templates-for-secret-files) section for details. |

__Note*:__ These Push to File annotations do not have an equivalent
environment variable setting. The Push to File feature must be configured
using annotations.

## Example Common Policy Path

Given the relationship between `conjur.org/conjur-secrets.{secret-group}` and
`conjur.org/conjur-secrets-policy-path.{secret-group}`, the following sets of
annotations will eventually retrieve the same secrets from Conjur:

```
conjur.org/conjur-secrets.db: |
  - url: policy/path/api-url
  - policy/path/username
  - policy/path/password
```

```
conjur.org/conjur-secrets-policy-path.db: policy/path/
conjur.org/conjur-secrets.db: |
  - url: api-url
  - username
  - password
```

## Example Secret File Formats

### Example YAML Secret File

Here is an example YAML format secret file. This format is rendered when
the `conjur.org/secret-file-format.{secret-group}` annotation is set
to `yaml`:

```
"api-url": "dev/redis/api-url"
"admin-username": "dev/redis/username"
"admin-password": "dev/redis/password"
```

### Example JSON Secret File

Here is an example JSON format secret file. This format is rendered when
the `conjur.org/secret-file-format.{secret-group}` annotation is set
to `json`:

```
{"api-url":"dev/redis/api-url","admin-username":"dev/redis/username","admin-password
":"dev/redis/password"}
```

### Example Bash Secret File

Here is an example bash format secret file. This format is rendered when
the `conjur.org/secret-file-format.{secret-group}` annotation is set
to `bash`:

```
     export api-url="dev/redis/api-url"
     export admin-username="dev/redis/username"
     export admin-password="dev/redis/password"
```

### Example dotenv Secret File

Here is an example dotenv format file secret file. This format is rendered when
the `conjur.org/secret-file-format.{secret-group}` annotation is set
to `dotenv`:

```
api-url="dev/redis/api-url"
admin-username="dev/redis/username"
admin-password="dev/redis/password"
```

## Custom Templates for Secret Files

In addition to offering standard file formats, Push to File allows users to
define their own custom secret file templates, configured with the
`conjur.org/secret-file-template.{secret-group}` annotation. These templates
adhere to Go's text template formatting. Providing a custom template will
override the use of any standard format configured with the annotation
`conjur.org/secret-file-format.{secret-group}`.

Injecting Conjur secrets into custom templates requires using the custom
template function `secret`. The action shown below renders the value associated
with `<secret-alias>` in the secret-file.

```
{{ secret "<secret-alias>" }}
```

### Global Template Functions

Custom templates support global functions native to Go's `text/template`
package. The following is an example of using template function to HTML
escape/encode a secret value.

```
{{ secret "alias" | html }}
{{ secret "alias" | urlquery }}
```

If the value retrieved from Conjur for `alias` is `"<Hello@World!>"`,
the following file content will be rendered, each HTML escaped and encoded,
respectively:

```
&lt;Hello;@World!&gt;
%3CHello%40World%21%3E
```

For a full list of global Go text template functions, reference the official
[`text/template` documentation](https://pkg.go.dev/text/template#hdr-Functions).

### Execution "Double-Pass"

To avoid leaking sensitive secret data to logs, and to ensure that a
misconfigured Push to File workflow fails fast, Push to File implements a
"double-pass" execution of custom templates. The template "first-pass" runs
before secrets are retrieved from Conjur, and validates that the provided custom
template successfully executes given `"REDACTED"` for each secret value.
Redacting secret values allows for secure, complete error logging for
malformed templates. The template "second-pass" runs when rendering secret
files, and error messages during this stage are sanitized. Custom templates that
pass the "first-pass" and fail the "second-pass" require experimenting locally
to identify bugs.

_**NOTE**: Custom templates should not branch conditionally on secret values.
This may result in a template first-pass execution that doesn't validate all
branches of the custom template._

### Example Custom Templates: Direct reference to secret values

The following is an example of using a custom template to render secret data by
referencing secrets directly using the custom template function `secret`.

```
conjur.org/secret-file-path.direct-reference: ./direct.txt
conjur.org/secret-file-template.direct-reference: |
  username | {{ secret "db-username" }}
  password | {{ secret "db-password" }}
```

Assuming that the following secrets have been retrieved for secret group
`direct-reference`:

```
db-username: admin
db-password: my$ecretP@ss!
```

Secrets Provider will render the following content for the file
`/conjur/secrets/direct.txt`:

```
username | admin
password | my$ecretP@ss!
```

### Example Custom Templates: Iterative approach

The following is an example of using a custom template to render secret data
using an iterative process instead of referencing all variables directly.

```
conjur.org/secret-file-path.iterative-reference: ./iterative.txt
conjur.org/secret-file-template.iterative-reference: |
  {{- range $index, $secret := .SecretsArray -}}
  {{- $secret.Alias }} | {{ $secret.Value }}
  {{- end -}}
```

Here, `.SecretsArray` is a reference to Secret Provider's internal array of
secrets that have been retrieved from Conjur. For each entry in this array,
there is a secret `Alias` and `Value` field that can be referenced in the custom
template.

Assuming that the following secrets have been retrieved for secret group
`iterative-reference`:

```
db-username: admin
db-password: my$ecretP@ss!
```

Secrets Provider will render the following content for the file
`/conjur/secrets/iterative.txt`:

```
db-username | admin
db-password | my$ecretP@ss!
```

### Example Custom Templates: PostgreSQL connection string

The following is an example of using a custom template to render a secret file
containing a Postgres connection string. For a secret group described by the
following annotations:

```
conjur.org/secret-file-path.postgres: ./pg-connection-string.txt
conjur.org/secret-file-template.postgres: |
  postgresql://{{ secret "dbuser" }}:{{ secret "dbpassword" }}@{{ secret "hostname" }}:{{ secret "hostport" }}/{{ secret "dbname" }}??sslmode=require
```

Assuming that the following secrets have been retrieved for secret group
`postgres`:

```
dbuser:     "my-user"
dbpassword: "my-secret-pa$$w0rd"
dbname:     "postgres"
hostname:   "database.example.com"
hostport:   5432
```

Secrets Provider will render the following content for the file
`/conjur/secrets/pg-connection-string.txt`:

```
postgresql://my-user:my-secret-pa$$w0rd@database.example.com:5432/postgres??sslmode=require
```

## Secret File Attributes

By default, the Secrets Provider will create secrets files with the following file attributes:

|  Attribute  |       Value        | Notes  |
| ----------- | ------------------ | ------ |
| User        | `secrets-provider` |        |
| Group       | `root`             | OpenShift requires that any files/directories that are shared between containers in a Pod must use a GID of 0 (i.e. GID for the root group) The Secrets Provider uses a GID of 0 for secrets files even for non-OpenShift platforms, for simplicity.       |
| UID         | `777`              |        |
| GID         | `0`                |        |
| Permissions | `rw-rw-r--`        | As shown in the table table, the Secrets Provider will create secrets files that are world readable. This means that the files will be readable from any other container in the same Pod that mounts the Conjur secrets shared Volume.       |

The file attributes can be overridden by defining a 
[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#podsecuritycontext-v1-core) for the application Pod.

For example, to have containers run as the `nobody` user (UID 65534) and `nobody` group (GID 65534), 
and have secrets files created with the corresponding GID, the Pod SecurityContext 
would be as follows:
```
    securityContext:
      runAsUser: 65534
      runAsGroup: 65534
      runAsNonRoot: true
      fsGroup: 65534
```

## Deleting Secret Files

Currently, it is recommended that applications do not delete secret files
after consuming the files. Kubernetes does not currently restart init
containers when primary (i.e. non-init) containers crash and cause
liveness or readiness probe failures. Because the Secrets Provider is run
as an init container for the Push to File feature, this means that it is
not restarted, and therefore secret files are not recreated, following
liveness or readiness failures.

## Upgrading Existing Secrets Provider Deployments

At a high level, converting an existing Secrets Provider deployment to use
annotation-based configuration and/or push-to-file mode:

- Inspect the existing application Deployment manifest (if available) or use
  ```
  kubectl get deployment $DEPLOYMENT_NAME -o yaml
  ```
  to inspect the application Deployment.
- Convert the Service Provider container/Conjur environment variable settings
  to the equivalent annotation-based setting. Edit the deployment with
  ```
  kubectl edit deployment $DEPLOYMENT_NAME
  ```
- If you are using the Secrets Provider as an init container, and you would
  like to convert from K8s Secrets mode to push-to-file mode:
  - Add push-to-file annotations:
    - For each existing Kubernetes Secret, you may wish to create a separate secrets group for push-to-file.
    - `conjur.org/secrets-destination: file`: Enable push-to-file mode
    - `conjur.org/conjur-secrets.{group}`: Inspect the manifests for the
      existing Kubernetes Secret(s). The manifests should contain a
      `stringData` section that contains secrets key/value pairs. Map the `stringData` entries to a YAML list value for conjur-secrets,
      using the secret names as aliases.
      - Alternatively, for existing deployments, this mapping can be obtained with the command
        ```
        kubectl get secret $SECRET_NAME -o jsonpath={.data.conjur-map} | base64 --decode
        ```

    - `conjur.org/secret-file-path.{group}`: Configure a target location
    - `conjur.org/secret-file-format.{group}`: Configure a desired type,
      depending on how the application will consume the secrets file.
  - Add push-to-file volumes:
      ```
      volumes:
        - name: podinfo
          downwardAPI:
            items:
              - path: annotations
                fieldRef:
                  fieldPath: metadata.annotations
        - name: conjur-secrets
          emptyDir:
            medium: Memory
      ```
  - Add push-to-file volume mounts to the Secrets Provider init container:
      ```
      volumeMounts:
        - name: podinfo
          mountPath: /conjur/podinfo
        - name: conjur-secrets
          mountPath: /conjur/secrets
      ```
  - Add push-to-file volume mount to the application container:
      ```
      volumeMounts:
        - name: conjur-secrets
          mountPath: /conjur/secrets
      ```
  - Remove environment variables used for Secrets Provider configuration from the init container (see annotations tables)
  - Remove environment variables referencing Kubernetes Secrets from the application container
  - Delete existing Kubernetes Secrets or their manifests:
    - If using Helm, delete Kubernetes Secrets manifests and do a
      `helm upgrade ...`
    - Otherwise, `kubectl delete ...` the existing Kubernetes Secrets
  - Modify application to consume secrets as files:
    - Modify application to consume secrets files directly, or...
    - Modify the Deployment's spec for the app container so that the
      `command` entrypoint includes sourcing of a bash-formatted secrets file.

### Using the Helper Script to Patch the deployment
There is a script in the secrets-provider-for-k8s bin directory named 
generate-annotation-upgrade-patch.sh that can be used to generate a patch file.

The patch can be output to a file:
```
bin/generate-annotation-upgrade-patch.sh --push-to-file \$DEPLOYMENT_NAME > patch.json
```
Test patch against a live deployment:
```
kubectl patch deployment \$DEPLOYMENT_NAME --type json --patch-file patch.json --dry-run=server
```
Preview the new deployment:
```
kubectl patch deployment \$DEPLOYMENT_NAME --type json --patch-file patch.json --dry-run=server --output yaml
```
Apply patch:
```
kubectl patch deployment \$DEPLOYMENT_NAME --type json --patch-file patch.json
```

