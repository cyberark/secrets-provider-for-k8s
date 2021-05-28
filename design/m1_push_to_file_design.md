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
<td><p><a href="https://cyberark365.sharepoint.com/:w:/r/sites/Conjur/_layouts/15/Doc.aspx?sourcedoc=%7B956DB935-7E3D-4D88-B964-188BCB6F7729%7D&file=Conjur%20Developer%20Experience%20for%20K8s%20PRD.docx&action=default&mobileredirect=true">link</a> (private)</p></td>
</tr>
<tr class="even">
<td>Aha Card</td>
<td><p><a href="https://cyberark.aha.io/epics/SCR-E-76">link</a> (private)</p>
<p><em>Note: This design document covers work for “Milestone 1: Push to File” as defined in this Aha Card.</em></p></td>
</tr>
<tr class="odd">
<td>Feature Doc</td>
<td><a href="https://cyberark365.sharepoint.com/:w:/r/sites/Conjur/_layouts/15/Doc.aspx?sourcedoc=%7BB782E509-693F-4086-85A6-5D477A0F4ABD%7D&file=Feature%20Doc%20-%20Kubernetes%20Developer%20Experience%20M1.docx&action=default&mobileredirect=true&cid=048c1606-6533-443c-ba09-f91590c68095">link</a> (private)</td>
</tr>
<tr class="even">
<td>Sample Manifests and Policies From Feature Spec</td>
<td><a href="https://gist.github.com/alexkalish/8a514defa9e741800c095dded9837582">link</a> (private)</td>
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

