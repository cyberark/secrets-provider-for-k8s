# Secrets Provider - Secrets Rotation

# Table of Contents

- [Table of Contents](#table-of-contents)
- [Overview](#overview)
- [Certification Level](#certification-level)
- [How Secrets Rotation Works](#how-secrets-rotation-works)
- [Set up Secrets Provider for secrets rotation](#set-up-secrets-provider-for-secrets-rotation)
- [Additional Configuration Annotations](#reference-table-of-configuration-annotations)
- [Troubleshooting](#troubleshooting)
- [Limitations](#limitations)

## Overview

The secrets rotation feature detailed below allows Kubernetes applications to
refresh Conjur secrets if there are any changes to the secrets.

Secrets rotation is only supported for Push to File mode, though K8s Secrets will be part of the GA release.


## Certification Level
![](https://img.shields.io/badge/Certification%20Level-Community-28A745?link=https://github.com/cyberark/community/blob/master/Conjur/conventions/certification-levels.md)

The Secrets Provider secrets rotation feature is a **Community** level project.
Community projects **are not reviewed or supported by CyberArk**. For more
detailed information on our certification levels, see
[our community guidelines](https://github.com/cyberark/community/blob/master/Conjur/conventions/certification-levels.md#community).

## How Secrets Rotation Works

![how secrets rotation works](./design/how-secrets-rotation-works.png)

Note: see [how-push-to-file-works](PUSH_TO_FILE.md#how-push-to-file-works) for more detail on
how Push to File works.


1. The Secrets Provider authenticates to the Conjur server using the
   Kubernetes Authenticator (`authn-k8s`).

2. The Secrets Provider reads all Conjur secrets required across all
   secret groups.

3. The Secrets Provider sidecar container starts up and retrieves the initial secrets.
   If enabled, after the duration specified by `conjur.org/secrets-refresh-interval` or the default interval
   the Secrets Provider will check if the secrets have changed by comparing the SHA-256 checksums
   of the secrets with the previous checksums. The Secrets Provider does not save any of the unencrypted secrets.
  If the time needed to fetch the secrets is longer than is specified
   for the duration, then the duration will be the actual time to retrieve the secrets.

   For example:
   If the duration is set to two seconds, but retrieving the secrets takes three second then the
   secrets will be updated every three seconds.

   Note:
   If one or more of the secrets have been removed from Conjur or have had access revoked, the Secrets Provider
   will remove the secrets files from the shared volume. To disable this feature, set the
   `conjur.org/remove-deleted-secrets-enabled` annotation to `false`.
4. The Secrets Provider renders secret files for each secret group, and
   writes the resulting files to a volume that is shared with your application
   container. The Secrets Provider will rewrite the secret files if there are any changes.
5. The application reads the secrets.
6. The application can optionally delete the secret files after consuming.
   If the secret files are deleted, they will only be recreated when the secret values have changed.


## Set up Secrets Provider for secrets rotation

There are two new annotations introduced and one annotation is updated.

Prerequisites:

Requires secrets-provider-for-k8s v1.4.0 or later.

<details><summary>For Push to File mode</summary>

Follow the procedure to set up Secrets Provider for [Push to File](PUSH_TO_FILE.md#set-up-secrets-provider-for-push-to-file)
</details>
<details><summary>For Kubernetes Secrets mode</summary>

Follow the procedure to set up [Kubernetes Secrets](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ic.htm?tocpath=Integrations%7COpenShift%252FKubernetes%7CSet%20up%20applications%7CSecrets%20Provider%20for%20Kubernetes%7CInit%20container%7C_____1#SetupSecretsProviderasaninitcontainer)
</details>

Modify the Kubernetes manifest
1. Change the Secrets provider container to be a sidecar. If it was configured
   as an init container remove the `initContainers`:
   ```
    spec:
      containers:
      - image: secrets-provider-for-k8s:latest
    ```
2. Update the `conjur.org/container-mode` annotation:
   ```
   conjur.org/container-mode: sidecar
   ```

3. Add the new Secrets Rotation annotations.
   There are two new annotations added, only one of the annotations is
   required to be set to enable secrets rotation.

   `conjur.org/secrets-refresh-enabled` enables the
   feature if the container mode is `sidecar`. The default duration is 5 minutes if the
   duration is not specified with the `conjur.org/secrets-refresh-interval`.


   `conjur.org/secrets-refresh-enabled` Sets the duration and is a string as defined [here](https://pkg.go.dev/time#ParseDuration).
   Setting a time implicitly enables refresh. Valid time units are `s`, `m`, and `h` 
   (for seconds, minutes, and hours, respectively). Some examples of valid duration 
   strings:<ul><li>`5m`</li><li>`2h30m`</li><li>`48h`</li></ul>The minimum refresh interval is 1 second.
   A refresh interval of 0 seconds is treated as a fatal configuration error.
   ```
   conjur.org/secrets-refresh-enabled: "true"
   conjur.org/secrets-refresh-interval: 10m
   ```

## Reference Table of Configuration Annotations

In addition to the [basic Secrets Provider configuration](https://github.com/cyberark/secrets-provider-for-k8s/blob/main/PUSH_TO_FILE.md#reference-table-of-configuration-annotations),
below is a list of annotations that are needed for secrets rotation.

| K8s Annotation  | Description |
|-----------------------------------------|----------------------------------|
| `conjur.org/container-mode`         | Allowed values: <ul><li>`init`</li><li>`application`</li><li>`sidecar`</li></ul>Defaults to `init`.<br>Must be set (or default) to `init` or `sidecar`for Push to File mode.|
| `conjur.org/secrets-refresh-enabled`  | Set to `true` to enable Secrets Rotation. Defaults to `false` unless `conjur.org/secrets-rotation-interval` is explicitly set. Secrets Provider will exit with error if this is set to `false` and `conjur.org/secrets-rotation-interval` is set. |
| `conjur.org/secrets-refresh-interval` | Set to a valid duration string as defined [here](https://pkg.go.dev/time#ParseDuration). Setting a time implicitly enables refresh. Valid time units are `s`, `m`, and `h` (for seconds, minutes, and hours, respectively). Some examples of valid duration strings:<ul><li>`5m`</li><li>`2h30m`</li><li>`48h`</li></ul>The minimum refresh interval is 1 second. A refresh interval of 0 seconds is treated as a fatal configuration error. The default refresh interval is 5 minutes. The maximum refresh interval is approximately 290 years. |
| `conjur.org/remove-deleted-secrets-enabled` | Set to `false` to disable deletion of secrets files from the shared volume when a secret is removed or access is revoked in Conjur. Defaults to `true`. |

## Troubleshooting

This section describes how to troubleshoot common Secrets Provider for Kubernetes issues.

To enable the debug logs, See [enable-logs](PUSH_TO_FILE.md#enable-logs)

|  Issue      |       Error code   | Resolution  |
| ----------- | ------------------ | ------ |
| No change in secret files, no secret files written |CSPFK018I| This is an info message and not an error. It indicates that the Secrets Provider did not detect a change in secrets for a secrets group. The secret file for this group will not be written. Note: there may be changes in other secret groups and those files will be written. |
| Invalid secrets refresh interval annotation |CSPFK050E| There is an error with the interval annotation, check the log message for the exact failure reason. See the [annotation reference](#reference-table-of-configuration-annotations) for more information on setting the annotations.|
| Invalid secrets refresh configuration |CSPFK051E| Secrets refresh is enabled either by setting `conjur.org/secrets-refresh-enabled` to true or setting a duration for `conjur.org/secrets-refresh-interval` and the mode is not `sidecar`. The mode must be `sidecar`. |

## Limitations

This feature is a **Community** level project that is still under development.
There could be changes to the documentation so please check back often for updates and additions.
Future enhancements to this feature will include:

- atomic writes for multiple Conjur secret values
- reporting of multiple errored secret variables for bulk Conjur secret retrieval and selective deletion of secret values from files
