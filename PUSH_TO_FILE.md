# Secrets Provider - Push to File Mode

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Overview](#overview)
- [How Push to File Works](#how-push-to-file-works)
- [Set up Secrets Provider for Push to File](#set-up-secrets-provider-for-push-to-file)
- [Reference Table of Configuration Annotations](#reference-table-of-configuration-annotations)
- [Example Common Policy Path](#example-common-policy-path)
- [Example Secret File Formats](#example-secret-file-formats)
- [Custom Templates for Secret Files](#custom-templates-for-secret-files)
- [Configuring Pod Volumes and Container Volume Mounts](#configuring-pod-volumes-and-container-volume-mounts-for-push-to-file)
- [Secret File Attributes](#secret-file-attributes)
- [Deleting Secret Files](#deleting-secret-files)
- [Decoding Base64 Encoded Secrets](#decoding-base64-encoded-secrets)
- [Upgrading Existing Secrets Provider Deployments](#upgrading-existing-secrets-provider-deployments)
- [Troubleshooting](#troubleshooting)

## Overview

The push to file feature detailed below allows Kubernetes applications to
consume Secrets Manager secrets directly through one or more files accessed through
a shared, mounted volume. Providing secrets in this way should require zero
application changes, as reading local files is a common, platform agnostic
delivery method.

The Secrets Provider can be configured to create and write multiple files
containing Secrets Manager secrets. Each file is configured independently as a group of
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
   - The policy paths from which Secrets Manager secrets should be retrieved.
   - The format of the secret file to be rendered for that group.
   - How retrieved Secrets Manager secret values should be mapped to fields
     in the rendered secret file.

1. The Secrets Provider authenticates to the Secrets Manager server using the
   Kubernetes Authenticator (`authn-k8s`).

1. The Secrets Provider reads all Secrets Manager secrets required across all
   secret groups.

1. The Secrets Provider renders secret files for each secret group, and
   writes the resulting files to a volume that is shared with your application
   container.

1. The Secrets Provider init container runs to completion.

1. Your application container starts and consumes the secret files from
   the shared volume.

## Set up Secrets Provider for Push to File

This section describes how to set up the Secrets Provider for Kubernetes for
Push to File operation.

![push to file workflow](./assets/p2f-workflow.png)

1. <details><summary>Before you begin</summary>

   - Make sure that a Kubernetes Authenticator has been configured and enabled.
     For more information, contact your Secrets Manager admin, or see
     [Enable Authenticators for Applications](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationCluster.htm).

     In this procedure, we will use `dev-cluster` for the Kubernetes
     Authenticator.

   - This procedure assumes you have a configured Kubernetes namespace, with a
     service account for your application. The namespace must be configured with
     the [Namespace Preparation Helm chart](https://github.com/cyberark/conjur-authn-k8s-client/tree/master/helm/conjur-config-namespace-prep#conjur-namespace-preparation-helm-chart).

     In this procedure, we will use `test-app-namespace` for the namespace,
     and `test-app-sa` for the service account.

   - This procedure assumes that you are familiar with loading K8s manifests
     into your workspace.

   </details>

1. <details><summary>Define the application as a Secrets Manager host in policy</summary>


   **Secrets Manager admin:** To enable the Secrets Provider for Kubernetes
   (`cyberark-secrets-provider-for-k8s init container`) to retrieve Secrets Manager
   secrets, it first needs to authenticate to Secrets Manager.

   - In this step, you define a Secrets Manager host used to authenticate the
     `cyberark-secrets-provider-for-k8s` container with the Kubernetes
     Authenticator.

     The Secrets Provider for Kubernetes must be defined by its **namespace**
     and **authentication container name**, and can also be defined by its
     **service account**. These definitions are defined in the host
     annotations in the policy. For guidelines on how to define annotations, see
     [Application Identity in Kubernetes](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_AppIdentity.htm).

     The following policy:

     - Defines a Secrets Manager identity for `test-app` by its namespace,
       `test-app-namespace`, authentication container name,
       `cyberark-secrets-provider-for-k8s`, as well as by its service account,
       `test-app-sa`.

     - Gives `test-app` permissions to authenticate to Secrets Manager using the
       `dev-cluster` Kubernetes Authenticator.

     Save the policy as **apps.yml**:

     ```yaml
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
       Secrets Manager. When you create the application deployment manifest below,
       verify that the CyberArk Secrets Provider for Kubernetes init container
       has the same name.__

   - Load the policy file to root.

     ```sh
     conjur policy load -f apps.yml -b root
     ```

   </details>

1. <details><summary>Define variables to hold the secrets for your application,
   and grant the access to the variables</summary>

   **Secrets Manager admin:** In this step, you define variables (secrets) to which
   the Secrets Provider for Kubernetes needs access.


   - Save the following policy as **app-secrets.yml**.

     This policy defines Secrets Manager variables and a group that has permissions on
     the variables.

     In the following example, all members of the `consumers` group are
     granted permissions on the `username` and `password` variables:

     ```yaml
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

     ```sh
     conjur policy load -f app-secrets.yml -b root
     ```

   - Populate the variables with secrets, for example `myUser` and `MyP@ssw0rd!`:

     ```sh
     conjur variable set -i secrets/username -v myUser
     conjur variable set -i secrets/password -v MyP@ssw0rd!
     ```

1. <details><summary>Set up the application deployment manifest</summary>

   **Application developer:** In this step you set up an application
   deployment manifest that includes includes an application container,
   `myorg/test-app`, and an init container that uses the
   `cyberark/secrets-provider-for-k8s` image. The deployment manifest also
   includes Pod Annotations to configure the Secrets Provider for Kubernetes
   Push to File feature. The annotations direct the Secrets Provider to
   generate and write a secret file containing YAML key/value settings
   into a volume that is shared with the application container.

   Copy the following manifest and load it to the application namespace,
   `test-app-namespace`.

   __NOTE:__ The `mountPath` values used for the
   `cyberark-secrets-provider-for-k8s` container must appear exactly as
   shown in the manifest below, i.e.:

   - `/conjur/podinfo` for the `podinfo` volume.
   - `/conjur/secrets` for the `conjur-secrets` volume.

   ```yaml
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
           conjur.org/authn-identity: host/test-app
           conjur.org/container-mode: init
           conjur.org/secrets-destination: file
           conjur.org/conjur-secrets-policy-path.test-app: secrets/
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
           env:
           - name: MY_POD_NAME
             valueFrom:
               fieldRef:
                 fieldPath: metadata.name
           - name: MY_POD_NAMESPACE
             valueFrom:
               fieldRef:
                 fieldPath: metadata.namespace
           envFrom:
           - configMapRef:
               name: conjur-connect
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

   ```yaml
   "admin-username": "myUser"
   "admin-password": "myP@ssw0rd!"
   ```

## Reference Table of Configuration Annotations

Below is a list of Annotations that are used for basic Secrets Provider configuration 
and to write the secrets to file.
All annotations begin with `conjur.org/` so they remain unique.
Push to File Annotations are organized by "secret groups". A secrets group is a logical grouping of application secrets, typically belonging to a particular component of an application deployment (e.g. all secrets related to a backend database). Each group of secrets is associated with a specific destination file.

Please refer to the
[Secrets Provider documentation](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ic.htm)
for a description of each environment variable setting:

| K8s Annotation  | Equivalent<br>Environment Variable | Description, Notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
|-----------------------------------------|---------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `conjur.org/authn-identity`         | `CONJUR_AUTHN_LOGIN`  | Required value. Example: `host/conjur/authn-k8s/cluster/apps/inventory-api`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `conjur.org/container-mode`         | `CONTAINER_MODE`      | Allowed values: <ul><li>`init`</li><li>`application`</li></ul>Defaults to `init`.<br>Must be set (or default) to `init`for Push to File mode.<br>NOTE: `sidecar` is supported for [Secrets Rotation](ROTATION.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `conjur.org/secrets-destination`    | `SECRETS_DESTINATION` | Allowed values: <ul><li>`file`</li><li>`k8s_secrets`</li></ul>                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `conjur.org/k8s-secrets`            | `K8S_SECRETS`         | This list is ignored when `conjur.org/secrets-destination` annotation is set to **`file`**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `conjur.org/retry-count-limit`      | `RETRY_COUNT_LIMIT`   | Defaults to 5                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          
| `conjur.org/retry-interval-sec`     | `RETRY_INTERVAL_SEC`  | Defaults to 1 (sec)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `conjur.org/log-level`              | `LOG_LEVEL`           | Allowed values: <ul><li>`debug`</li><li>`info`</li><li>`warn`</li><li>`error`</li></ul>Defaults to `info`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `conjur.org/conjur-secrets.{secret-group}`      | Note\* | List of secrets to be retrieved from Secrets Manager. Each entry can be either:<ul><li>A Secrets Manager variable path</li><li> A key/value pairs of the form <br>`<alias>:<Secrets Manager variable path>`<br>`[content-type: <type>]`<br>where the `alias` represents the name of the secret to be written to the secrets file and the optional `content-type` is either text or base64, defaulting to text. See [Decoding Base64 Encoded Secrets](#decoding-base64-encoded-secrets) for more information.                                                                                                                                                                                                      |
| `conjur.org/conjur-secrets-policy-path.{secret-group}` | Note\* | Defines a common Secrets Manager policy path, assumed to be relative to the root policy.<br><br>When this annotation is set, the policy paths defined by `conjur.org/conjur-secrets.{secret-group}` are relative to this common path.<br><br>When this annotation is not set, the policy paths defined by `conjur.org/conjur-secrets.{secret-group}` are themselves relative to the root policy.<br><br>(See [Example Common Policy Path](#example-common-policy-path) for an explicit example of this relationship.)                                                                                                                                                                           |
| `conjur.org/secret-file-path.{secret-group}`    | Note\* | Relative path for secret file or directory to be written. This path is assumed to be relative to the respective mount path for the shared secrets volume for each container.<br><br>If the `conjur.org/secret-file-format.{secret-group}` is set to `template`, then this secret file path defaults to `{secret-group}.out`. For example, if the secret group name is `my-app`, the the secret file path defaults to `my-app.out`.<br><br>Otherwise, this secret file path defaults to `{secret-group}.{secret-group-file-format}`. For example, if the secret group name is `my-app`, and the secret file format is set for YAML, the the secret file path defaults to `my-app.yaml`. 
| `conjur.org/secret-file-permissions.{secret-group}`| Note\*| Explicitly defines secret file permissions. <br><br>Defaults to `-rw-r--r--` (Octal `644`)<br><br>Values must be formatted as a valid permission string _(Directory bit is optional)_. For example:<li>`-rw-rw-r--`</li><li>`rw-rw-r--`</li>Owner must have at a minimum read/write permissions (`-rw-------`)                                                                                                                                                                                                                                                                                                                                                                         
| `conjur.org/secret-file-format.{secret-group}`  | Note\* | Allowed values:<ul><li>yaml (default)</li><li>json</li><li>dotenv</li><li>bash</li><li>properties</li><li>template</li></ul><br>This annotation must be set to `template` when using custom templates.<br><br>(See [Example Secret File Formats](#example-secret-file-formats) for example output files.)                                                                                                                                                                                                                                                                                                                                                                              |
| `conjur.org/secret-file-template.{secret-group}`| Note\* | Defines a custom template in Golang text template format with which to render secret file content. See dedicated [Custom Templates for Secret Files](#custom-templates-for-secret-files) section for details.                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |

__Note*:__ These Push to File annotations do not have an equivalent
environment variable setting. The Push to File feature must be configured
using annotations.

## Example Common Policy Path

Given the relationship between `conjur.org/conjur-secrets.{secret-group}` and
`conjur.org/conjur-secrets-policy-path.{secret-group}`, the following sets of
annotations will eventually retrieve the same secrets from Secrets Manager:

```yaml
conjur.org/conjur-secrets.db: |
  - url: policy/path/api-url
  - policy/path/username
  - policy/path/password
```

```yaml
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

```yaml
"api-url": "dev/redis/api-url"
"admin-username": "dev/redis/username"
"admin-password": "dev/redis/password"
```

### Example JSON Secret File

Here is an example JSON format secret file. This format is rendered when
the `conjur.org/secret-file-format.{secret-group}` annotation is set
to `json`:

```json
{"api-url":"dev/redis/api-url","admin-username":"dev/redis/username","admin-password":"dev/redis/password"}
```

### Example Bash Secret File

Here is an example bash format secret file. This format is rendered when
the `conjur.org/secret-file-format.{secret-group}` annotation is set
to `bash`:

```sh
export api-url="dev/redis/api-url"
export admin-username="dev/redis/username"
export admin-password="dev/redis/password"
```

### Example dotenv/properties Secret File

Here is an example dotenv format file secret file. This format is rendered when
the `conjur.org/secret-file-format.{secret-group}` annotation is set
to `dotenv` or `properties`:

```sh
api-url="dev/redis/api-url"
admin-username="dev/redis/username"
admin-password="dev/redis/password"
```

> NOTE: The only difference between dotenv and properties is the validation of the key. For dotenv, the key cannot contain '.'. For properties, the key can contain '.'.

## Custom Templates for Secret Files

In addition to offering standard file formats, Push to File allows users to
define their own custom secret file templates. These templates adhere to Go's
text template formatting. Using custom templates requires the following:

- The annotation `conjur.org/secret-file-format.{secret-group}` must be set to
  `template`.
- The annotation `conjur.org/secret-file-path.{secret-group}` must be set, and
  must specify a file name.

The custom template must be provided by only one of the methods listed below,
either by Pod annotation or by volume-mounted ConfigMap. Providing a template by
both or neither method fails before retrieving secrets from Secrets Manager.

1. <details><summary>Pod annotation</summary>

   Custom templates can be defined with the Pod annotation
   `conjur.org/secret-file-template.{secret-group}`. The following annotations
   describe a valid secret group that uses a custom template defined in Pod
   annotations:

   ```yaml
   conjur.org/secret-group.example: |
     - admin-username: <variable-policy-path>
     - admin-password: <variable-policy-path>
   conjur.org/secret-file-format.example: "template"
   conjur.org/secret-file-template.example: |
     "database": {
       "username": {{ secret "admin-username" }},
       "password": {{ secret "admin-password" }},
     }
   ```

</details>

2. <details><summary>Volume-mounted ConfigMap</summary>

   Custom templates can be provided as fields in a ConfigMap. This feature
   requires the following:

   - The ConfigMap containing template files must be created before the Secrets
     Provider container.
   - The ConfigMap must be mounted in the Secrets
     Provider container at `/conjur/templates`.
   - Each template's key in the ConfigMap's `data` field must be formatted as
     `{secret-group}.tpl`.

   ### Creating a ConfigMap for Secret File Templates

   #### Defining Templates in a ConfigMap Manifest

   The following is an example of a ConfigMap manifest defining a custom Go
   template for a secret group `example`:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: my-custom-template
   data:
     example.tpl: |
       "database": {
         "username": {{ secret "admin-username" }},
         "password": {{ secret "admin-password" }},
       }
   ```

   If custom secret file templates are required for multiple secret groups, add
   entries to the `data` field, adhering to the `{secret-group}.tpl` key
   pattern.

   #### Creating a ConfigMap from an Existing Template File

   A ConfigMap can also be created from an existing, compatible template file,
   allowing secret file templates to be checked into version control independent
   from a K8s manifest.

   Given the following template file, `example.tpl`:

   ```go
   "database": {
     "username": {{ secret "admin-username" }},
     "password": {{ secret "admin-password" }},
   }
   ```

   Create a ConfigMap from the template files:

   ```sh
   kubectl create configmap my-custom-template --from-file=/path/to/example.tpl
   ```

   The resulting ConfigMap will be functionally identical to one created from
   the [example ConfigMap manifest](#defining-templates-in-a-configmap-manifest)
   shown above.

   If secret file templates are required for multiple secret groups, a ConfigMap
   can be created from a directory of template files, each adhering to the
   `{secret-group}.tpl` naming pattern:

   ```sh
   kubectl create configmap my-custom-template --from-file=/path/to/template/dir/
   ```

   ### Configuring Secrets Provider to Consume ConfigMap-based Templates

   The following annotations describe a valid secret group that uses the custom
   template created by either of the previously described methods:

   ```yaml
   conjur.org/secret-group.example: |
     - admin-username: <variable-policy-path>
     - admin-password: <variable-policy-path>
   conjur.org/secret-file-format.example: "template"
   ```

   In order for Secrets Provider to consume the templates, another volume needs
   to be added to the application's deployment manifest, populated with the
   contents of the ConfigMap:

   ```yaml
   volumes:
   - name: conjur-templates
     configMap:
       name: my-custom-template
   ```

   Then, mount the ConfigMap-populated volume to the Secrets Provider init
   container at `/conjur/templates`:

   ```yaml
   volumeMounts:
   - name: conjur-templates
     mountPath: /conjur/templates
   ```

   </details>

### Using Secrets Manager Secrets in Custom Templates

Injecting Secrets Manager secrets into custom templates requires using the built-in custom
template function `secret`. The action shown below renders the value associated
with `<secret-alias>` in the secret-file.

```go
{{ secret "<secret-alias>" }}
```

### Global Template Functions

Custom templates support global functions native to Go's `text/template`
package. The following is an example of using template function to HTML
escape/encode a secret value.

```go
{{ secret "alias" | html }}
{{ secret "alias" | urlquery }}
```

If the value retrieved from Secrets Manager for `alias` is `"<Hello@World!>"`,
the following file content will be rendered, each HTML escaped and encoded,
respectively:

```txt
&lt;Hello;@World!&gt;
%3CHello%40World%21%3E
```

For a full list of global Go text template functions, reference the official
[`text/template` documentation](https://pkg.go.dev/text/template#hdr-Functions).

### Additional Template Functions

Custom templates also support a limited number of additional functions. Currently
supported functions are:

- `b64enc`: Base64 encode a value.
- `b64dec`: Base64 decode a value.

These can be used as follows:

```go
{{ secret "alias" | b64enc }}
{{ secret "alias" | b64dec }}
```

When using the `b64dec` function, an error will occur if the value retrieved from
Secrets Manager is not a valid Base64 encoded string.

### Execution "Double-Pass"

To avoid leaking sensitive secret data to logs, and to ensure that a
misconfigured Push to File workflow fails fast, Push to File implements a
"double-pass" execution of custom templates. The template "first-pass" runs
before secrets are retrieved from Secrets Manager, and validates that the provided custom
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

```yaml
conjur.org/secret-file-path.direct-reference: ./direct.txt
conjur.org/secret-file-template.direct-reference: |
  username | {{ secret "db-username" }}
  password | {{ secret "db-password" }}
```

Assuming that the following secrets have been retrieved for secret group
`direct-reference`:

```yaml
db-username: admin
db-password: my$ecretP@ss!
```

Secrets Provider will render the following content for the file
`/conjur/secrets/direct.txt`:

```txt
username | admin
password | my$ecretP@ss!
```

### Example Custom Templates: Iterative approach

The following is an example of using a custom template to render secret data
using an iterative process instead of referencing all variables directly.

```yaml
conjur.org/secret-file-path.iterative-reference: ./iterative.txt
conjur.org/secret-file-template.iterative-reference: |
  {{- range $index, $secret := .SecretsArray -}}
  {{- $secret.Alias }} | {{ $secret.Value }}
  {{- end -}}
```

Here, `.SecretsArray` is a reference to Secret Provider's internal array of
secrets that have been retrieved from Secrets Manager. For each entry in this array,
there is a secret `Alias` and `Value` field that can be referenced in the custom
template.

Assuming that the following secrets have been retrieved for secret group
`iterative-reference`:

```txt
db-username: admin
db-password: my$ecretP@ss!
```

Secrets Provider will render the following content for the file
`/conjur/secrets/iterative.txt`:

```txt
db-username | admin
db-password | my$ecretP@ss!
```

### Example Custom Templates: PostgreSQL connection string

The following is an example of using a custom template to render a secret file
containing a Postgres connection string. For a secret group described by the
following annotations:

```yaml
conjur.org/secret-file-path.postgres: ./pg-connection-string.txt
conjur.org/secret-file-template.postgres: |
  postgresql://{{ secret "dbuser" }}:{{ secret "dbpassword" }}@{{ secret "hostname" }}:{{ secret "hostport" }}/{{ secret "dbname" }}??sslmode=require
```

Assuming that the following secrets have been retrieved for secret group
`postgres`:

```yaml
dbuser:     "my-user"
dbpassword: "my-secret-pa$$w0rd"
dbname:     "postgres"
hostname:   "database.example.com"
hostport:   5432
```

Secrets Provider will render the following content for the file
`/conjur/secrets/pg-connection-string.txt`:

```txt
postgresql://my-user:my-secret-pa$$w0rd@database.example.com:5432/postgres??sslmode=require
```

### Example Custom Templates: Spring Boot configuration

Many Spring Boot applications use [externalized configuration](https://docs.spring.io/spring-boot/docs/current/reference/html/features.html#features.external-config)
loaded at start-up, usually as an `application.yaml` or `.properties` file. For
Spring Boot applications deployed in Kubernetes, Secrets Provider's Push-to-File
feature can generate properly formatted config files, populated with secret
data, using custom templates.

This example assumes that the target Secrets Manager server has been loaded with the
following secrets, representing a PostgreSQL database and its credentials:

- `backend/pg/url`
- `backend/pg/username`
- `backend/pg/password`

These secrets will be used by a secret group, `spring-app`, to render a Spring
Boot configuration file.

Create a file defining a Spring Boot configuration template, saved as
`spring-app.tpl`. The following represent template examples for an
`application.yml` template:

```yaml
spring:
  datasource:
    platform: postgres
    url: jdbc:{{ secret "url" }}
    username: {{ secret "username" }}
    password: {{ secret "password" }}
  jpa:
    generate-ddl: true
    hibernate:
      ddl-auto: update
```

...and an `application.properties` template:

```txt
spring.datasource.platform: postgres
spring.datasource.url: jdbc:{{ secret "url" }}
spring.datasource.username: {{ secret "username" }}
spring.datasource.password: {{ secret "password" }}
spring.jpa.generate-ddl: true
spring.jpa.hibernate.ddl-auto: update
```

Create a ConfigMap from the new template file:

```sh
kubectl create configmap spring-boot-templates \
  --from-file=/path/to/spring-app.tpl \
  --namespace test-app-namespace
```

Now that the `spring-boot-templates` ConfigMap is deployed to the desired
namespace, set up a Deployment manifest, with:

- a Secrets Provider init container configured for Push-to-File with custom
  secret file templates,
- a Spring Boot application container, configured to consume the newly-generated
  configuration file with the `--spring.config.location` flag.

Copy the following manifest and load it into the application namespace,
`test-app-namespace`. Configuration unique to this example is marked in <b>bold</b>.

<pre><code>apiVersion: apps/v1
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
        conjur.org/authn-identity: host/test-app
        conjur.org/container-mode: init
        conjur.org/secrets-destination: file
        <b>conjur.org/conjur-secrets-policy-path.spring-app: backend/pg/
        conjur.org/conjur-secrets.spring-app: |
          - url
          - username
          - password
        conjur.org/secret-file-path.spring-app: "./application.{yaml|properties}"
        conjur.org/secret-file-format.spring-app: "template"</b>
    spec:
      serviceAccountName: test-app-sa
      containers:
      - name: test-app
        image: myorg/test-springboot-app
        <b>command: ["java", "-jar", "/app.jar", "--spring.config.location=file:/opt/secrets/conjur/application.{yaml|properties}"]</b>
        volumeMounts:
          - name: conjur-secrets
            mountPath: /opt/secrets/conjur
            readOnly: true
      initContainers:
      - name: cyberark-secrets-provider-for-k8s
        image: 'cyberark/secrets-provider-for-k8s:latest'
        imagePullPolicy: Never
        env:
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        envFrom:
        - configMapRef:
            name: conjur-connect
        volumeMounts:
          - name: podinfo
            mountPath: /conjur/podinfo
          - name: conjur-secrets
            mountPath: /conjur/secrets
          <b>- name: conjur-templates
            mountPath: /conjur/templates</b>
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
        <b>- name: conjur-templates
          configMap:
            name: spring-boot-templates</b>
            
</code></pre>

## Configuring Pod Volumes and Container Volume Mounts for Push to File

Enabling Push to File on your application Pod requires the addition of several Volumes and VolumeMounts.
Some example Volume/VolumeMount configurations are shown below.

- Example Pod Volumes:
  
  ```yaml
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

- Example volume mounts for the Secrets Provider init container. The `mountPath` values must appear exactly as shown below:

  ```yaml
  volumeMounts:
    - name: podinfo
      mountPath: /conjur/podinfo
    - name: conjur-secrets
      mountPath: /conjur/secrets
  ```

- Example volume mounts for the application container. The mountPath to use is dependent upon where your application expects to find secret files:

  ```yaml
  volumeMounts:
    - name: conjur-secrets
      mountPath: <your-desired-path-to-secret-files>
  ```

## Secret File Attributes

By default, the Secrets Provider will create secrets files with the following file attributes:

|  Attribute  |       Value        | Notes  |
| ----------- | ------------------ | ------ |
| User        | `secrets-provider` |        |
| Group       | `root`             | OpenShift requires that any files/directories that are shared between containers in a Pod must use a GID of 0 (i.e. GID for the root group) The Secrets Provider uses a GID of 0 for secrets files even for non-OpenShift platforms, for simplicity.       |
| UID         | `777`              |        |
| GID         | `0`                |        |
| Permissions | `rw-rw-r--`        | As shown in the table table, the Secrets Provider will create secrets files that are world readable. This means that the files will be readable from any other container in the same Pod that mounts the Secrets Manager secrets shared Volume.       |

The file attributes can be overridden by defining a 
[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#podsecuritycontext-v1-core) for the application Pod.

For example, to have containers run as the `nobody` user (UID 65534) and `nobody` group (GID 65534), 
and have secrets files created with the corresponding GID, the Pod SecurityContext 
would be as follows:

```yaml
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

## Decoding Base64 Encoded Secrets

Secrets can be stored in Secrets Manager as base64 encoded strings. If the applications consuming the secrets cannot be modified
to decode the secrets, those secrets become unusable. Secrets Provider can decode the secrets before handing it over to
the applications for consumption.

### Decoding Base64 with Push to File

Secrets Provider in push to file mode is configured using annotations.
The `conjur.org/conjur-secrets.{secret-group}` annotation will need to be modified
to decode secret.
Add the `content-type` annotation.
Note the `alias` must also be defined with the `path`.

Given the following secrets annotation:

```yaml
conjur.org/conjur-secrets.db: |
  - url: policy/path/api-url
  - policy/path/username
  - policy/path/password
```

Update with base64 decode as below:

```yaml
conjur.org/conjur-secrets.db: |
  - url: policy/path/api-url
  - policy/path/username
  - password: policy/path/password
    content-type: base64
```

With this annotation the contents of password will be base64 decoded.
If the contents cannot be decoded, a warning is displayed in the log files
and the contents retrieved will not be decoded.


### Decoding Base64 with Kubernetes Secrets

Secrets Provider in Kubernetes Secrets mode is configured using annotations and
by configuring a kubernetes secret with a `conjur-map`.

Given the following Kubernetes Secret configuration

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

To add base64 decoding to the `test-secrets-provider-k8s-app-db/password`
modify the `DB_PASSWORD` with an id: and content-type: as below.

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
    DB_PASSWORD:
      id: test-secrets-provider-k8s-app-db/password
      content-type: base64
```

If the contents cannot be decoded, a warning is displayed in the log files
and the contents retrieved will not be decoded.

## Upgrading Existing Secrets Provider Deployments

At a high level, converting an existing Secrets Provider deployment to use
annotation-based configuration and/or Push to File mode can be done in two ways:

- Inspect the existing application Deployment manifest (if available) or inspect
  a live deployment with:

  ```sh
  kubectl get deployment $DEPLOYMENT_NAME -o yaml
  ```

  Update the manifest with the changes described below, and then re-apply then
  configuration:

  ```sh
  kubectl apply -f <updated-manifest-filepath>
  ```

- Edit a live deployment, applying the changes described below, with the
  interactive command:

  ```sh
  kubectl edit deployment $DEPLOYMENT_NAME
  ```

To convert Secrets Provider from K8s Secrets mode to Push to File mode, make the
following changes to the deployment:

- Convert the Service Provider container/Secrets Manager environment variable settings
  to the equivalent annotation-based settings. Use the
  [Annotation Reference Table](#reference-table-of-configuration-annotations)
  for envvar-to-annotation mappings.
  - `conjur.org/secrets-destination: file` enables Push to File mode.
  - `conjur.org/conjur-secrets.{group}` defines Secrets Manager policy paths previously
    defined in Kubernetes Secrets.
    - For each existing Kubernetes Secret, you may wish to create a separate
      secrets group.
    - Inspect the manifests for the existing Kubernetes Secret(s). The manifests
      should contain a `stringData` section that contains secrets key/value pairs.
      Map the `stringData` entries to a YAML list value for conjur-secrets, using
      the secret names as aliases.
    - Alternatively, for existing deployments, this mapping can be obtained with
      the command

      ```sh
      kubectl get secret $SECRET_NAME -o jsonpath={.data.conjur-map} | base64 --decode
      ```

  - `conjur.org/secret-file-path.{group}` configures a target file destination.
  - `conjur.org/secret-file-format.{group}` configures a desired file type,
    depending on how the application will consume the secrets file.
  - Remove environment variables used for Secrets Provider configuration from
    the init container (see Annotation Reference table).
  - Remove environment variables referencing Kubernetes Secrets from the
    application container.
- Add push-to-file volumes:

    ```yaml
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
      - name: conjur-templates
        emptyDir:
          medium: Memory
    ```

- Add push-to-file volume mounts to the Secrets Provider init container:

    ```yaml
    volumeMounts:
      - name: podinfo
        mountPath: /conjur/podinfo
      - name: conjur-secrets
        mountPath: /conjur/secrets
      - name: conjur-templates
        mountPath: /conjur/templates
    ```

- Add push-to-file volume mount to the application container:

    ```yaml
    volumeMounts:
      - name: conjur-secrets
        mountPath: /conjur/secrets
    ```

- Delete existing Kubernetes Secrets or their manifests:
  - If using Helm, delete Kubernetes Secrets manifests and do a
    `helm upgrade ...`
  - Otherwise, `kubectl delete ...` the existing Kubernetes Secrets
- Modify application to consume secrets as files:
  - Modify application to consume secrets files directly, or...
  - Modify the Deployment's spec for the app container so that the
    `command` entrypoint includes sourcing of a bash-formatted secrets file.

## Troubleshooting

This section describes how to troubleshoot common Secrets Provider for Kubernetes issues.

### Enable debug logs

To enable debug logs, add the debug parameter to the application deployment manifest.

```yaml
annotations:
    conjur.org/log-level  "debug"
```

For details, see [Reference Table of Configuration Annotations](#reference-table-of-configuration-annotations)

### Display logs

To display the Secrets Provider logs in Kubernetes.

- Go to the app namespace

  ```sh
  kubectl config set-context --current --namespace=<namespace>
  ```

- Find the pod name

  ```sh
  kubectl get pods
  ```

- Display the logs

  ```sh
  kubectl logs <pod-name> -c <init-container-name>
  ```

  To display the Secrets Provider logs in Openshift.

- Go to the app namespace
  ```sh
  oc project <project-name>
  ```

- Find the pod name

  ```sh
  oc get pods
  ```

- Display the logs

  ```sh
  oc logs <pod-name> -c <init-container-name>
  ```

### Common issues displayed in the logs and resolutions

|  Issue      |       Error code   | Resolution  |
| ----------- | ------------------ | ------ |
| Secrets Provider for Kubernetes failed to update secrets in file mode | `CSPFK039E` | Check that the secret key pairs have been configured correctly.  |
| Failed to open annotations file                                       | `CSPFK041E` | The annotations file cannot be opened by the Secrets provider. Check the volume mounts and downward API configuration.  |
| Annotation 'x' does not accept value                                  | `CSPFK042E` | This annotation requires a specific type for its value. The value provided is the wrong type. |
| Annotation 'x' does not accept value                                  | `CSPFK043E` | This annotation only accepts one of a specific set of values. The value provided is the wrong value. |
| Annotation file line 'x' is malformed:                                | `CSPFK045E` | The annotation described in the message is malformed. Check the annotation has a valid key and value       |
| Secret Store Type needs to be configured                              | `CSPFK046E` | Either the conjur.org/secrets-destination annotation has not been set, or the downward API volumes have not been configured as required. See the [Configuring Pod Volumes and Container Volume Mounts](#configuring-pod-volumes-and-container-volume-mounts-for-push-to-file) section above for more information.|
| Secrets Provider in Push-to-File mode can only be configured with Pod annotations | `CSPFK047E`| To configure push-to-file the Secrets store type must have the `conjur.org/secrets-destination` with pod annotations and cannot be set with the `SECRETS_DESTINATION` environment variable. If the secrets-destination is set via annotations verify that the pod volumes and volumeMounts are configured correctly. See the [Configuring Pod Volumes and Container Volume Mounts](#configuring-pod-volumes-and-container-volume-mounts-for-push-to-file) section above for more information. |
| Secrets Provider in K8s Secrets mode requires either the 'K8S_SECRETS' environment variable or 'conjur.org/k8s-secrets' | `CSPFK048E ` | If the `secrets-destination` is set to `k8s_secrets` then the `K8S_SECRETS` environment variable or `conjur.org/k8s-secrets` needs to be configured. This does not apply to push-to-file.       |
| Failed to validate Pod annotations                                    | `CSPFK049E` | The service provider was unable to successfully parse the annotations. This could be due to a previous error. Check the logs for a specific error before this.      |
| Unable to initialize Secrets Provider: unable to create secret group collection  |`CSPFK053E` | Secrets provider could be initialized. Check futher back in the log file for any specific configuration errors. |
| Unable to initialize Secrets Provider: unrecognized Store Type        | `CSPFK054E` | The `secrets-destination` value, either defined by the `SECRETS_DESTINATION` environment variable or the `conjur.org/secrets-destination` is an invalid value. `conjur.org/secrets-destination` should be `files` or `k8s-secrets`. |

### Other Common issues resolutions

| Issue       | Resolution |
| ----------- | ------------------ |
| There are no files created and no errors in the logs | Verify there is no SECRETS_DESTINATION environmental variable set and the volumes are set up correctly. |
<!--
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
-->
