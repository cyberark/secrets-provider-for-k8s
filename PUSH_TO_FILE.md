# Secrets Provider - Push to File Guide


# Table of Contents

- [Table of Contents](#table-of-contents)
- [Overview](#overview)
- [Certification Level](#certification-level)
- [Prerequisites](#prerequisitesassumptions)
- [Annotations](#reference-table-of-configuration-annotations)
- [Volume Mounts](#volume-mounts)
- [Example Manifest](#example-manifest-for-push-to-file-with-yaml-output)
- [Upgrading Existing Secrets Provider Deployments](#upgrading-existing-secrets-provider-deployments)
  - [Upgrading with the helper script](#using-the-helper-script-to-patch-the-deployment)


## Overview

The push to file feature detailed below allows Kubernetes applications to consume Conjur 
secrets through one or more files accessed through a shared, mounted volume. 
Secrets Provider is configured to run as an init container for an application. 
When the pod is launched, this init container reads configuration from Kubernetes 
annotations, fetches the desired secrets from Conjur and writes them to files in a 
volume shared with the application container.  Providing secrets in this way should 
require zero application changes, as reading local files is a common, 
platform agnostic delivery method.

As mentioned above, Secrets Provider can write multiple files containing Conjur secrets. 
Each file is configured independently as a named secrets group using 
[Kubernetes Pod Annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/). 
Using annotations is new to Secrets Provider with this feature and provides a more 
idiomatic experience.

## Certification Level
![](https://img.shields.io/badge/Certification%20Level-Community-28A745?link=https://github.com/cyberark/community/blob/master/Conjur/conventions/certification-levels.md)

The Secrets Provider push to File feature is a **Community** level project. It's a community contributed project that **is not reviewed or supported
by CyberArk**. For more detailed information on our certification levels, see [our community guidelines](https://github.com/cyberark/community/blob/master/Conjur/conventions/certification-levels.md#community).

Known limitations with this release:
- The push-to-file annotation `conjur.org/secret-file-path.{secret-group}` 
needs to be specified as `/conjur/secrets/[file name]`.

For example 
```
conjur.org/secret-file-path.init-app: /conjur/secrets/init-app.yaml
```
- The file name for the secrets file cannot be a directory and must be a single file. 

See the 
[Reference table of configuration annotations](#reference-table-of-configuration-annotations) 
for more details.

These will be resolved with the next release.

## Prerequisites/Assumptions
- This guide does not cover Conjur configuration and setup. Please refer to
  [Secrets Provider for Kubernetes documentation](https://docs.conjur.org/Latest/en/Content/Integrations/k8s-ocp/cjr-secrets-provider-lp.htm) for more information.
- Push to File feature requires the Secrets Provider must be run as an init container.
- Configuration of the Secrets Provider must be done using Annotations rather than using
  environment variables
  This reference describes how to configure push to file using the Secrets Provider. 

## Reference table of configuration annotations.

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
| `conjur.org/container-mode`         | `CONTAINER_MODE`      | Allowed values: <ul><li>`init`</li><li>`application`</li></ul>Defaults to `init`.<br>Must be set to init for Push to File mode.|
| `conjur.org/secrets-destination`    | `SECRETS_DESTINATION` | Allowed values: <ul><li>`file`</li><li>`k8s_secret`</li></ul> |
| `conjur.org/k8s-secrets`            | `K8S_SECRETS`         | This list is ignored when `conjur.org/secrets-destination` annotation is set to **`file`** |
| `conjur.org/retry-count-limit`      | `RETRY_COUNT_LIMIT`   | Defaults to 5
| `conjur.org/retry-interval-sec`     | `RETRY_INTERVAL_SEC`  | Defaults to 1 (sec)              |
| `conjur.org/debug-logging`          | `DEBUG`               | Defaults to `false`              |
| `conjur.org/conjur-secrets.{secret-group}`      | Push to File config is not available with environmental variables | List of secrets to be retrieved from Conjur. Each entry can be either:<ul><li>A Conjur variable path</li><li> A key/value pairs of the form `<alias>:<Conjur variable path>` where the `alias` represents the name of the secret to be written to the secrets file |
| `conjur.org/secret-file-path.{secret-group}`    | Push to File config is not available with environmental variables | Path for secrets file to be written. <br> For the initial release of push-to-file the secret file path for the shared secrets must be '/conjur/secrets' . The file path must also include a file name (i.e. must not be a directory). Values ending with `/` are rejected and cause the Secrets Provider to abort.
| `conjur.org/secret-file-format.{secret-group}`  | Push to File config is not available with environmental variables | Allowed values:<ul><li>yaml (default)</li><li>json</li><li>dotenv</li><li>bash</li> |


## Volume mounts
When the Secrets Provider is configured for file mode, as described above, it will 
write secrets to file(s) in an volume that is shared with the application container. 
The volumes required for this mode are as follows:
* `conjur-secrets`: An `emptydir` volumed mounted to both the application container 
and Secrets Provider.  Secrets fetched from Conjur are written here.
* `podinfo`: A volume mounted to just Secrets Provider containing pod annotations from the Downward API.

Below is sample YAML defining the two volumes:
```
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
Below is sample volume mounts for the Secrets Provider init container:
```
volumeMounts:
  - name: podinfo
    mountPath: /conjur/podinfo
  - name: conjur-secrets
    mountPath: /conjur/secrets
```

Below is sample volume mount for the target application:
```
volumeMounts:
  - name: conjur-secrets
    mountPath: /opt/secrets/conjur
    readOnly: true
```

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


## Example Manifest for Push to File with YAML output

Below is an example of using annotations in a Kubernetes manifest:

```

apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: test-env
  name: test-env
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-env
  template:
    metadata:
      labels:
        app: test-env
      annotations:
        # Equivalent to Environment Variable CONJUR_AUTHN_LOGIN
        conjur.org/authn-identity: host/conjur/authn-k8s/cluster/apps/inventory-api
        # Equivalent to Environment Variable CONTAINER_MODE
        conjur.org/container-mode: init
        # Equivalent to Environment Variable SECRETS_DESTINATION
        conjur.org/secret-destination: file
        # Annotations for writing to file
        conjur.org/conjur-secrets.cache: |
        - dev/redis/api-url
        - admin-username: dev/redis/username
        - admin-password: dev/redis/password
        conjur.org/secret-file-path.cache: "./redis.yaml"
        conjur.org/secret-file-format.cache: "yaml"
    spec:
       serviceAccountName: test-env-sa
      containers:
      - image: debian
        name: init-env-app
        command: ["sleep"]
        args: ["infinity"]
        volumeMounts:
          - name: conjur-secrets
            mountPath: /opt/secrets/conjur
            readOnly: true
      initContainers:
      - image: 'secrets-provider-for-k8s:latest'
        imagePullPolicy: Never
        name: cyberark-secrets-provider-for-k8s
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

      imagePullSecrets:
        - name: dockerpullsecret
```

This will create a file /opt/secrets/conjur/redis.yaml, with contents as below.
```
"api-url": "value-dev/redis/api-url"
"admin-username": "value-dev/redis/username"
"admin-password": "value-dev/redis/password"
```

Below are code snippet is for JSON output.

```
conjur.org/conjur-secrets.cache: |
  - dev/redis/api-url
  - admin-username: dev/redis/username
  - admin-password: dev/redis/password
     conjur.org/secret-file-path.cache: "./testdata/redis.json"
     conjur.org/secret-file-format.cache: "json"
```

This will create a file redis.json, with contents as below.
```
{"api-url":"value-dev/redis/api-url","admin-username":"value-dev/redis/username","admin-password
":"value-dev/redis/password"}
```

Below are code snippet is for Bash output.

```
conjur.org/conjur-secrets.cache: |
  - dev/redis/api-url
  - admin-username: dev/redis/username
  - admin-password: dev/redis/password
     conjur.org/secret-file-path.cache: "./testdata/redis.sh"
     conjur.org/secret-file-format.cache: "bash"
```
This will create a file redis.sh, with contents as below.
```
     export api-url="value-dev/redis/api-url"
     export admin-username="value-dev/redis/username"
     export admin-password="value-dev/redis/password"

```

Below are code snippet is for dotenv output.

```
conjur.org/conjur-secrets.cache: |
  - dev/redis/api-url
  - admin-username: dev/redis/username
  - admin-password: dev/redis/password
     conjur.org/secret-file-path.cache: "./testdata/redis.env"
     conjur.org/secret-file-format.cache: "dotenv"
```

This will create a file redis.env, with contents as below.
```
api-url="value-dev/redis/api-url"
admin-username="value-dev/redis/username"
admin-password="value-dev/redis/password"
```

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

