# Solution Design - Kubernetes Developer Experience M1 - Push to File

## Table of Contents

- [Useful Links](#useful-links)
- [Background](#background)
- [Solution](#solution)
- [Design](#design)
- [Performance](#performance)
- [Backwards Compatibility](#backwards-compatibility)
- [Affected Components](#affected-components)
- [Test Plan](#test-plan)
- [Logs](#logs)
- [Documentation](#documentation)
- [Version update](#version-update)
- [Security](#security)
- [Audit](#audit)
- [Development Tasks](#development-tasks)
- [Definition of Done](#definition-of-done)
- [Solution Review](#solution-review)
- [Appendix](#appendix)

## Useful Links

<table>
<thead>
<tr class="header">
<th>Name</th>
<th>Link</th>
</tr>
</thead>
<tbody>
<tr class="odd">
<td>PRD: Conjur Developer Experience for K8s</td>
<td><p>Sharepoint (private)</p></td>
</tr>
<tr class="even">
<td>Aha Card</td>
<td><p>SCR-E-76 (private)</p>
<p><em>Note: This design document covers work for "Milestone 1: Push to File" as defined in this Aha Card.</em></p></td>
</tr>
<tr class="odd">
<td>Feature Doc</td>
<td>Sharepoint (private)</td>
</tr>
<tr class="even">
<td>Sample Manifests and Policies From Feature Spec</td>
<td><a href="https://gist.github.com/alexkalish/8a514defa9e741800c095dded9837582">link</a></td>
</tr>
<tr class="odd">
<td>Kubernetes Documentation:Annotations</td>
<td><a href="https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/">link</a></td>
</tr>
<tr class="even">
<td>Exposing Pod Annotations to Containers</td>
<td><a href="https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/">link</a></td>
</tr>
<tr class="odd">
<td>Secrets Provider Documentation</td>
<td><a href="https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/11.2/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm?tocpath=Integrations%7COpenShift%252C%20Kubernetes%252C%20and%20GKE%7CDeploy%20Applications%7C_____3">link</a></td>
</tr>
<tr class="even">
<td>Secrets Provider application-mode Helm chart manifest</td>
<td><a href="https://github.com/cyberark/secrets-provider-for-k8s/blob/master/helm/secrets-provider/templates/secrets-provider.yaml">link</a></td>
</tr>
</tbody>
</table>

## Background

### Secrets Provider Push to File Support

The goal of the Milestone 1 (M1) Push-to-File initiative is to enhance the
Secrets Provider init container integration with an option for writing
secrets values that have been retrieved from Conjur into files that can be
directly accessed by an application that is running in the same Pod.

The intent is to make it much easier for developers to integrate their
applications directly with Conjur, without having to make significant changes
or additions to those applications or their containers; that is, without
having to add Summon to the application container or integrate a Conjur
client API into the application itself.

### Use of Kubernetes Annotations

The M1 Push-to-File feature requires a flexible way to allow developers or
deployers to map secrets that are to be retrieved from Conjur to specific
target file location(s). This will allow applications to consume secrets
from files at expected locations, further minimizing changes to the
application.

The M1 Push-to-File feature makes use of
[Kubernetes Pod Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/)
to provide a method of flexibly configuring the mapping of secrets to desired
target locations.

The M1 Push-to-File feature also adds support for configuring other
Secrets Provider container/application settings using Kubernetes annotations
as an alternative to using Pod environment variable settings. Allowing
Secrets Provider to be more generally configured via Pod annotations
provides a uniform mechanism for configuring the Secrets Provider, and should
lend itself to an easier upgrade/migration to the proposed CyberArk Dynamic
Sidecar Injector design.

## Solution

### User Experience

#### Annotations for Secrets Provider Container/Application Configuration

The M1 Push-to-File feature adds the capability of setting Secrets Provider
container/application configuration using Pod annotations.

When the Secrets Provider is configured for Push-to-File mode, then this
configuration must be set using Pod annotations. (Configuration via Pod
environment variables, i.e.
[legacy configuration](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/11.2/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm#Step4AddtheCyberArkSecretsProviderforKubernetesimagetotheapplicationdeploymentmanifest)
is not supported in Push-to-File mode.)

When the Secrets Provider is configured for Kubernetes Secrets mode, then
the user has the option of setting this configuration either via Pod
annotations or by using Pod environment variable settings.
If both a Pod Annotation and a Pod environment variable are configured for
a given Secrets Provider setting, then the Pod Annotation configuration
takes precedence.

_**QUESTION: Should we deprecate the current configuration method that uses
environment variable settings?**_

The following annotations are supported for Secrets Provider configuration.
Please refer to the
[Secrets Provider documentation](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/11.2/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm#Step4AddtheCyberArkSecretsProviderforKubernetesimagetotheapplicationdeploymentmanifest)
for a description of each environment variable setting:

| Annotation | Equivalent<br /> Environment Variable | Description, Notes |
|-----------------------------------------|---------------------|----------------------------------|
| conjur.org/<br />authn-identity         | CONJUR_AUTHN_LOGIN  | Required value. Example: `host/conjur/authn-k8s/cluster/apps/inventory-api` |
| conjur.org/<br />container-mode         | CONTAINER_MODE      | Allowed values: <ul><li>`init`</li><li>`application`</li></ul>Defaults to `init` |
| conjur.org/<br />secrets-destination    | SECRETS_DESTINATION | Required value. Allowed values: <ul><li>`file`</li><li>`k8s_secret`</li></ul> |
| conjur.org/<br />k8s-secrets            | K8S_SECRETS         | This list is ignored when `conjur.org/secrets-destination` annotation is set to `file` |
| conjur.org/<br />retry-count-limit      | RETRY_COUNT_LIMIT   | Defaults to 5
| conjur.org/<br />retry-interval-sec     | RETRY_INTERVAL_SEC  | Defaults to 1 (sec)              |
| conjur.org/<br />debug-logging          | DEBUG               | Defaults to `false`              |

#### Annotations for Push-to-File Secrets Mappings

##### Secret Groups

The configuration of the Secrets Provider push-to-file secrets mappings
makes use of "secret groups". A secret group is a logical grouping of
application secrets, typically belonging to a particular component of an
application deployment (e.g. all secrets related to a backend database).

As shown in the table below, all push-to-file annotations contain a reference
to a secret group, using the form: `conjur.org/<parameter-name>.<group>`.

There is typically a one-to-one correspondence between secret groups and
files that are written to the application Pod's Conjur secrets shared volume.
The one exception to this general rule is for the `plaintext` secrets file
format, for which there will be one secrets file per Conjur secret.

##### Supported Annotations

| Annotation                                     | Description |
|------------------------------------------------|-------------|
| conjur.org/conjur-secrets.{secret-group}       | List of secrets to be retrieved from Conjur. Each entry can be either:<ul><li>A Conjur variable path</li><li> A key/value pairs of the form `<alias>:<Conjur variable path>` where the `alias` represents the name of the secret to be written to the secrets file |
| conjur.org/conjur-secrets-policy-path.{secret-group}  | Prefix to use for all Conjur variable paths for a secret group. An example is `prod/backend/`. The path is assumed to be relative to root policy path. A leading `/` is ignored. A trailing `/` is optional. Defaults to the root policy path. |
| conjur.org/secret-file-path.{secret-group}     | Relative path for secrets file or directory to be written. This path is assumed to be relative to the respective mount path for the shared secrets volume for each container (for the Secrets Provider container, the mount path is `/conjur/secrets/`). (Refer to [Example Kubernetes Manifests and Helm Named Templates](#example-kubernetes-manifests-and-helm-named-templates) for example Volumes and VolumeMounts.)<br><br>If the `conjur.org/secret-file-template.{secret-group}` annotation is set, then this secret file path must also be set, and it must include a file name (i.e. must not end in `/`).<br><br>If the `conjur.org/secret-file-template.{secret-group}` annotation is not set, then this secret file path defaults to `{secret-group}.{secret-group-file-format}`. For example, if the secret group name is `my-app`, and the secret file format is set for `yaml`, then the secret file path defaults to `my-app.yaml`. |
| conjur.org/secret-file-format.{secret-group}     | Allowed values:<ul><li>yaml (default)</li><li>json</li><li>dotenv</li><li>bash</li><li>(future support) plaintext</li></ul>See the [Output File Formats](#output-file-formats) section below for example file formats.<br><br>This setting is ignored when `conjur.org/secret-file-template.{secret-group}` is configured. |
| conjur.org/secret-file-template.{secret-group} | Custom template for secret file contents in [Golang text template](https://golang.org/pkg/text/template/) format. Templates can include any of the [pre-defined Golang template functions](https://golang.org/pkg/text/template/#hdr-Functions). See [Custom Templates for Secret Files](#custom-templates-for-secret-files) section below. |

##### Example `conjur-secrets` Lists

###### Example `conjur-secrets` List Without Aliases

The following is an example of a `conjur.org/conjur-secrets.{secret-group}`
Annotation that defines a flat (i.e. no aliases) list of Conjur variable paths
for a hypothetical secret group named `database`:

```
    conjur.org/conjur-secrets.database: |
      - prod/backend/url
      - prod/backend/port
      - prod/backend/password
      - prod/backend/username
```

For `conjur-secrets` entries without aliases, the last word in the path is
used as the application secret name. So for the example above, if
no other annotations are set for the `database` secret group, then Secrets
Provider will write to a file `database.yaml` in the `/conjur/secrets/`
directory (the default value for the `conjur.org/secret-file-path.{secret-group}`
annotation) with contents that look like the following:

```
url: https://database.example.com
port: 5
password: my-secret-p@$$w0rd
username: postgres
```

###### Example `conjur-secrets` List With Aliases and `conjur-secrets-policy-path` Prefix

The following is an example of a `conjur.org/conjur-secrets.{secret-group}`
Annotation that defines a list of Conjur variable paths, including some that
use an alias, for a hypothetical secret group named `cache`:

```
    conjur.org/conjur-secrets.cache: |
      - url
      - admin-password: password
      - admin-username: username

    conjur.org/conjur-secrets-policy-path.cache: dev/memcached/

```

In this example, the paths for the secrets that are retrieved from Conjur are:

- dev/memcached/url
- dev/memcached/password
- dev/memcached/username

and the names of the secrets that are written to the secrets file are:

- url
- admin-password
- admin-username

so that the `cache.yaml` file that Secrets Provider writes includes:

```
url: https://database.example.com
admin-password: my-secret-p@$$w0rd
admin-username: admin
```

#### Custom Templates for Secret Files

In addition to offering standard file formats, Push to File allows users to
define their own custom secret file templates, configured with the
`conjur.org/secret-file-template.{secret-group}` annotation. These templates
adhere to Go's text template formatting. Providing a custom template will
override the use of any standard format configured with the annotation
`conjur.org/secret-file-format.{secret-group}`.

Injecting Conjur secrets into custom templates requires using the custom
template function `secret`. The action shown below renders the value
associated with <secret-alias> in the secret-file:

```
{{ secret "<secret-alias>" }}
```

##### Global Template Functions
Custom templates support global functions native to Go's text/template package. The following is an example of using template function to HTML escape/encode a secret value.

```
{{ secret "alias" | html }}
{{ secret "alias" | urlquery }}
```

If the value retrieved from Conjur for alias is "`<Hello@World!>`", the
following file content will be rendered, each HTML escaped and encoded,
respectively:

```
&lt;Hello;@World!&gt;
%3CHello%40World%21%3E
```

For a full list of global Go text template functions, reference the official
[text/template documentation](https://pkg.go.dev/text/template#hdr-Functions).

##### Execution "Double-Pass"

To avoid leaking sensitive secret data to logs, and to ensure that a
misconfigured Push to File workflow fails fast, Push to File implements a
"double-pass" execution of custom templates. The template "first-pass" runs
before secrets are retrieved from Conjur, and validates that the provided
custom template successfully executes given "REDACTED" for each secret value.
Redacting secret values allows for secure, complete error logging for
malformed templates. The template "second-pass" runs when rendering secret
files, and error messages during this stage are sanitized. Custom templates
that pass the "first-pass" and fail the "second-pass" require experimenting
locally to identify bugs.

NOTE: Documentation should make clear that user-supplied custom templates
should not branch conditionally on secret values. This may result in a
template first-pass execution that doesn't validate all branches of the
custom template.

##### Example Custom Templates: Direct reference to secret values
The following is an example of using a custom template to render secret data
by referencing secrets directly using the custom template function secret.

```
conjur.org/secret-file-path.direct-reference: ./direct.txt
conjur.org/secret-file-template.direct-reference: |
  username | {{ secret "db-username" }}
  password | {{ secret "db-password" }}
```

Assuming that the following secrets have been retrieved for secret group
direct-reference:

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

##### Example Custom Templates: Iterative approach

The following is an example of using a custom template to render secret data
using an iterative process instead of referencing all variables directly:

```
conjur.org/secret-file-path.iterative-reference: ./iterative.txt
conjur.org/secret-file-template.iterative-reference: |
  {{- range $index, $secret := .SecretsArray -}}
  {{- $secret.Alias }} | {{ $secret.Value }}
  {{- end -}}
```

Here, `.SecretsArray` is a reference to Secret Provider's internal array of
secrets that have been retrieved from Conjur. For each entry in this array,
there is a secret Alias and Value field that can be referenced in the custom
template.

Assuming that the following secrets have been retrieved for secret group
iterative-reference:

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

##### Example Custom Templates: PostgreSQL connection string

The following is an example of using a custom template to render a secret file
containing a Postgres connection string. For a secret group described by the
following annotations:

```
conjur.org/secret-file-path.postgres: ./pg-connection-string.txt
conjur.org/secret-file-template.postgres: |
  postgresql://{{ secret "dbuser" }}:{{ secret "dbpassword" }}@{{ secret "hostname" }}:{{ secret "hostport" }}/{{ secret "dbname" }}??sslmode=require
```

Assuming that the following secrets have been retrieved for secret group
postgres:

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

#### Secret File Attributes

The Secrets Provider will create with file permissions described in the
[Secret File Attributes](#secrets-file-attributes-1) subsection of the
[Security](#security) section below.

#### Input Validation for Annotations

Annotations will be validated as they are being parsed by the Secrets Provider
This input validation is done before any secrets values are retrieved from
the Conjur instance. The Secrets Provider will restart if any validation
error is detected.

##### Allowable Characters for Conjur Variable Paths

For the `yaml`, `json`, `bash`, and `dotenv` secrets file formats, the
`conjur.org/conjur-secrets.{secret-group}` annotation is expected to
contain a list of elements that can be a mix of:

- Conjur secret variable paths
- Key/value pairs of the form `{alias}: {Conjur secret variable path}`

The Secrets Provider will validate that each Conjur secret variable path that
is included in `conjur-secrets` mappings represent valid Conjur variable
paths, and that the variable name (the last word in the path) is no longer
than 126 characters.

##### Allowable Characters and Maximum Lengths for Secret Aliases

For the `yaml`, `json`, `bash`, or `dotenv` secrets file format, the
`conjur.org/conjur-secrets.{secret-group}` annotation can include key/value
pairs of the form `{alias}: {Conjur secret variable path}`. The secret
alias in this case will be used as a key when writing a rendered secret
(i.e. after secret is retrieved from the Conjur instance) as a key/value
pair to an output secrets file.

The number of characters allowed for keys used in YAML, JSON, bash,
or `.env` files varies based on format.

###### Aliases for the YAML Secrets File Format

The YAML standard allows only the **printable** subset of the Unicode
character set. (See the "Chapter 5. Characters" section of the
[YAML 1.2 Specification](https://yaml.org/spec/1.2/spec.html).

Of course, any key (i.e. alias) that the Secrets Provider writes to a secrets
file will ultimately be consumed by the target application. The target
application may be more restrictive (in comparison to the YAML specification)
in the range of keys that it expects or requires in its secrets key/value
pairs. However, the Secrets Provider should be flexible enough to handle
applications with the most liberal range of characters allowed as YAML keys.

The Secrets Provider will validate that aliases that are included in
`conjur.org/conjur-secrets.{secret-group]` mappings for YAML output file
format to contain only characters that are allowed per the YAML specification
as described above.

The YAML standard defines a maximum length of 1024 characters (see
[definition of **simple key**](http://yaml.org/spec/1.1/#simple%20key/)).
The Secrets Provider will validate that aliases that are included in
`conjur.org/conjur-secrets.{secret-group]` mappings for YAML output file
format are no longer than 1024 characters in length.

###### Aliases for the JSON Secrets File Format

The JSON standard allows strings that are composed of any Unicode codepoint
"except `"` or `\` or control characters". See the `string` syntax defined
on this [Introducing JSON document](https://www.json.org/json-en.html).

Here is an example found on the Internet of how liberal the JSON standard is
for allowable characters:

```
{
  "🐶🔫": "not nice, but still valid json"
}
```

The Secrets Provider will validate that aliases that are included in
`conjur.org/conjur-secrets.{secret-group]` mappings for JSON output file
format to contain only characters that are allowed for strings per the JSON
specification as described above.

The JSON specification does not directly impose restrictions on the length of
keys, however there is a restriction on the maximum length of an entire
JSON document which is 2097152 characters, which is equivalent to 4 MB
of Unicode string data. (There may be stricter restrictions imposed by the
Golang JSON parser). The Secrets Provider will validate that the rendered
secrets file content for a JSON format secrets file does not exceed the
maximum length allowed for a JSON document.

###### Allowable Characters for `bash` and `dotenv` Aliases

The Secrets Provider will validate that aliases that are included in
`conjur.org/conjur-secrets.{secret-group]` mappings for the `bash` and
`dotenv` output file formats conform to the
[POSIX name format](https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap03.html#tag_03_231),
that is, aliases must consist solely of alphanumerics (a-z, A-Z, or 0-9) and
underscores, and the first character cannot be a digit. The format can be
represented as the following regex:

```
[a-zA-Z_][a-zA-Z0-9_]*
```

#### Avoiding Leaking of Sensitive Secrets Data in Container Logs

When secrets files are rendered using secrets values that have been
retrieved from Conjur, we need to be careful that those sensitive secrets
values are not exposed in container logs as part of error messages
when rendering errors occur.

The
[Preventing Leaking of Sensitive Information in Error Logs](#preventing-leaking-of-sensitive-information-in-error-logs)
section below provides details as to how leaking of sensitive data is avoided
while rendering secrets file content.

#### Detecting Duplicate Secrets File Names

Because users have the option to explicitly name the output secrets file
name for each secret group (using the
`conjur.org/secret-file-path.{secret-group}` annotation), it is possible that
the chosen names for two or more secret groups could be identical. For the
M1 release of the Push-to-File feature, this will be considered a fatal
misconfiguration. The Secrets Provider will check for any duplications
in secrets file names between groups after all annotations have been parsed.

In a future release, we may consider adding concurrency mechanisms to allow
multiple secret groups to append to a shared target secrets file.

#### Detecting Duplicate Secrets Aliases Within a Group

For secret groups that use `yaml`, `json`, `bash`, or `dotenv` formatting
for secrets files, the Secrets Provider will detect when a secret alias is
being reused in annotation(s) for a given secret group. Reuse of a secret
alias is considered a fatal misconfiguration, and will cause the Secrets
Provider to fail (abort execution).

#### Conjur Connection Configuration

The current Secrets Provider design supports configuration of Conjur
connection information via the following Pod environment variables (please
refer to the
[Secrets Provider Documentation](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/11.2/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm#Step4AddtheCyberArkSecretsProviderforKubernetesimagetotheapplicationdeploymentmanifest)
for a descripion of each of these environment variables):

- CONJUR_ACCOUNT
- CONJUR_AUTHN_URL
- CONJUR_SSL_CERTIFICATE
- CONJUR_URL

The M1 Push-to-File feature provides a second option for configuring these
Conjur connection parameters using a Conjur Connection ConfigMap. The
application Deployment manifest must be configured to mount this ConfigMap
data using a `mountPath` value of `/conjur/connect`, as shown in the
[example Helm named template](#example-helm-partial-named-template-for-secrets-provider-init-container)
below. With this configuration, the Secrets Provider will look for
Conjur connection information in the following files:

- /conjur/connect/CONJUR_ACCOUNT
- /conjur/connect/CONJUR_AUTHN_URL
- /conjur/connect/CONJUR_SSL_CERTIFICATE
- /conjur/connect/CONJUR_URL

with each file containing the Conjur connection parameter corresponding to
the file's name. Any values that are found in these files take precedence
over the corresponding environment variable setting.

#### Exposing the Pod Name and Pod Namespace to the Secrets Provider Container

The current design supports exposing the Secrets Provider's Pod Name and Pod
Namespace to the Secrets Provider container through
[Kubernetes Downward API environment variables](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/).

The M1 Push-to-File supports a second method for exposing this Pod information
to the Secrets Provider container using a
[Kubernetes Downward API shared volume](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/).
This method requires that the application manifest use a `mountPath` of
`/conjur/podinfo` for the downward API volume as shown in the example
[Kubernetes manifests and Helm named templates](#example-kubernetes-manifests-and-helm-named-templates)
below. With this configuration, the Pod Name and Namespace will be made
available to the Secrets Provider container at the following locations:

- /conjur/podinfo/MY_POD_NAME
- /conjur/podinfo/MY_POD_NAMESPACE

(Any values that are set in the Pod info shared volume take precedence
over environment variable settings.)

#### Helm Named Templates to Eliminate Manifest Boilerplate

As described in the M1 Feature Doc,
our current guidance for taking an existing application Kubernetes manifest
and modifying it to add a Conjur authentication client (e.g. Conjur authn-k8s,
Secretless Broker, or Secrets Provider) init/sidecar container requires the
addition of many lines of YAML configuration. Much of this configuration
is fairly static (unchanging from deployment to deployment), and can be
considered to be repetitive "boilerplate" configuration.

For customers who use Helm to deploy their applications, the M1 Push to File
feature will include the development of
[named Helm templates](https://helm.sh/docs/chart_template_guide/named_templates/),
(sometimes called a "partials" or "subtemplates") to encapsulate this
boilerplate configuration and make it available for reuse. Customers will be
able to reference this partial configuration in their manifests via simple Helm
`include` statements. This will minimize the amount of lines that need to be
copy/pasted into Deployment manifests when adding Conjur authentication support.

##### Named template for Secrets Provider init container

  See [example in appendix](#example-helm-partial-named-template-for-secrets-provider-init-container).
  This named template will include:

  - Init container image specification
  - Volume mounts

  The only templated field in this named template is the Docker image
  (repo with version tag) for the Secrets Provider container:

  ```
  - image: {{ .Values.conjur.secretsProvider.image }}
  ```

  This named template can be included in an application manifest by
  adding the following (see
  [example Deployment manifest](#example-deployment-manifest)):

  ```
      {{ include "conjur-secrets-provider.init-container" . | indent 6 }}
  ```

##### Named template for Secrets Provider Volumes

  See [example in appendix](#example-helm-partial-named-template-for-secrets-provider-volumes).
  This named template will include the Volume definitions for:

  - Conjur secrets files
  - Conjur connection information
  - Conjur Pod info (for Kubernetes Downward API)
  - Conjur error logs (for redirected standard error during secrets file
    rendering)

  This named template can be included in an application manifest by
  including the following:

  ```
      {{ include "conjur-secrets-provider.volumes" . | indent 8 }}
  ```

##### Adding Named Templates to a Helm Chart as a Helm Dependency

For optimum user experience, the Secrets Provider named templates described
above will be published to a special Helm chart that will be created for
Conjur authentication named templates such as these. This will allow customers
to add the named templates to their application Helm charts as a
[Helm dependency](https://helm.sh/docs/helm/helm_dependency/).

Using a Helm dependency will eliminate the need for customers to copy/paste
the named templates directly into their application Helm charts.

The Helm chart for Conjur authentication named templates can be added
as a Helm dependency to an application Helm chart by adding the following
entry to the Helm chart's `Chart.yaml` file:

```
dependencies:
- name: conjur-named-templates
  version: "<Conjur named templates chart version>"
  repository: "https://cyberark.github.io/helm-charts"
```

#### Example Kubernetes Manifests and Templates

For reference,
[Kubernetes manifests and Helm named templates](#example-kubernetes-manifests-and-helm-named-templates)
for an example application that uses a Secrets Provider init container in
Push-to-File mode are listed in the Appendix.

#### Output File Formats

##### Example YAML Secrets File

```
url: https://database.example.com
admin-password: p@$$w0rd
admin-username: zappa
```

##### Example JSON Secrets File

```
{
   "url": "https://database.example.com",
   "admin-password": "p@$$w0rd",
   "admin-username": "zappa"
}
```

##### Example `bash` Secrets File

```
   export URL="https://database.example.com"
   export ADMIN_PASSWORD="p@$$w0rd"
   export ADMIN_USERNAME="zappa"
```

##### Example `dotenv` (.env) Secrets File

```
   URL="https://database.example.com"
   ADMIN_PASSWORD="p@$$w0rd"
   ADMIN_USERNAME="zappa"
```

##### (Future Support) Example `plaintext` Secrets Files

_**NOTE:** Support for the `plaintext` secrets file format is future work
that may be added after the Full-Featured (Certified) Community Release._

The `plaintext` secrets file format is intended to provide an easy upgrade
path to push-to-file functionality for Kubernetes Deployments that currently
[consume Kubernetes Secret values as files](https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets-as-files-from-a-pod).

When the `conjur.org/secret-file-format.{secret-group}` annotation is set to
`plaintext`, the Secrets Provider will create a secrets file for each Conjur
secret listed in the `conjur.org/conjur-secrets.{secret-group}` annotation.

The following applies to secrets files that are created by the Secrets
Provider for the `plaintext` file format:

- One secret file is created per Conjur secret.
- The secret file name is the secret name/alias.
- The secret file content is the Conjur secret value.
- If the `conjur.org/secret-file-path.{secret-group}` annotation is not
  explicitly set, the destination directory for `plaintext` secrets files
  defaults to `/conjur/secrets/{secret-group}`.
- If the `conjur.org/secret-file-path.{secret-group}` annotation is
  explicitly set, it must be a directory (i.e. it must end with a `/`
  character) or Secrets Provider will fail and restart.

For example, if the following annotations are included in a Pod spec manifest:

```
    conjur.org/conjur-secrets-policy-path.database: dev/database/
    conjur.org/secret-file-path.database: dev/postgres/
    conjur.org/conjur-secrets.database: |
      - url
      - admin-username: username
      - admin-password: password
```

Assuming the following secrets are retrieved from Conjur:

```
    dev/database/url:       "https://postgres.example.com"
    dev/database/username:  "admin"
    dev/database/password:  "open-$e$ame"
```

Then the Secrets provider will create three secrets files as follows:

| File Path | File Contents |
| --------- | ------------- |
| `/conjur/secrets/postgres/url` | `https://postgres.example.com` |
| `/conjur/secrets/postgres/admin-username` | `admin` |
| `/conjur/secrets/postgres/admin-password` | `open-$e$ame` |

### Performance Metrics

_**NOTE: Support for a performance metrics endpoint is considered a "best
 effort" feature for the early GA milestone. If this functionality cannot be
 realized within the milestone deadlines, then Secrets Provider push-to-file
 performance will be measured manually, perhaps using temporary instrumention
 of Secrets Provider code to measure latency.**_

For performance monitoring, the M1 push-to-file feature will add a metrics
HTTP endpoint that can be scraped by a Prometheus server. As described in
the [Performance](#performance) section below, the following latency
metrics will be supported:

- Latency (processing time) for parsing Pod annotations
- Latency for generating secrets requests and processing responses
- Latency for writing retrieved secrets to files

Performance metrics for Secrets Provider CPU and memory footprints is
considered a nice-to-have feature.

### Upgrading Existing Secrets Provider Deployments

At a high level, converting an existing Secrets Provider deployment to use
annotation-based configuration and/or push-to-file mode:

- Inspect the existing application Deployment manifest (if available) or
  use `kubectl edit` to inspect the application Deployment.
- Convert the Service Provider container/Conjur environment variable settings
  to the equivalent annotation-based setting. Retain the `K8S_SECRETS`
  setting for now.
- If you are using the Secrets Provider as an init container, and you would
  like to convert from K8s Secrets mode to push-to-file mode:
    - Add push-to-file annotations:
        - For each existing Kubernetes Secret, you may wish to create a separate
          secret group for push-to-file.
        - `conjur.org/conjur-secrets.{group}`: Inspect the manifests for the
          existing Kubernetes Secret(s). The manifests should contain a
          `stringData` section that contains secrets key/value pairs.
          Map the `stringData` entries to a YAML list value for conjur-secrets,
          using the secret names as aliases.
        - `conjur.org/secret-file-path.{group}`: Configure a target location
        - `conjur.org/secret-file-format.{group}`: Configure a desired type,
          depending on how the application will consume the secrets file.
    - Delete existing Kubernetes Secrets or their manifests:
        - If using Helm, delete Kubernetes Secrets manifests and do a
          `helm upgrade ...`
        - Otherwise, `kubectl delete ...` the existing Kubernetes Secrets
    - Delete the `K8S_SECRETS` environment variable setting from the application
      Deployment (or its manifest).
    - Modify application to consume secrets as files:
        - Modify application to consume secrets files directly, or...
        - Modify the Deployment's spec for the app container so that the
          `command` entrypoint includes sourcing of a bash-formatted secrets file.

### Project Scope and Limitations

The initial implementation and testing will be limited to:

- Authentication containers to be tested:

  - Secrets Provider init container
    - Use existing tests for environment-variable-based configuration.
    - Add new tests for annotation-based configuration, including
      Push-to.File functionality.
  - Secrets Provider standalone Pod
    - Use existing tests for environment-variable-based configuration.
    - Add new tests for annotation-based configuration, excluding
      Push-to.File functionality.

- Platforms:

  - Kubernetes (this will be either Kubernetes-in-Docker, or GKE).
  - OpenShift 4.8

- Scale:

  Scaling limits for the M1 Push-to-File feature will remain at the
  current limits defined for the Secrets Provider:

  - Support for up to 50 DAP secrets per Secrets Provider init container,
    where the variable paths are, on average, 100 characters.

#### Out of Scope

- For this release, configuration will only be read upon startup.
- For this release,
  [push-to-file annotations](#annotations-for-push-to-file-secrets-mappings)
  are required to have a group.
- For this release, support for custom templates will only include support
  for the standard,
  [built-in Go template functions](https://golang.org/pkg/text/template/#hdr-Functions).
  In future releases, we will consider adding support for things like:

  - Functions based on the standard
    [Go `strings` module](https://golang.org/pkg/strings/).
  - Functions in the
    [`sprig` template function library](http://masterminds.github.io/sprig/).

  Adding support for functions in these modules would allow for use of
  functions such as `ToLower`, `Repeat`, `Replace`, `TrimRight`, etc.
  Adding support for these functions would require explicitly including each
  desired function in a Go template `funcMap`, after doing a Security analysis
  to assure that the function is safe. Essentially, we would be implementing an
  "allow list" on a per-function basis.

##  Design

### Flow Diagram

![Secrets Provider Push to File Flow Diagram](./m1_push_to_file_flow.png)

### Data Model

#### Per-Group Secrets Mapping

As shown in the flow diagram above, the Secrets Provider will need to parse
all Pod annotations, and compile an array of per-group secrets mapping
information. The structure below portrays the information that will
need to be gathered.

Note that this structure is shown for conceptual purposes; the names
of fields used in the actual code may change through the development process:

```
// SecretSpec specifies a secret to be retrieved from Conjur by defining
// its alias (i.e. the name of the secret from an application's perspective)
// and its variable path in Conjur.
type SecretSpec struct {
	Alias string
	Path  string
}

// SecretGroup incorporates all of the information about a secret group
// that has been parsed from that secret group's Annotations.
type SecretGroup struct {
    Name             string
    FilePath         string
    FileTemplate     string
    FileFormat       string
    PolicyPathPrefix string
    FilePermissions  os.FileMode
    SecretSpecs      []SecretSpec
}

// SecretGroups comprises secrets mapping info for all secret groups
var SecretGroups map[string]SecretGroup{}
```

### Workflow

1. When Secrets Provider container starts up, it has Pod annotations available
   in a file named `annotations` in a Pod info volume. Each entry in this
   file will be a YAML key/value pair, and can be either:

    - Secrets Provider container/application configuration
    - Push-to-file configuration

1. Secrets Provider parses the YAML in the `annotations` file, and iterates
   over all entries.

    - For SP container/application configuration, values are written to
      the SP `Config` structure.

    - For Push-to-File configuration, each annotation **key** will be parsed and
      split into three fields:

        - Annotation type (e.g. `conjur-secrets`, `conjur-secrets-policy-path`, etc)
        - Secret group
        - Annotation value. The annotation value is a string that can be any
          of the following formats:

            - Plain string
            - YAML list of secrets
            - Secrets file Golang template

    - If the annotation **value** is a YAML list of secrets, the SP will then
      iterate over each entry in the YAML list. Each entry is parsed into
      two fields:

        - Conjur variable path
        - Secret alias. If not provided explicitly, then the last word in
          the Conjur variable path is used as an alias.

    - Depending upon the annotation type, values are then written to a
      per-Group `SecretGroup` structure (see definition in the
      [Per-Group Secrets Mapping](#per-group-secrets-mapping) section above.

1. After all annotations have been processed, Secrets Provider iterates
   through the `SecretGroup` data structures to compile a complete set of
   secrets (over all secret groups) to be retrieved from Conjur. The Secrets
   Provider then connects/authenticates with Conjur and retrieves all
   secret values.

1. The Secrets Provider iterates again through all `SecretGroup` data
   structures to render and write a secret file for each group as follows:

    - If a secrets file Golang template has not been provided explicitly,
      Secrets Provider will use a "canned" Golang template based on the
      secrets file format (YAML, JSON, dotenv, or bash).
    - Secrets Provider connects/authenticates with Conjur and retrieves all
      secrets for that secret group.
    - Secrets Provider then renders and writes a secrets file at the configured
      destination file path.

## Performance

From a high level, the performance of the Secrets Provider running in
push-to-file mode will be a measure of how fast the Secrets Provider can
perform the following as an init container:

- Parse Pod annotations
- Retrieve secrets for all secret groups from Conjur
- Write those secrets to a volume that is shared with the application container

To maximize the performance of Conjur secrets retrieval, the Secrets Provider
will retrieve secrets from Conjur using batch retrievals on a
per-secrets-group basis. In addition, retrieval of secrets from Conjur will
be done using parallel Goroutines (with the maximum number of Goroutines
allowed TBD). It is therefore expected that the throughput in terms of
the rate of secrets that the Secrets Provider can retrieve from Conjur
should not differ significantly for Push-to-File mode versus Kubernetes
Secrets mode.

For performance monitoring, the M1 push-to-file feature will add a metrics
HTTP endpoint that can be scraped by a Prometheus server. This includes
support for the following latency metrics:

- Latency (processing time) for parsing Pod annotations
- Latency for generating secrets requests and processing responses
- Latency for writing retrieved secrets to files

As described in the [Performance testing](#performance-testing) section below,
performance will be measured using a representative configuration in terms of
the number of secret groups and the number of secrets per secret group.

## Backwards Compatibility

For backwards compatibility, the Secrets Provider must continue to support
the environment-variable based configuration for container and
Conjur configuration for the Kubernetes Secrets mode of operation. For
push-to-file mode, only annotation-based configuration will be tested and
supported.

## Affected Components

This feature affects the Secrets Provider component, for both init/sidecar
operation and standalone (application) mode. No other components will be
affected.

## Test Plan

### Test environments

To validate support, E2E test cases will need to be run against different versions of
- Kubernetes
- Openshift

### Test assumptions

### Out of scope

### Test prerequisites

Unit tests require
- Assertion library (stretchr/testify)
- Mocks for Conjur server, parser, secret fecher, and file writer
- Code is dependency injectable
- Environment with all the dependencies loaded. A container image is typically defined that contains all the dependencies for running the tests (Go + tooling) and any postprocessing of results (junit)

E2E tests require
- Kubernetes/Openshift cluster
- Conjur setup and ready. Policy will need to be run to define an authenticator, secrets and permissions.
- Secrets provider image is available to Kubernetes/Openshift cluster

### Test Cases

#### Unit Tests: Basic "Happy Path" Functionality

In the following table, the entries in the "User Stories" column refer to user stories as defined in the
M1 Feature Document.

| | Section | Given | When | Then | User Stories |
| --- | --- | --- | --- | --- | --- |
| 1 | Secrets Provider container can be configured using annotations | You provide SP config as annotations | Parsing is carried out | The internal SP config is set base on annotations | US-03 |
| 2 | Annotation configuration takes precedence over environment variables | You provide the annotations and environment variables | Parsing is carried out | The internal representation of the parsed configuration prioritised values from the annotations | US-03 |
| 3 | Parsing of Conjur secrets list | Valid "conjur-secrets" list annotation with aliases | Parsing is carried out | The internal representation matches expectations  | US-05 |
| 4 | Parsing of Conjur secrets list | Valid "conjur-secrets" list annotation without aliases | Parsing is carried out | The internal representation matches expectations  | US-05 |
| 5 | Parsing of Conjur secrets list | Valid "conjur-secrets" list annotation with mix of entries with/without aliases | Parsing is carried out | The internal representation matches expectations  | US-05 |
| 6 | Parsing of Conjur secrets list | Empty list defined for "conjur-secrets" list annotation (allowed for custom template without secrets) | Parsing is carried out | No errors encountered | |
| 7 | Parsing of Conjur secrets path | Valid "conjur-secrets" list and "conjur-secrets-policy-path" annotations | Parsing is carried out | The internal representation matches expectations  | US-06 |
| 8 | Parsing multiple secret group annotations | Annotations for multiple secret groups with different (including default) file formats | Parsing is carried out | The annotation are divided according to their groups in the internal representation  | US-04 |
| 9 | Default value for relative secrets file path | "secret-file-path" annotation not configured | Parsing is carried out | The secret file is written to `/conjur/secrets/{secret-group}.{secret-file-format}` | US-07 |
| 10 | Parsing of relative secrets file path | "secret-file-path" annotation set to subdirectory (no filename) | Parsing is carried out | The secret file is written to `/conjur/secrets/{secret-file-path}/{secret-group}.{secret-file-format}` | US-07 |
| 11 | Parsing of relative secrets file path | "secret-file-path" annotation set to subdirectory + filename | Parsing is carried out | The secret file is written to `/conjur/secrets/{secret-file-path}` | US-07 |
| 12 | Parsing of relative secrets file path | "secret-file-path" annotation set to subdirectory + filename with extension not matching file format | Parsing is carried out | The secret file is written to `/conjur/secrets/{secret-file-path}` | US-07 |
| 13 | Default value for secrets file format | "secret-file-format" annotation is not set | Parsing is carried out | The default secrets file format set to `yaml` | US-08 |
| 14 | Parsing of secrets file format | Valid "secret-file-format" annotation (`yaml`, `json`, `dotenv`, `bash`, or [for future support] `plaintext`) | Parsing is carried out | The internal representation matches expectations  | US-08 |
| 15 | Parsing of custom secret file template | "secret-file-template" annotation referencing secrets by name (alias) | Parsing is carried out | The internal representation matches expectations  | US-09 |
| 16 | Parsing of custom secret file template | "secret-file-template" annotation iterating over all secrets | Parsing is carried out | The internal representation matches expectations  | US-09 |
| 17 | Parsing of custom secret file template | "secret-file-template" annotation using built-in template functions (e.g. `printf` or `html`) | Parsing is carried out | The internal representation matches expectations  | US-09 |
| 18 | Parsing of custom secret file template | "secret-file-template" annotation using custom functions with nested templates | Parsing is carried out | The internal representation matches expectations  | US-09 |
| 19 | Batch retrieval of secrets | A mock for Conjur server batch retrieval exists | Fetch secret logic is run | Returns expected payload | |
| 20 | Table test for generation of file formats | Some mock secret file data | File generation code is executed for various file formats | The generated files match the format | US-08 |
| 21 | File creation with custom secret file template | "secret-file-template" annotation referencing secrets by name (alias) | File created | The file contents match expectations  | US-09 |
| 22 | File creation using custom secret file template | "secret-file-template" annotation iterating over all secrets | File created | The file contents match expectations  | US-09 |
| 23 | File creation using custom secret file template | "secret-file-template" annotation using built-in template functions (e.g. `printf` or `html`) | File created | The file contents match expectations  | US-09 |
| 24 | Helm test using Named Templates as Chart Dependency | SP init container and Volume named templates are published | 'helm template' is run on a sample deployment Helm chart that uses named templates as a Helm dependency | The generated deployment manifests is as expected | US-01 |
| 25 | Prometheus metrics endpoint testing | The SP is configured to create a sampling of secrets files | The metrics endpoint is queried for latency measurements | The latency metrics in the response are as expected | |
| 26 | (Future support) File generation for `plaintext` format using default destination directory | Some mock secret file data | File generation code is executed for `plaintext` format without explicit dest directory | The generated files are as expected | |
| 27 | (Future support) File generation for `plaintext` format using explicit destination directory | Some mock secret file data | File generation code is executed for `plaintext` format with explicit dest directory | The generated files are as expected | |

#### Error Handling / Recovery / Supportability Tests

| | Section | Given | When | Then | Error Code | Test Type (Unit, Integration, E2E) | User Story |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Missing secrets destination | You are using SP | You do not provide the secrets destination in an annotation or environment variable | Error: Missing secrets destination. Acceptable values are "k8s" and "file".<br><br>SP fails to start | | Unit | |
| 2 | Invalid input file format | You are using SP in "file" mode | You have provided an unknown or unsupported type in the secret-file-format annotation | Error: Invalid output file format: "{value}" for secret group "{value}". Acceptable values are "json", "yaml", "bash", "dotenv", and "plaintext".<br><br>SP fails to start | | Unit | |
| 3 | Conjur variable names collision | You are using SP in "file" mode | Two or more secrets in the same Secret Group share the same name or alias. | Error: Multiple variables in the "{value}" secret group are called "{value}". Provide a unique alias for each variable in the "{value}" secret group.<br><br>SP fails to start | | Unit | |
| 4 | Unparseable Conjur secrets list | You are using SP in "file" mode | The "conjur-secrets" annotation is not parseable for one or more secret groups. | Error: The list of secrets for the "{value}" secret group is not formatted correctly. Error: "{value}". Verify that the annotation value is a YAML list with optional keys for each list item.<br><br>SP fails to start | | Unit | |
| 5 | Invalid Conjur secrets path | You are using SP in "file" mode | The "conjur-secrets-policy-path" annotation is not parseable as a full secret path prefix. | Error: The Conjur secrets path "{value}" provided for the "{value}" secret group is invalid. | | Unit | US-06 |
| 6 | Conjur variable path too long | You are using SP in "file" mode | The "conjur-secrets" annotation contains variable path(s) that are too long for one or more secret groups. | Error is displayed which includes all improper variable paths.<br><br>SP fails to start | | Unit | |
| 7 | Secret alias for YAML secrets file too long | You are using SP in "file" mode for YAML format | The "conjur-secrets" annotation contains a variable alias that is longer than 1024 characters. | Error is displayed which includes improper variable alias.<br><br>SP fails to start | | Unit | |
| 8 | Rendered file content for JSON secret file too long | You are using SP in "file" mode for JSON format | The rendered file content is longer than the max allowed for a JSON document (2097152 chars, equivalent to 4 MB Unicode string). | Error is displayed which includes secret group and rendered content length.<br><br>SP fails to start | | Unit | |
| 9 | Secret alias for `bash` or `dotenv` format with invalid characters | You are using SP in "file" mode for `bash` or `dotenv` format | The "conjur-secrets" annotation contains a variable alias that includes invalid characters for an environment variable name. | Error is displayed which includes improper variable alias.<br><br>SP fails to start | | Unit | |
| 10 | Invalid file template definition – not parseable by Go | You are using SP in "file" mode | File template for one or more secret groups cannot be used as written. | Error: The file template for the "{value}" secret group cannot be used as written. Error: "{value}". Update your template definition to address this and try again.<br><br>SP fails to start | | Unit | US-09 |
| 11 | Invalid file template definition – references undefined secret keys | You are using SP in "file" mode and have provided a value for the `conjur.org/secret-file-template` annotation | File template for one or more secret groups references secrets that do not exist in the secret group. | Error: The file template for the "{value}" secret group references the "{value}" secret, but no such secret is defined in the secret group. Add an alias to set the correct secret name to "{value}", and try again.<br><br>SP fails | | Unit | US-09 |
| 12 | Unable to retrieve Conjur variables | You are using SP in "file" mode | | Error: Failed to provide DAP/Conjur secrets<br><br>SP fails | CSPFK016E | Unit | |
| 13 | Missing volume mounts | You are using SP in "file" mode | One or more volumes cannot be found by Secrets Provider. | Error: Unable to access volume "{value}". Error: "{value}". Ensure that the correct volumes are defined in the deployment manifest and attached to the Secrets Provider container.<br><br>SP fails to start | | Unit | US-07 |
| 14 | Missing Conjur variable value while rendering custom template file content | You are using SP with a custom secrets file template | One or more variables used in a custom template are not included in secrets list. | Error displays missing variable name, and must not include sensitive secrets values.<br><br>SP fails | | Unit | |
| 15 | Sanitize errors while rendering file content | You are using SP in "file" mode | An error is encountered while rendering secrets file content. | Errors must not include sensitive secrets values.<br><br>SP fails | | Unit | US-07 |
| 16 | File permissions error | You are using SP in "file" mode | Secrets Provider does not have permission to write a secrets file to the specified volume and mount path. | Error: Secrets Provider does not have permission to write the secrets file "{value}". Please check the volume mount permissions on the Secrets Provider container and try again.<br><br>SP fails | | Unit | US-07 |
| 17 | Out of disk space error | You are using SP in "file" mode | Secrets Provider does not have sufficient disk space to write a secrets file. | SP displays error as reported by operating system.<br><br>SP fails | | Unit | US-07 |
| 18 | Invalid file path provided | You are using SP in "file" mode and have provided a value for the `conjur.org/secret-file-path` annotation | The provided file path is not a valid file path. | Error: You have provided an invalid file path "{value}" for the "{value}" secret group.<br><br>SP fails to start | | Unit | US-07 |
| 19 | File conflict between two secret groups | You are using SP in "file" mode and have defined multiple secret groups | One or more secret groups have the same ouput file. | Error: You have provided conflicting file paths; secret groups "{value}", "{value}", ... all have output file "{value}".<br><br>SP fails to start | | Unit | US-07 |
| 20 | Template provided but no output file defined | You are using SP in "file" mode and have provided a file template definition | You have not also provided an output file path. | Error: The output file path is missing for the "{value}" secret group. File path is required when using a custom template.<br><br>SP fails to start | | Unit | |
| 21 | Invalid retry annotation settings | You are using SP in "file" mode and have provided retry configuration as annotations | Retry settings re invalid. | Error is generated indicating incorrect setting(s).<br><br>SP fails to start | | Unit | |
| 22 | Conjur retrieval fails and retry is required | You are using SP in "file" mode and have provided retry configuration as annotations | Conjur retrievals fail file path. | Secrets Provider retries up the the retry count limit.<br><br>SP fails to start | | Integration | |
| 23 | (Future support) Invalid file path for `plaintext` format | You are using SP in "file" mode, you are using `plaintext` secrets path format, and have provided a value for the `conjur.org/secret-file-path` annotation | The provided file path is not a directory. | Error: The provided secret file path for the {value} secret group is not a directory. You must provide a directory for the "plaintext" secret file format.<br><br>SP fails to start | | Unit | |

#### E2E tests

E2E tests are expensive. We should try to limit their numbers. There are cheaper ways to validate behavior. For the push to file functionality it will likely be possible to validate the happy path in one fell swoop by using configuration tht captures all the supported annotations and template types.

- [ ] Happy path, Secret Provider runs as an init container in file mode and successfully injects files. This single test can cover all supported annotations and template types.

The sad path is where things tend to become difficult, because there's a myriad of ways in which failure can occur. A possible way to reduce the need to E2E test failures is to establish a nexus point for all failures in the code so that an error can be validated at the unit/functional testing level.

- [ ] Sad path, Secret Provider runs as an init container in file mode and fails, thereby preventing the deployment from proceeding. TBC it's possible we don't need this at all.

#### Security testing

Security testing will include:

- Automated Vulnerability scans

####  Performance testing

As described in the [Performance](#performance) section above, the M1
push-to-file feature will add support for an HTTP metrics endpoint that
can be used to collect the following metrics:

- Latency (processing time) for parsing Pod annotations
- Latency for generating secrets requests and processing responses
- Latency for writing retrieved secrets to files

The measurements will be made using a representative configuration in terms
of the number of secret groups and the number of secrets per secret group.
Details are TBD, but a representative configuration might look something like
this:

- 5 secret groups
- 5 secrets / secret group
- Mix of secret value lengths (10 to 100 characters, plus some base64-encoded
  TLS certificates)

This profile represents about half of the 50 Conjur secrets per Secrets
Provider init container scaling limit as described in the
[Project Scope and Limitations](#project-scope-and-limitations)
section below.

## Logs

For "happy path" operation, the M1 Push-to-File feature will add INFO level
logs to report status at least for significant checkpoints in the Push-to-File
workflow:

- Start of annotation parsing
- Completion of annotation parsing
- Start of validation for each secret group
- Start of secrets retrieval for each secret group
- Start of custom template parsing (if applicable) for each secret group
- Start of file rendering for each secrets file
- Start of file writing for each secrets file
- Completion of file writing for each secrets file

For error conditions, the Secrets Provider will include ERROR-level logs.
Error messages for each error condition are described in the
[Error Handling / Recovery / Supportability Tests](#error-handling-recovery-supportability-tests) table below.

Some general comments that can be made about error logging:

1. Annotation values are treated as non-sensitive and will not be redacted
   in log messages.
2. For forward compatability (i.e. future-proofing), annotation keys with
   the prefix `conjur.org` but with an unexpected annotation type will be
   logged at `Info` level as unknown annotation type, but otherwise will
   be ignored.
3. Care will be taken to prevent leaking of sensitive secrets values during
   the rendering of secrets file content as described in the
   [Preventing Leaking of Sensitive Information in Error Logs](#preventing-leaking-of-sensitive-information-in-error-logs)
   section below.

## Documentation

Documentation for the M1 Secrets Provider Push to File feature will include:

- Update the
  [Secrets Provider init container configuration documentation](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ic.htm).
- Update the [Secrets Provider application container configuration documentation](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ac.htm).
- Add new documentation to describe the push-to-file annotation configuration
- Add documentation for the process to upgrade from the Secrets Provider legacy (environment variable based) configuration to the annotation-based configuration.

## Version update

The M1 Push-to-File feature will necessitate a minor version bump for
the Secrets Provider. Because configuration will maintain backwards
compatibility, it is not expected that a major version bump will be required.

## Security

### Secret File Attributes

Since the M1 Push-to-File feature involves writing sensitive information to
secrets files that are shared by an application container, we need to consider
whether the secrets files are being created with the appropriate access
permissions.

By default, the Secrets Provider will create secrets files with the following
file attributes:

|  Attribute  |       Value        | Notes  |
| ----------- | ------------------ | ------ |
| User        | `secrets-provider` |        |
| Group       | `root`             | See [Why Use The `root` Group?](#why-use-the-root-group) |
| UID         | `777`              |        |
| GID         | `0`                | See [Why Use The `root` Group?](#why-use-the-root-group) |
| Permissions | `rw-rw-r--`        | See [Why Use World-Readable Permissions?](#why-use-world-readable-permissions) |

#### Why Use The `root` Group for Secrets Files?

OpenShift requires that any files/directories that are shared between
containers in a Pod must use a GID of `0` (i.e. GID for the `root` group),
as described in this
[Guide to OpenShift and UIDs blog](https://www.openshift.com/blog/a-guide-to-openshift-and-uids).
The Secrets Provider uses a GID of `0` for secrets files even for
non-OpenShift platforms, for simplicity.

#### Why Use World-Readable Permissions?

As shown in the table table, the Secrets Provider will create secrets files
that are world readable. This means that the files will be readable from any
other container in the same Pod that mounts the Conjur secrets shared Volume.

This permissions value is used to allow for other containers in a
non-OpenShift platform and in the same Pod as the Secrets Provider to be
run with any arbitary user UID and GID and still be able to read the secrets
files, even if the UID and GID do not match those of the secrets files
that have been created by the Secrets Provider.

Note that this potential for permissions/user mismatch is not a concern
in OpenShift platforms. As described in
[this blog](https://www.openshift.com/blog/a-guide-to-openshift-and-uids),
OpenShift avoids the potential for UID/GID mismatches between containers
in a Pod by:

- Running all containers using an arbitrary (assigned at run time),
  unprivileged, non-root user.
- Running all containers using a GID of `0` (a.k.a. the `root` GID).
- Requiring that all Volumes to be written have permissions that allow
  writing and executing by anyone on the common group (i.e. GID of `0`).

From a security perspective, making secrets files world-readable throughout
the Pod (for the sake of non-OpenShift platforms) is no more permissive than
the OpenShift approach, whereby files created in a shared Volume are
readable from all containers in the same Pod that mount that Volume, because
the containers are run with the common GID of `0`.

An alternative to creating the secrets files with world-readable permissions
that had been considered is:

- Create secrets files with permissions `rw-rw----` (octal `660`).
- Document how users can run their application Pods with arbitrary UID and GID
  and still be able to read secrets files in shared Volumes by including
  [PodSecuritContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#podsecuritycontext-v1-core)
  definitions in their application manifests to override, among other things
  (see the
  [PodSecuritContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#podsecuritycontext-v1-core) for a complete list):

  - Run-time UID for all containers in a Pod (`runAsUser`)
  - Run-time GID for all containers in a Pod (`runAsGroup`)
  - GID used for certain Volume types (including emptyDir Volumes) and all
    files created in those Volumes (`fsGroup`)

  For example, the following configuration can be included in an application
  Deployment's Pod specification to configure all containers in the Pod to
  run with UID of `65534` (i.e. the `nobody` user) and a GID of `65534`
  (i.e. the `nobody group), with all files that are created in `emptyDir`
  Volumes having an owner GID of `65534`:

  ```
      securityContext:
        runAsUser: 65534
        runAsGroup: 65534
        runAsNonRoot: true
        fsGroup: 65534
  ```

The disadvantage of this approach is that it would need to be heavily
documented, and could be prone to user error.

### Preventing Leaking of Sensitive Information in Error Logs

When secrets files are rendered using secrets values that have been
retrieved from Conjur, we need to be careful that those sensitive secrets
values are not exposed in container logs as part of error messages
when rendering errors occur.

#### Sanitizing Errors for `yaml`, `json`, `bash`, and `dotenv` Secrets File Formats

When secrets files are being created using the `yaml`, `json`, `bash`, or
`dotenv` file formats, leaking of sensitive secrets values will be prevented
by using the following workflow for rendering each secrets file:

- Retrieve all secrets values for a secret group from Conjur, and populate
  those values in a secrets file information array.
- Iterate through all entries in the secrets file info array.
  For each entry:
  - Attempt to render the line of output for that secrets setting, redirecting
    standard error to `/dev/null`.
  - If an error occurs, print a sanitized error to standard out, excluding
    the sensitive secret value, but including the secret alias and Conjur
    variable path.
- If rendering is successful for all secrets values, write the rendered
  output to `/conjur/secrets/{secrets-file}`.

#### Preventing Leaking of Sensitive Information While Executing Custom Templates

In general, processing of a Golang template is done in two steps:

- Parse the template
- Execute the template to render output text, using a map of values
  to be used for rendering

For the Secrets Provider push-to-file feature, the first step does not
involve handling of sensitive secrets retrieved from Conjur, so parsing
errors can be included in container logs without any sanitizing.

However, the second step involves the rendering of secrets file content using
sensitive secrets information that has been retrieved from Conjur. Depending
upon the custom template provided by the user, it is entirely possible that
the Go template execution could produce errors that include this sensitive
information. To prevent these error messages from leaking into container
logs while the custom template is being executed, the Secrets Provider will
process custom templates in three steps:

- Parse the template
- Do a "test" execution of the template to render "dummy" secrets file
  content, using a map of dummy values to use for rendering, as follows:
  - Dummy values will be included in this values map for all secrets that are
    defined in the corresponding secrets list.
  - The dummy values will be strings that contain the word "REDACTED". 
  - All errors that occur during the "test" template execution will be
    redirected to standard error with no sanitizing, so that the errors will
    be available in Secrets Provider container logs.
  - This "test" execution will be able to detect what will probably be the
    most likely user misconfiguration error when custom templates are used: 
    A custom template uses a Conjur secret that is not included in the
    corresponding secrets list for that secret group.
- Execute the template to render secrets file content, while redirecting
  all "raw" output to `/dev/null`. If any errors occur, a non-specific
  error message will be generated that includes the custom template
  definition, but no secrets values.


The last step will hide explicit template execution errors from the user
in the very rare cases where the custom template being used is sensitive to
actual secrets values (for the sake of security). It is expected that in
these rare cases, the user will be able to troubleshoot the failure "out of
band" by processing the custom template on a local machine, using secrets
values retrieved from Conjur to render secrets file content.

Some alternatives to this approach that we considered include:

- During template execution, redirect all output to a log file that
  resides in an ephemeral (i.e. `emptyDir`) volume. If errors occur, a
  privileged user with sufficient permissions to `kubectl exec ...`
  inside the Secrets Provider container would be able to read the
  template execution errors.
- During template execution, log all errors at the DEBUG logging level.
  Template execution errors would be hidden unless the user has
  sufficient authorization to enable DEBUG logging level for the Secrets
  Provider.
- Implement another annotation that can be used (on an "opt-in" basis)
  to enable inclusion of possibly sensitive execution error messages in
  Pod/container logs.

## Audit

No changes to Conjur audit behavior are required for this feature.

##  Development Tasks

Development tasks for this feature are organized into tasks corresponding to
three phases of development:

- Minimally-featured community release
- Minimally-featured GA release
- Full-featured GA release

### Development Tasks: Minimally-Featured Community Release

- [ ] Solution design approval
- [ ] Security review approval
- [ ] Community and Integrations team ramp-up for becoming familar with
      existing Secrets Provider testing (unit, integration, E2E)
- [ ] Refactor SP config: Separate SP config into the following:
    - [ ] Container config:
        - PodName - MY_POD_NAME
        - PodNamespace - MY_POD_NAMESPACE
        - RetryCountLimit - RETRY_COUNT_LIMIT
        - RetryIntervalSec - RETRY_INTERVAL_SEC
        - StoreType – SECRETS_DESTINATION
    - [ ] Kubernetes Secrets config:
        - RequiredK8sSecrets – K8S_SECRETS
- [ ] Existing SP config options can be set by annotations
    - [ ] Container config:
        - PodNamespace - MY_POD_NAMESPACE (still comes from downward API)
        - RetryCountLimit - `conjur.org/retry-count-limit`
        - RetryIntervalSec - `conjur.org/retry-interval-sec`
        - StoreType - `conjur.org/secrets-destination`
    - [ ] Kubernetes Secrets config:
        - RequiredK8sSecrets – `conjur.org/k8s-secrets`
- [ ] Refactor Authn Client config: Separate Authn Client config into the following:
    - Container config
    - Conjur config
    - App identity config
- [ ] Existing Authn Client config options can be set by annotations:
    - Container config
    - Conjur config
    - App identity config
- [ ] Define data structures for annotation parsing
- [ ] Given list of secrets with aliases, populate data structure(s)
    - [ ] Add logic to parse annotation keys: Split annotation keys into:
        - secret group name
        - Push-to-File Annotation type
        - Annotation value (can be a YAML formatted string)
    - [ ] Add logic to parse annotation values:
        - Parse `conjur.org/conjur-secrets.{secret-group}` (with aliases)
- [ ] Given populated data structure, get secrets and write to YAML file
- [ ] SP upgrade process has been validated - init container, env to annotations-based config, k8s secrets to push-to-file
- [ ] SP upgrade process has been validated - job, env to annotations-based config
- [ ] Dap-wiki docs have updated info on new flows - initial community release
- [ ] Custom upgrade instructions are documented (upgrade application to use SP with annotations and push-to-file)
- [ ] Quick start for SP in file mode, secrets with aliases, YAML secrets file
- [ ] UX has been reviewed and issues have been corrected (file mode, secrets aliases, YAML secrets file)
- [ ] SP is release w/manual security scans

### Development Tasks: Minimally-Featured GA Release

- [ ] Support for Prometheus metrics endpoint for latency metrics is added and tested
- [ ] Pet Store demo app can get config via input file
- [ ] Happy path e2e test with SP init & annotation-based config: K8s secrets
- [ ] Happy path e2e test with SP job  & annotation-based config
- [ ] Happy path e2e test with SP init & annotation-based config: file
- [ ] Dap-wiki docs have updated info on new flows - GA release
- [ ] Basic troubleshooting
- [ ] Custom upgrade instructions are documented
- [ ] Instructions for updating annotations (e.g. restarting Pod) are documented
- [ ] Documents have clear instructions for volume mapping and file permissions
- [ ] Performance of SP with the file flow has been measured
- [ ] Final security review has been performed

### Development Tasks: Full-Featured (Certified) Community Release

- [ ] Add support for secrets without aliases
- [ ] Add support for supplying secrets path prefix
- [ ] Add support for specifying JSON file format and output to file
- [ ] Add support for specifying (non-default) output file path/name
- [ ] Add support for Bash export output
- [ ] Add support for dotenv export output
- [ ] Add support for templated file output
- [ ] Provide named Helm template for SP init container def
- [ ] Provide named Helm template for SP volumes
- [ ] Blog post
- [ ] Recorded demo
- [ ] Training session

## Definition of Done

The Definition of Done criteria for this feature are organized into tasks
corresponding to three phases of development:

- [ ] Minimally-featured community release
- [ ] Minimally-featured GA release
- [ ] Full-featured GA release

### Definition of Done: Minimally-Featured Community Release

- [ ] All items in the
  [Development Tasks: Minimally-Featured Community Release](#development-tasks-minimally-featured-community-release)
  section will be implemented with unit tests.
- [ ] SP upgrade process has been validated for existing users
- [ ] Dap-Wiki has been updated with info on the new flows (initial community release)
- [ ] Any custom upgrade instructions are documented
- [ ] There is a quick start environment for running SP in "file" mode locally
- [ ] The UX of the feature has been reviewed
- [ ] SP has been released with the new "file" functionality, including manual security scans

### Definition of Done: Minimally-Featured GA Release

- [ ] All items in the
  [Development Tasks: Minimally-Featured GA Release](#development-tasks-minimally-featured-ga-release)
  section for the will be implemented with unit tests.
- [ ] Pet store demo app supports getting its database configuration via input file.
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  using the K8s Secrets init container with annotation-based configuration
- [ ] There is a happy path e2e test in the Secrets Provider test cases to
  validate using the K8s Secrets Job with annotation-based configuration
- [ ] There are e2e tests in supported OpenShift versions and GKE with Secrets Provider init container in "file" mode running with:
    - Conjur Open Source
    - Conjur Enterprise with follower in Kubernetes
    - Conjur Enterprise with follower outside Kubernetes
- [ ] Init container and Job flows have both been tested
- [ ] There is documentation collateral for the TWs
- [ ] Dap-Wiki has been updated with info on the new flows
- [ ] There is basic troubleshooting information available
- [ ] Any custom upgrade instructions are documented
- [ ] Instructions for restarting pod on annotations changes have been documented
- [ ] Documentation is clear about requirements for volume mapping and file permissions
- [ ] Performance of the SP with the "file" flow supported has been measured
- [ ] SP has been released with the new "file" functionality, including manual security scans
- [ ] All high and critical automated scanner issues and known high and critical bugs have been addressed

### Full-Featured (Certified) Community Release

- [ ] All items in the
  [Deployment Tasks: Full-Featured (Certified) Community Release](#deployment-tasks-full-featured-certified-community-release)
  section will be implemented with unit tests.
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  using annotation-based configuration without secrets aliases
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  using secrets path prefix
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  writing of JSON formatted file
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  using an output file path/name
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  writing of Bash formatted file
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  writing of dotenv formatted file
- [ ] There is a happy path e2e test to validate Helm named templates for init
  container def, volume mounts, and volumes
- [ ] There is a blog post describing how to transition from an existing application to using the new Secrets Provider "push to file" flow
- [ ] There is a recorded demo of the new SP "file" functionality
- [ ] The new SP "file" functionality has been shared at a training session

### Future Work

- [ ] Add support for plaintext export output
- [ ] Add ability to configure secret file permissions via conjur.org/secret-file-perms.{secret-group} annotation

## Solution Review

<table>
<thead>
<tr class="header">
<th><strong>Persona</strong></th>
<th><strong>Name</strong></th>
<th><strong>Design Approval</strong></th>
</tr>
</thead>
<tbody>
<tr class="odd">
<td>Team Leader</td>
<td>Dane LeBlanc</td>
<td><ul>
<p> ✅</p>
</ul></td>
</tr>
<tr class="even">
<td>Product Owner</td>
<td>Alex Kalish</td>
<td><ul>
<p> ✅</p>
</ul></td>
</tr>
<tr class="odd">
<td>System Architect</td>
<td>Rafi Schwarz</td>
<td><ul>
<p> ✅</p>
</ul></td>
</tr>
<tr class="even">
<td>Security Architect</td>
<td>Andy Tinkham</td>
<td><ul>
<p> ✅</p>
</ul></td>
</tr>
<tr class="odd">
<td>QA Architect</td>
<td>Andy Tinkham</td>
<td><ul>
<p> ✅</p>
</ul></td>
</tr>
</tbody>
</table>

## Appendix

### Example Kubernetes Manifests and Helm Named Templates

#### Example Deployment Manifest

Below is an example of an application Deployment manifest that uses:

- Conjur secrets annotations.
- References a partial Helm named template for a Secrets Provider init
  container with volume mounts.
- References a partial Helm named template for Secrets Provider volumes.

```
---
##########################################
### K8s Client App Deployment Manifest ###
##########################################
# This is an example deployment manifest intended to demonstrate the UX.  It
# defines a fictitious app representing the customer workload that manages
# inventory through an API.
#
# Comments have been added to sections related to this integration.  They often
# include the following structured tags:
#   - boilerplate: yes indicates this is literal copy/paste into manifest, or,
#     alternatively, included via Helm.
#   - sidecar-injected: yes indicates this section will be added by sidecar
#     injector in a later milestone.

apiVersion: apps/v1beta1
kind: Deployment
metadata:
  labels:
    app: inventory-api
  name: inventory-api

spec:
  replicas: 1
  selector:
    matchLabels:
      app: inventory-api
  template:
    metadata:
      labels:
        app: inventory-api
      annotations:

        ##########################
        ### Conjur Annotations ###
        ##########################
        # Annotations are now used to both provide some basic configuration
        # details (i.e. host ID) and configure exactly which secrets to pull
        # from Conjur and to which files they should be written. To start, they
        # need to be defined on the Pod.  Later, with the admission controller,
        # they can be moved to the controller resource (e.g. the deployment
        # here).
        # - boilderplate: no
        # - sidecar-injected: no

        # Host ID for authenticating with Conjur. This is traditionally named
        # CONJUR_AUTHN_LOGIN in most clients, but this new name is more
        # obvious.
        conjur.org/authn-identity: host/conjur/authn-k8s/cluster/apps/inventory-api

        # Core config for SP, instructing it how to operate.
        # Could have defaults?
        conjur.org/container-mode: init
        conjur.org/secret-destination: file

        # This maps to the existing DEBUG environment variable.
        conjur.org/debug-logging: true

        # Define variables for the unique group "database". For reference,
        # annotations for secrets are in the form,
        # "conjur.org/<parameter-name>.<group>".  This is the most basic way to
        # configure the secrets file.  The file name is assumed using the group
        # name and the default type (i.e. YAML) is used.  Thus, this would
        # result in a file at /opt/secrets/conjur/database.yml with contents:
        #     url: <value>
        #     port: <value>
        #     password: <value>
        #     username: <value>
        # Notice that the variable name is used without the path.
        conjur.org/conjur-secrets.database: |
          - /policy/path/to/url
          - /policy/path/to/port
          - /policy/path/to/password
          - /policy/path/to/username
        # Define variables for the unique group "cache".  This example
        # demonstrates how variables can be aliased, if aplications secrets to
        # be named differently than the vault.  Here, "admin-password" is the
        # secret name written to the file and "password" is the Conjur
        # variable.
        conjur.org/conjur-secrets.cache: |
          - url
          - admin-password: password
          - admin-username: username
        # For Conjur variables with very long policy branches, a path prefix
        # can be provided that will apply to all variables inside the group.
        # For example, "url" above will actually be found at
        # "/very/long/conjur/policy/path/url".
        conjur.org/conjur-secrets-policy-path.cache: /very/long/conjur/policy/path

        # The file path and name can be customized from the default.
        conjur.org/secret-filepath.cache: /files/cache-config.json

        # Additionally, instead of outputting the default YAML format, "json"
        # or "bash" can be specified as named templates.  JSON would look
        # like:
        #   {
        #     url: <value>
        #     admin-password: <value>
        #     admin-username: <value>
        #   }
        # And bash (aka environment variables) would look like:
        #   export url="<value>"
        #   export admin-password="<value>"
        #   export admin-username="<value>
        conjur.org/secret-template.cache: json

        # Finally, users can provide completely customized secret file
        # templates using the Golang templating language.  Variables can be
        # referenced via their name (or alias if provided) and are replaced
        # with their value from Conjur.
        conjur.org/secret-template-custom.cache: |
          {
            "cache": {
              "url": {{ .url }},
              "password": {{ .admin-password }},
              "username": {{ .admin-username }},
              "port": 123456
            }
          }
    spec:
      serviceAccountName: inventory-api-service-account
      containers:
      - image: my_company/inventory-api
        imagePullPolicy: Always
        name: inventory-api
        ports:
        - containerPort: 8080
        volumeMounts:

          ############################
          ### Secrets Volume Mount ###
          ############################
          # Mount the volume containing secrets to the app container. Developer
          # can customize the "mountPath" within the volume.
          #   - boilerplate: no
          #   - sidecar-injected: yes

          - mountPath: /opt/secrets/conjur
            name: conjur-secrets
            readOnly: true

    initContainers:

      #######################################
      ### Secrets Provider Init Container ###
      #######################################
      # Defines the SP init container.  The example below uses a Helm named
      # template via "includes" to avoid copy/paste.
      #   - boilerplate: yes
      #   - sidecar-injected: yes

      {{ include "conjur-secrets-provider.init-container" . | indent 6 }}

      volumes:

        ######################
        ### Secrets Volumes ##
        ######################
        # Define the volume to which secret files will be written.  Also,
        # define a downward API volume from which SP will read pod info and
        # annotations that contain secret nd configuration information. The
        # example below uses a Helm named template via "includes" to avoid
        # copy/paste.
        #   - boilerplate: yes
        #   - sidecar-injected: yes

        {{ include "conjur-secrets-provider.volumes" . | indent 8 }}
```

#### Example Helm Partial Named Template for Secrets Provider Init Container

Below is an example of a Helm partial named template for a Secrets Provider
init container with volume mounts (as referenced in the Deployment manifest
above):

```
{{/* Deployment manifest init containers for Secrets Provider */}}
{{- define "conjur-secrets-provider.init-container" -}}
- image: {{ .Values.conjur.secretsProvider.image }}
  imagePullPolicy: Always
  name: conjur-secrets-provider

  volumeMounts:
    - mountPath: /conjur/secrets
      name: conjur-secrets
    - mountPath: /conjur/connect
      name: conjur-connect
    - mountPath: /conjur/podinfo
      name: conjur-podinfo
{{- end -}}
```

#### Example Helm Partial Named Template for Secrets Provider Volumes

Below is an example of a Helm partial named template for Volumes used by
a Secrets Provider init container (as referenced in the Deployment manifest
above):

```
{{/* Deployment manifest volumes for Secrets Provider */}}
{{- define "conjur-secrets-provider.volumes" -}}
- name: conjur-secrets
  emptyDir:
    medium: Memory

- name: conjur-connect
  configMap:
    name: {{ .Values.conjur.configMaps.connect | default "conjur-connect" }}

- name: conjur-podinfo
  downwardAPI:
    items:
      - path: annotations
        fieldRef:
          fieldPath: metadata.annotations
      - path: MY_POD_NAME
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
      - path: MY_POD_NAMESPACE
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.namespace
      - path: MY_POD_IP
        fieldRef:
          fieldPath: status.podIP
{{- end -}}
```

#### Example Helm Chart values.yaml File

Below is an example Helm chart `values.yaml` file corresponding to the
manifest / Helm named tempates above:

```
conjur:
  volumes:
    secrets:
      name: "conjur-secrets"
      mountPath: "/conjur/secrets"
    connect:
      name: "conjur-connect"
      mountPath: "/conjur/connect"
    podinfo:
      name: "conjur-podinfo"
      mountPath: /conjur/podinfo"
  configMaps:
    connect: "conjur-connect-configmap"
  secretsProvider:
    name: "cyberark-secret-provider-for-k8s"
    image: "cyberark/cyberark-secret-provider-for-k8s:2.0.0"
```

#### Example Conjur Policy for Above Manifests and Named Templates

Below is an example Conjur policy corresponding to the example manifest and
Helm chart named templates above:

```
---
### Conjur Policy ###
# This is an example set of Conjur policy intended to demonstrate the UX.
# It defines a policy with sample inventory API client workload host and
# database credentials.

- !policy
  id: k8s-cluster-apps
  annotations:
    description: Apps and services in company cluster.
  body:

  - !layer k8s-apps

  - &apps
    - !host
      id: inventory-api
      annotations:
        authn-k8s/namespace: my_namespace
        authn-k8s/authentication-container-name: cyberark-secrets-provider-for-k8s

  - !grant
    role: !layer k8s-apps
    members: *apps

  - &database-variables
    - !variable url
    - !variable port
    - !variable password
    - !variable username

  - !permit
    role: !layer k8s-apps
    privileges: [ read, execute ]
    resources: *database-variables
```