The Secrets Provider container/application configuration can be set using
either Pod annotations or by using Pod environment variable settings
([legacy configuration](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/11.2/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm#Step4AddtheCyberArkSecretsProviderforKubernetesimagetotheapplicationdeploymentmanifest)).
If both a Pod Annotation and a Pod environment variable is configured for
any particular Secrets Provider setting, then the Pod Annotation configuration
takes precedence.

_**QUESTION: Should we deprecate the current configuration method that uses
   environment variable settings?**_

Note that Pod Annotation configuration will not be supported for the
`MY_POD_NAME` and `MY_POD_NAMESPACE` settings. These settings will continue
to use the standard
[Kubernetes Downward API configuration](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/).

The following annotations are supported for Secrets Provider configuration.
Please refer to the
[Secrets Provider documentation](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/11.2/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm#Step4AddtheCyberArkSecretsProviderforKubernetesimagetotheapplicationdeploymentmanifest)
for a description of each setting:

| Annotation | Equivalent<br /> Environment Variable | Description, Notes |
|--------------------------------------|---------------------|----------------------------------|
| conjur.org/<br />authn-identity      | CONJUR_AUTHN_LOGIN  | Required value. Example: `host/conjur/authn-k8s/cluster/apps/inventory-api` |
| conjur.org/<br />container-mode      | CONTAINER_MODE      | Allowed values: <ul><li>`init`</li><li>`application`</li><li>`sidecar`</li></ul>Defaults to `sidecar` |
| conjur.org/<br />secrets-destination | SECRETS_DESTINATION | Required value. Allowed values: <ul><li>`file`</li><li>`k8s_secret`</li></ul> |
| conjur.org/<br />k8s-secrets         | K8S_SECRETS         | This list is ignored when `conjur.org/secrets-destination` Annotation is set to `file` |
| conjur.org/<br />retry-count-limit   | RETRY_COUNT_LIMIT   | Defaults to 5
| conjur.org/<br />retry-interval-sec  | RETRY_INTERVAL_SEC  | Defaults to 1 (sec)              |
| conjur.org/<br />debug-logging       | DEBUG               | Defaults to `false`              |
| conjur.org/<br />conjur-configmap    | CONJUR_AUTHN_URL<br />CONJUR_URL<br />CONJUR_ACCOUNT<br />CONJUR_SSL_CERTIFICATE | ConfigMap containing Conjur connection information. |

#### Annotations for Push-to-File Secrets Mappings

##### Secrets Groups

The configuration of the Secrets Provider push-to-file secrets mappings
makes use of "secrets groups". A secrets group is a logical grouping of
application secrets, typically belonging to a particular component of an
application deployment (e.g. all secrets related to a backend database).

As shown in the table below, all push-to-file annotations contain a reference
to a secret group, using the form: `conjur.org/<parameter-name>.<group>`.

There is a one-to-one correspondence between secrets groups and files that
are written to the application Pod's Conjur secrets shared volume.

##### Supported Annotations

| Annotation                                     | Description |
|------------------------------------------------|-------------|
| conjur.org/conjur-secrets.{secret-group}       | List of secrets to be retrieved from Conjur. Each entry can be either:<ul><li>A Conjur variable path</li><li> A key/value pairs of the form `<alias>:<Conjur variable path>` where the `alias` represents the name of the secret to be written to the secrets file |
| conjur.org/conjur-secrets-path.{secret-group}  | Prefix to use for all Conjur variable paths for a secrets group. Defaults to root policy path. |
| conjur.org/secret-file-path.{secret-group}     | File path (including file name) for secrets file to be written. Default is `{secret-group}.{secret-file-type}`, e.g. `database.yaml`. |
| conjur.org/secret-file-type.{secret-group}     | Allowed values:<ul><li>yaml (default)</li><li>json</li><li>dotenv</li><li>bash</li></ul>This setting is ignored when `conjur.org/secret-file-template.{secret-group}` is configured. |
| conjur.org/secret-file-perms.{secret-group}    | File permissions for secrets file to be written, in octal format. Defaults to `660` |
| conjur.org/secret-file-template.{secret-group} | Customized secret file template using the Golang templating language. |

###### Example `conjur-secrets` List Without Aliases

The following is an example of a `conjur.org/conjur-secrets.{secret-group}`
Annotation that defines a flat (i.e. no aliases) list of Conjur variable paths
for a hypothetical secrets group named `database`:

```
    conjur.org/conjur-secrets.database: |
      - prod/backend/url
      - prod/backend/port 
      - prod/backend/password
      - prod/backend/username
```

For `conjur-secrets` entries without aliases, the last word in the path is
used as the application secret name. So for the example above, if
`conjur.org/secret-file-type.database` is set to `yaml`, then the resulting
secrets file that is written to the shared secrets volume might look similar
to the following:

```
url: https://database.example.com
port: 5
password: my-secret-p@$$w0rd
username: postgres
```

###### Example `conjur-secrets` List With Aliases and `conjur-secrets-path` Prefix

The following is an example of a `conjur.org/conjur-secrets.{secret-group}`
Annotation that defines a list of Conjur variable paths, including some that
use an alias, for a hypothetical secrets group named `cache`:

```
    conjur.org/conjur-secrets.cache: |
      - url
      - admin-password: password
      - admin-username: username

    conjur.org/conjur-secrets-path.cache: dev/memcached/

```

In this example, the paths for the secrets that are retrieved from Conjur are:

- dev/memcached/url
- dev/memcached/password
- dev/memcached/username

and the names of the secrets that are written to the secrets file are:

- url
- admin-password
- admin-username

###### Example `conjur.org/secret-file-template.{secret-group}`

The following is an example of a custom secrets file template that uses the
Golang templating language. Variables can be referenced via their name
(their alias if provided, otherwise, the last word in their Conjur variable
path) and are replaced with their value from Conjur:

```
    conjur.org/secret-template-custom.cache: |
      {
        "cache": {
          "url": {{ .url }},
          "password": {{ .admin-password }},
          "username": {{ .admin-username }},
          "port": 123456
        }
      }
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
      secrets group for push-to-file.
    - `conjur.org/conjur-secrets.{group}`: Inspect the manifests for the
      existing Kubernetes Secret(s). The manifests should contain a
      `stringData` section that contains secrets key/value pairs.
      Map the `stringData` entries to a YAML list value for conjur-secrets,
      using the secret names as aliases.
    - `conjur.org/secret-file-path.{group}`: Configure a target location
    - `conjur.org/secret-file-type.{group}`: Configure a desired type,
      depending on how the application will consume the secrets file.
    - `conjur.org/secret-file-perms.{group}`: Determine desired file
      permissions (see [Security](#security) section).
  - Add Pod `securityContext` (see [Security](#security) section).
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

-   Authentication containers to be tested:

    -   Secrets Provider init container
    -   Secrets Provider standalone Pod

-   Platforms:

    -   Kubernetes (this will be either Kubernetes-in-Docker, or GKE).
    -   OpenShift 4.6

#### Out of Scope

- For this release, configuration will only be read upon startup.
- For this release, annotations are required to have a group.

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
// SecretsPaths comprises Conjur variable paths for all secrets in a secrets
// group, indexed by secret name.
type SecretsPaths map[string]string

// GroupSecretsInfo comprises secrets mapping information for a given secrets
// group.
type GroupSecretsInfo struct {
    Secrets SecretsPaths
    SecretsPathPrefix string
    FilePath string
    FileType string
    FilePerms int
    Template string
}

// GroupSecrets comprises secrets mapping info for all secrets groups
var GroupSecrets map[string]GroupSecretsInfo{}
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

     - Annotation type (e.g. `conjur-secrets`, `conjur-secrets-path`, etc)
     - Secrets group
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
     per-Group `SecretsGroupMapping` structure (see definition in Appendix).


1. After all annotations have been processed, Secrets Provider iterates
   through the `SecretsGroupMapping` data structures for all secrets groups.

   For each secrets group:
   
   - If a secrets file Golang template has not been provided explicitly,
     Secrets Provider will use a "canned" Golang template based on the
     secrets file type (YAML, JSON, dotenv, or bash).
   - Secrets Provider connects/authenticates with Conjur and retrieves all
     secrets for that secrets group.
   - Secrets Provider then creates and writes a secrets file at the configured
     destination file path.

## Performance

From a high level, the performance of the Secrets Provider running in
push-to-file mode will be a measure of how fast the Secrets Provider can
perform the following as an init container:

- Parse Pod annotations
- Retrieve secrets for all secrets groups from Conjur
- Write those secrets to a volume that is shared with the application container

To maximize the performance of Conjur secrets retrieval, the Secrets Provider
will retrieve secrets from Conjur using batch retrievals on a
per-secrets-group basis. In addition, retrieval of secrets from Conjur will
be done using parallel Goroutines (with the maximum number of Goroutines
allowed TBD). It is therefore expected that the throughput in terms of
the rate of secrets that the Secrets Provider can retrieve from Conjur
should not differ significantly for Push-to-File mode versus Kubernetes
Secrets mode.

The performance of the Secrets Provider in Push-to-File mode is described in
the [Performance testing](#performance-testing) section below. The performance
measurements will include the following metrics:

- Latency (processing time) for parsing Pod annotations
- Latency for generating secrets requests and processing responses
- Latency for writing retrieved secrets to files

The measurements will be made using a representative configuration (details
TBD) in terms of the number of secrets groups and the number of secrets per
secrets group.

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

### Test assumptions

### Out of scope

### Test prerequisites

### Test cases 

#### Error Handling / Recovery / Supportability tests

| | Section | Given | When | Then | Error Code | Test Type (Unit, Integration, E2E) |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Missing secrets destination | You are using SP | You do not provide the secrets destination in an annotation or environment variable | Error: Missing secrets destination. Acceptable values are “k8s” and “file”.<br><br>SP fails to start | | |
| 2 | Invalid input file type | You are using SP in “file” mode and have provided a value for the conjur.org/ secret-file-type annotation | You have provided an unknown or invalid secret-file-type annotation, which is expected to contain one of the supported types: json, yaml, dotenv. | Error: Invalid output file type: “{value}” for secret group “{value}”. Acceptable values are “json”, “yaml”, and “dotenv”.<br><br>SP fails to start | | |
| 3 | Conjur variable names collision | You are using SP in “file” mode | Two or more secrets in the same Secret Group share the same name or alias. | Error: Multiple variables in the “{value}” secret group are called “{value}”. Provide a unique alias for each variable in the “{value}” secret group.<br><br>SP fails to start | | |
| 4 | Unparseable Conjur secrets list | You are using SP in “file” mode | The “conjur-secrets” annotation is not parseable for one or more secret groups. | Error: The list of secrets for the “{value}” secret group is not formatted correctly. Error: “{value}”. Verify that the annotation value is a YAML list with optional keys for each list item.<br><br>SP fails to start | | |
| 5 | Invalid Conjur secrets path | You are using SP in “file” mode and have provided a value for the conjur.org/conjur-secrets-path annotation | The “conjur-secrets-path" annotation is not parseable as a full secret path prefix. | Error: The Conjur secrets path “{value}” provided for the “{value}” secret group is invalid. | | |
| 6 | Invalid file template definition – not parseable by Go | You are using SP in “file” mode and have provided a value for the conjur.org/secret-file-template annotation | File template for one or more secret groups cannot be used as written. | Error: The file template for the “{value}” secret group cannot be used as written. Error: “{value}”. Update your template definition to address this and try again.<br><br>SP fails to start | | |
| 7 | Invalid file template definition – references undefined secret keys | You are using SP in “file” mode and have provided a value for the conjur.org/secret-file-template annotation | File template for one or more secret groups references secrets that do not exist in the secret group. | Error: The file template for the “{value}” secret group references the “{value}” secret, but no such secret is defined in the secret group. Add an alias to set the correct secret name to “{value}”, and try again.<br><br>SP fails | | |
| 8 | Unable to retrieve Conjur variables | You are using SP in “file” mode | | Error: Failed to provide DAP/Conjur secrets<br><br>SP fails | CSPFK016E | |
| 9 | Missing volume mounts | You are using SP in “file” mode | One or more volumes cannot be found by Secrets Provider. | Error: Unable to access volume “{value}”. Error: “{value}”. Ensure that the corrrect volumes are defined in the deployment manifest and attached to the Secrets Provider container.<br><br>SP fails to start | | |
| 10 | File permissions error | You are using SP in “file” mode | Secrets Provider does not have permission to write a secrets file to the specified volume and mount path. | Error: Secrets Provider does not have permission to write the secrets file “{value}”. Please check the volume mount permissions on the Secrets Provider container and try again.<br><br>SP fails | | |
| 11 | Invalid file path provided | You are using SP in “file” mode and have provided a value for the conjur.org/secret-file-path annotation | The provided file path is not a valid file path. | Error: You have provided an invalid file path “{value}” for the “{value}” secret group.<br><br>SP fails to start | | |
| 12 | File conflict between two secret groups | You are using SP in "file" mode and have defined multiple secret groups | One or more secret groups have he same ouput file. | Error: You have provided conflicting file paths; secret groups "{value}", "{value}", ... all have output file "{value}".<br><br>SP fails to start | | |

#### Integration / E2E tests

#### Security testing

Security testing will include:

- Automated Vulnerability scans

####  Performance testing

As described in the [Performance](#performance) section above, the performance
of the Secrets Provider for Push-to-File mode will be manually measured using
the following metrics:

- Latency (processing time) for parsing Pod annotations
- Latency for generating secrets requests and processing responses
- Latency for writing retrieved secrets to files

_**QUESTION: Do we need to measure CPU and memory footprints?**_

The measurements will be made using a representative configuration in terms
of the number of secrets groups and the number of secrets per secrets group.
Details are TBD, but a representative configuration might look something like
this:

- 5 secrets groups
- 5 secrets / secrets group
- Mix of secret value lengths (10 to 20 characters, plus some base64-encoded
  TLS certificates)

## Logs

| **Scenario** | **Log Level** | **Notes** |
| ------------ | ------------- | --------- |
| Annotation has `conjur.org` prefix, but invalid key or value format | `Error` | Note 1 |
| Annotation has `conjur.org` prefix, but unknown annotation type | `Info` | Note 2 |
| Start of secrets retrieval for a group | `Info` | Includes group name |
| Start of secrets file creation for a group | `Info` | Includes group name |
| Error processing Go template | `Error` | Note 3 |
| Error writing secrets file | `Error` | Note 3 |

- *Note 1:* Annotation values are treated as non-sensitive and will be included the logs.
- *Note 2:* For forward compatability, annotation keys with the prefix
  `conjur.org` but an unexpected annotation type will be logged at `Info` level.
- *Note 3:* Care must be taken to prevent secrets file content from leaking.

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

### Secrets File Attributes

Since the Push-to-File feature involves writing sensitive information to
files that are shared by an application container, care needs to be taken
so that files are created with the proper file attributes:

- File permissions
- File owner/UID
- File group/GID

By default, the Secrets Provider will create secrets files with the following
file attributes, based on the default user defined in the Secrets Provider
container image:

- File permissions: 660         (`rw-rw----`)
- File owner: secrets-provider  (UID 777)
- File group: secrets-provider  (GID 777)

If the application container is built to run with a non-root user that has
different a UID and GID than the default values shown above, then the
application will not be able to access secrets files that have been
created using the default attributes.

In these cases, users will want to include a 
[Pod Security Context](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#podsecuritycontext-v1-core)
in their Pod Deployment to change either the user that is running inside
the Secrets Provider container, or the GID with which secrets files will be
created. Pod Security Context configuration can be added to a Pod Deployment
as decribed [here](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/).

For example, to configure the user and group with which the Secrets Provider
container runs, and enforce that the Pod containers must run as a non-root
user, the following can be included in the Pod spec:

```
    securityContext:
      runAsUser: 1000
      runAsGroup: 3000
      runAsNonRoot: true
```

In this example, the access to secrets files can be restricted further by
modifying the file permissions that are used when secrets files are created,
for example, using the following annotation:

```
    conjur.org/secret-file-perms.database=600
```

Another option is to configure the GID with which all files (including secrets
files) are created in `emptyDir` volumes:

```
    securityContext:
      fsGroup: 2000
```

Note that defining an `fsGroup` will cause the file permissions bits to be
OR'd with `rw-rw----`.

(For OpenShift deployments, the `fsGroup` may need to be configured as `0`.)

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
    - RetryCountLimit - conjur.org/retry-count-limit
    - RetryIntervalSec - conjur.org/retry-interval-sec
    - StoreType - conjur.org/secrets-destination
  - [ ] Kubernetes Secrets config:
    - RequiredK8sSecrets – conjur.org/k8s-secrets
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
    - Secrets group name
    - Push-to-File Annotation type
    - Annotation value (can be a YAML formatted string)
  - [ ] Add logic to parse annotation values:
    - Parse `conjur.org/conjur-secrets.{secret-group}` (with aliases)
- [ ] Given populated data structure, get secrets and write to YAML file
- [ ] SP upgrade process has been validated - init container
- [ ] SP upgrade process has been validated - job
- [ ] Dap-wiki docs have updated info on new flows - initial community release
- [ ] Custom upgrade instructions are documented
- [ ] Quick start for SP in file mode
- [ ] UX has been reviewed and issues have been corrected
- [ ] SP is release w/manual security scans

### Development Tasks: Minimally-Featured GA Release

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

### Development Tasks: Full-Featured (Certified) Community Release

- [ ] Add support for secrets without aliases
- [ ] Add support for supplying secrets path prefix
- [ ] Add support for specifying JSON file type and output to file
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
- [ ] There is a quick start environment for running SP in “file” mode locally
- [ ] The UX of the feature has been reviewed
- [ ] SP has been released with the new “file” functionality, including manual security scans

### Definition of Done: Minimally-Featured GA Release

- [ ] All items in the
  [Development Tasks: Minimally-Featured GA Release](#development-tasks-minimally-featured-ga-release)
  section for the will be implemented with unit tests.
- [ ] Pet store demo app supports getting its database configuration via input file.
- [ ] There is a happy path e2e test in the Secrets Provider test cases to validate
  using the K8s Secrets init container with annotation-based configuration
- [ ] There is a happy path e2e test in the Secrets Provider test cases to
  validate using the K8s Secrets Job with annotation-based configuration
- [ ] There are e2e tests in supported OpenShift versions and GKE with Secrets Provider init container in “file” mode running with:
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
- [ ] (TBD) Performance of the SP with the “file” flow supported has been measured, and has not appreciably decreased from prior measurements (prior perf tests)
- [ ] SP has been released with the new “file” functionality, including manual security scans

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
- [ ] There is a blog post describing how to transition from an existing application to using the new Secrets Provider “push to file” flow
- [ ] There is a recorded demo of the new SP “file” functionality
- [ ] The new SP “file” functionality has been shared at a training session

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
<td>            </td>
<td><ul>
<li><blockquote>
<p> </p>
</blockquote></li>
</ul></td>
</tr>
<tr class="even">
<td>Product Owner</td>
<td>Alex Kalish</td>
<td><ul>
<li><blockquote>
<p> </p>
</blockquote></li>
</ul></td>
</tr>
<tr class="odd">
<td>System Architect</td>
<td>Rafi Schwarz</td>
<td><ul>
<li><blockquote>
<p> </p>
</blockquote></li>
</ul></td>
</tr>
<tr class="even">
<td>Security Architect</td>
<td>Andy Tinkham</td>
<td><ul>
<li><blockquote>
<p> </p>
</blockquote></li>
</ul></td>
</tr>
<tr class="odd">
<td>QA Architect</td>
<td>Andy Tinkham</td>
<td><ul>
<li><blockquote>
<p> </p>
</blockquote></li>
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
        conjur.org/conjur-secrets-path.cache: /very/long/conjur/policy/path

        # The file path and name can be customized from the default.
        conjur.org/secret-filepath.cache: /files/cache-config.json

        # The file permissions for the secret file can be configured.
        # The default value is octal '660' ('rw-rw----').
        conjur.org/secret-file-perms.cache=600

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
      securityContext:
        runAsUser: 1000
        runAsGroup: 3000
        runAsNonRoot: true
        fsGroup: 3000
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
  name: {{ .Values.conjur.secretsProvider.name }}

  volumeMounts:
    - mountPath: {{ .Values.conjur.volumes.secrets.mountPath }}
      name: {{ .Values.conjur.volumes.secrets.name }}
    - mountPath: {{ .Values.conjur.volumes.connect.mountPath }}
      name: {{ .Values.conjur.volumes.connect.name }}
    - mountPath: {{ .Values.conjur.volumes.podinfo.mountPath }}
      name: {{ .Values.conjur.volumes.podinfo.name }}
{{- end -}}
```

#### Example Helm Partial Named Template for Secrets Provider Volumes

Below is an example of a Helm partial named template for Volumes used by
a Secrets Provider init container (as referenced in the Deployment manifest
above):

```
{{/* Deployment manifest volumes for Secrets Provider */}}
{{- define "conjur-secrets-provider.volumes" -}}
- name: {{ .Values.conjur.volumes.secrets.name }}
  emptyDir:
    medium: Memory

- name: {{ .Values.conjur.volumes.connect.name }}
  configMap:
    name: {{ .Values.conjur.configMaps.connect }}
  
- name: {{ .Values.conjur.volumes.podinfo.name }}
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

