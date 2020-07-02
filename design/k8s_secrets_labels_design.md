# K8s Secrets Labels Design

## Table of Contents

[//]: # "You can use this tool to generate a TOC - https://ecotrust-canada.github.io/markdown-toc/"

## Glossary

| **Term**                                                     | **Description**                                              |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [Label](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) | Key/value pairs that are attached to K8s objects. They are used to identify and group objects for effective querying and selection. |

## Background

As part of Phase 2 for Secrets Provider for K8s effort, we want to improve the customer experience by offering the ability to use labels for easy identification of K8s secrets whose secret values should come from Conjur. This will allow customers to list in terms of groupings instead of having to list the K8s secrets individually.

## Issue description

At current, the Secrets Provider code looks at the `K8S_SECRETS` environment variable in each of the pod manifests to know which K8s Secret to update with Conjur values. 

Listing each K8s Secret in this way is not a scalable solution, especially for customers with hundreds of K8s Secrets. 

## Solution

We will offer the customer an *additional* option to define which K8s Secrets values are hosted in Conjur by defining them in terms of a label instead of individually. They will define the label that is attached to the K8s Secret and add it under `K8S_SECRETS_LABEL` environment variable in the Secrets Provider manifest. 

The Secrets Provider will then make a separate API call to [K8s](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#secret-v1-core)/[OC](https://docs.openshift.com/container-platform/4.4/rest_api/index.html#list-14), fetching all K8s Secrets with that label. We will use that information to fetch their values in Conjur and update the K8s Secrets.

This enhancement can be used in the current *init solution* and will help build the foundation for later Milestones.

### Design

|      | Fetch/Update K8s Secret                                      | Pros                                                         | Cons                                                         | UX                                                           | Effort estimation |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------- |
| 1    | Get K8s secrets with the names specified in `K8S_SECRETS`    | - No code changes  - Backwards compatible                    | - Repetitive work <br />- Not scalable                       | List K8S Secrets one-by-one                                  | Free              |
| 2    | Read all K8s Secrets that are marked with labels specified in `K8S_SECRETS_LABEL` env var | - Scalable grouping  - Less intervention on our part as a customer can define which types of secrets we should parse  - K8s native, as-is solution | - Not general solution so we are still K8s bound - Demands SA has "list" privileges on secrets | Add "list" to SA `verbs: ["get", "update", "list"]` <br />- List label grouping in Secrets Provider manifest<br />- Label key `conjur-secrets-label` will need to be added to each K8s Secret | 5 days            |

*Decision*: Solution #2, Read all K8s Secrets that are marked with labels. It would be the most K8s native and avoids the repetitive work of having to list all the K8s Secrets each app needs. 

### Customer Experience 

A customer would add a label on a K8s Secret like so:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-secret
  labels:
    conjur-secrets-label: prod # label name can be configurable
  type: Opaque
stringData:
  conjur-map: |-
    username: secrets-accessors/db_username
    password: secrets-accessors/db_password
```

Secrets Provider Manifest with the `K8S_SECRETS_LABEL` environment variable:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: secrets-provider-job
  namespace: app-namespace
spec:
  template:
    spec:
      serviceAccountName: secret-provider-sa
      containers:
      - image: secrets-provider:latest
        name: cyberark-secrets-provider-1
        env:
        - name: SECRETS_DESTINATION
          value: k8s_secrets

        - name: K8S_SECRETS_LABEL
          value: conjur-secrets-label:prod
        
        # K8s authn-client configurations (CONJUR_APPLIANCE_URL, CONJUR_AUTHN_URL, CONJUR_ACCOUNT, CONJUR_SSL_CERTIFICATE) 
```

These labels will have an OR inclusive relationship, not an AND relationship. The label key will not be configurable and will have to be `conjur-secrets-label`. In other words, to use this enhancement, they will need to add `conjur-secrets-label` as an entry under labels.

Utilizing labels is a more K8s native solution and avoids the repetitive work of having to list all the K8s Secrets each app needs.

To be able to read the labels set on K8s Secrets, the Service Account used for the Secrets Provider will need "list" privileges on K8s Secrets resources.

The Service Account / Role / RoleBinding will be defined as the following:

```yaml
# Service Account definition 
apiVersion: v1
kind: ServiceAccount
metadata:
  name: secret-provider-sa

# Role definition
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: app-namespace
  name: secrets-access
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "update", "list"] # "list" is a new privilege here

# RoleBinding definition to associate the SA with the Role
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: secrets-access-binding
  namespace: app-namespace
subjects:
  - kind: ServiceAccount
    name: secret-provider-sa
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: secrets-access
  apiGroup: rbac.authorization.k8s.io
```

#### Code Changes

Although we will now offer `K8S_SECRETS_LABEL` for easy filtering on K8s Secrets, we will still support the `K8S_SECRETS` environment variable if the customer prefers to list the K8s Secrets individually. 

We will also support the use of `K8S_SECRETS` and `K8S_SECRETS_LABEL` if there are K8s Secrets without  labels on them.

Changes will take place in `config.go` of codebase where we will check for the existance of either `K8S_SECRETS` or `K8S_SECRETS_LABEL`. If `K8S_SECRETS_LABEL`, we will make an additional request to the K8s API to fetch all K8s Secrets with the defined labels. The K8s Secrets we get back will be used to fill the `requiredK8sSecret` field in the Config object.

The flow is as follows:

```pseudocode
K8S_SECRETS exists in manifest?
	YES? K8S_SECRETS_LABELS exists in manifest?
    YES? Fetch k8s secrets defined under K8S_SECRETS
         Fetch k8s secrets with label defined under K8S_SECRETS_LABELS
    NO? Fetch k8s secrets defined under K8S_SECRETS
  NO? Log failure and exit
```

### Backwards compatibility

Although we will now offer `K8S_SECRETS_LABELS` for easy filtering on K8s Secrets, we will still support the `K8S_SECRETS` environment variable if the customer prefers to list the K8s Secrets they need values for from Conjur. If so, they will not need to add "list" permissions on their Service Account.

Customers will be able to use a combination of `K8S_SECRETS` and `K8S_SECRETS_LABELS` if there are K8s Secrets without existing labels on them.

### Affected Components

- Secrets Provider for K8s

## Security

[//]: # "Are there any security issues with your solution? Even if you mentioned them somewhere in the doc it may be convenient for the security architect review to have them centralized here"

## Test Plan

### Integration

|      | ** Title**                                                   | **Given**                                                    | **When**                       | **Then**                                                     | **Comment**                                |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------ | ------------------------------------------------------------ | ------------------------------------------ |
| 1    | *Vanilla flow*, Secret Provider Job successfully updates K8s Secrets | - Conjur is running - Authenticator is defined - Secrets defined in Conjur and K8s Secrets are configured - Service Account has correct permissions (get/update/list) - Secrets Provider Job manifest is defined - `K8S_SECRETS_LABELS` (or `K8S_SECRETS`) env variable is configured | Secrets Provider runs as a Job | - Secrets Provider pod authenticates and fetches Conjur secrets successfully - All K8s Secrets with label(s) defined in `K8S_SECRETS_LABELS` are updated with Conjur value - App pods receive K8s Secret with Conjur secret values as environment variable <br />- Secrets Provider Job terminates on completion of task <br />- Verify logs |                                            |
| 2    | Secret Provider Job updates K8s Secrets                      | - Without `K8S_SECRETS_LABELS` - Without `K8S_SECRETS` env variable configured in Job manifest | Secrets Provider runs as a Job | Failure on missing environment variable. Either `K8S_SECRETS_LABELS` or `K8S_SECRETS` must be provided <br />- Failure is logged |                                            |
| 2.1  | Empty `K8S_SECRETS_LABELS` value list                        | - `K8S_SECRETS_LABELS` env variable configured but the list is empty - Without `K8S_SECRETS` env variable | Secrets Provider runs as a Job | Failure on missing env variable value <br />- Failure is logged |                                            |
| 2.2  | `K8S_SECRETS_LABELS` with value                              | - `K8S_SECRETS_LABELS` env variable configured with label that is not attached to any K8s Secret | Secrets Provider runs as a Job | Failure because no secret exists with that label <br />- Failure is logged |                                            |
| 2.3  | Empty `K8S_SECRETS_LABEL` value list with `K8S_SECRETS`      | - `K8S_SECRETS_LABEL` env variable configured but the list is empty - `K8S_SECRETS` env variable configured | Secrets Provider runs as a Job | - `K8S_SECRETS` takes precedence. All K8s Secrets defined under `K8S_SECRETS` will be updated |                                            |
| 2.4  | K8S_SECRETS ***backwards compatibility***                    | - `K8S_SECRETS` and `K8S_SECRETS_LABELS` env variable configured | Secrets Provider runs as a Job | - K8s Secrets defined under `K8S_SECRETS` and all K8s Secrets that have the label defined under `K8S_SECRETS_LABELS`will be updated - Verify logs |                                            |
| 3    | Secret Provider Service Account has insuffient privileges ("list") | - Service Account lacks "list" permissions on K8s Secrets `K8S_SECRETS_LABELS` and `K8S_SECRETS` is *not* | Secrets Provider runs as a Job | - Failure on retrieving K8s Secret due to incorrect permissions given to Service Account - Failure is logged |                                            |
| 3.1  | Service Account with insuffient privileges ("list")          | - Service Account lacks "list" permissions on K8s Secrets - `K8S_SECRETS_LABELS` env variable is *not* configured and `K8S_SECRETS` is | Secrets Provider runs as a Job | - All K8s Secrets defined under `K8S_SECRETS` environment variable in Job manifest will be updated |                                            |
| 4    | * Regression tests*                                          |                                                              |                                | All regression tests should pass                             | All init container tests should still pass |

### Unit tests

|      | **Given**                                                    | **When**                                                     | **Then**                                             | **Comment**                                                |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ---------------------------------------------------- | ---------------------------------------------------------- |
| 1    | `K8S_SECRETS_LABEL` defined in env but no ‘list’ permissions on the 'secrets' k8s resource | When Secrets Provider attempts to fetch labels               | Fail and return proper permissions error             |                                                            |
| 2    | Given `K8S_SECRETS_LABEL` label list returns no K8s secrets  | When Secrets Provider attempts to fetch labels               | Fail and return proper error                         |                                                            |
| 3    | Given one K8s secret with conjur-map                         |                                                              | Validate content of conjur-map (encoded, size)       | *Security test* that we are properly handling input values |
| 4    | *Update existing UT* Missing environment variable (`K8S_SECRETS_LABEL` or `K8S_SECRETS`) | Secrets Provider Job is run                                  | Fail and return proper missing environment var error |                                                            |
| 5    | Given one K8s secret with a Conjur secret value              | When Secrets Provider Job runs a second time and attempts to update | Returns proper "already up-to-date" info log         |                                                            |

## Logs

| **Scenario**                                                 | **Log message**                                              | Type  |
| ------------------------------------------------------------ | ------------------------------------------------------------ | ----- |
| Admin supplies `K8S_SECRETS_LABELS` value                    | Fetching all k8s secrets with label %s in namespace…         | Info  |
| Admin does not provide `K8S_SECRETS_LABELS` or `K8S_SECRETS` environment variables | Environment variable `K8S_SECRETS_LABELS` or `K8S_SECRETS` must be provided... | Error |
| Admin provides label that does not return K8s Secrets (empty list) | Failed to retrieve k8s secrets with label %s                 | Error |
| `K8S_SECRETS_LABELS` key-value pairs has invalid character, K8s API has problem with value (400 error) | Invalid characters in `K8S_SECRETS_LABELS`                   | Error |
| Secrets Provider was unable to provide K8s secret with Conjur value | Failed to retrieve Conjur secrets. Reason: %s (Already exists) | Error |

## Open questions

1. Should we write documentation for this feature?

## Implementation plan

### Delivery plan

-  Solution design approval + Security review approval
-  Golang Ramp-up ***(~3 days)***
  -  Get familiar with Secrets Provider code and learn Go and Go Testing
-  Create dev environment ***(~2 days)***
-  Implement  `K8S_SECRETS_LABELS` enhancement ***(~5 days)***
-  Implement test plan (Integration + Unit)  ***(~5 days)***
-  Logs review by TW + PO   ***(~1 days)***
- Create demo for Milestone 1 functionality  ***(~1 day)***
-  Versions are bumped in all relevant projects (if necessary)  ***(~1 day)***

**Total:** ~18 days of non-parallel work **(~3.5 weeks)**

*Risks that could delay project completion*

1. New language (Golang), new testing (GoConvey), and new platform (K8S/OC)

   