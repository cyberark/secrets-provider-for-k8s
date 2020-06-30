# Secrets Provider - Phase 2

## Table of Contents
  * [Glossary](#glossary)
  * [Useful links](#useful-links)
  * [Background](#background)
  * [Solution](#solution)
    + [Milestone 1: Serve K8s secrets to multiple applications once (no rotation)](#milestone-1--serve-k8s-secrets-to-multiple-applications-once--no-rotation-)
    + [Design](#design)
    + [Customer Experience](#customer-experience)
    + [Enhancements](#enhancements)
      * [**Customer Experience**](#--customer-experience--)
        + [Permission definitions](#permission-definitions)
    + [Order of Deployment](#order-of-deployment)
    + [Lifecycle/Deletion](#lifecycle-deletion)
    + [Backwards compatibility](#backwards-compatibility)
    + [Performance](#performance)
    + [Affected Components](#affected-components)
  * [Security](#security)
  * [Test Plan](#test-plan)
  * [Logs](#logs)
    + [Audit](#audit)
  * [Documentation](#documentation)
  * [Open questions](#open-questions)
  * [Implementation plan](#implementation-plan)

## Glossary

| **Term**                                                     | **Description**                                              |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [Job](https://kubernetes.io/docs/concepts/workloads/controllers/job/) | K8s entity that runs a pod for one-time operation. Once the pod exists, the Job terminates. |
| [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) | K8s entity that ensures multiple replications of a pod are running all the time. If a pod terminates, another one will be created. |

## Useful links

| Name                | Link                                                         |
| ------------------- | ------------------------------------------------------------ |
| Phase 2 Feature Doc | https://github.com/cyberark/secrets-provider-for-k8s/issues/102 |

## Background

The Cyberark Secrets Provider for K8s enables customers to use secrets stored and managed in Conjur/DAP vault and consume them as K8s Secrets for their application containers.

During Phase 1, an *init container* was deployed in the same Pod as the customer's applications to populate the K8s secrets defined in the pod application's manifest.

### Motivation

The current implementation of the Secrets Provider only supports one application and makes the deployment of the Secrets Provider deeply coupled to the customer's applications.

### Requirements

The solution should support the following capabilities:

Milestone 1 *(current)*

- Secrets Provider runs as a separate entity, serving multiple application containers that run on multiple pods
- Solution needs to be native to Kubernetes
- Lifecycle should support update/removal of deployment
- Provide a way for customer to understand the state of the Secret Provider - when it finished initializing

Milestone 2

- Secrets Provider support secret rotation

### Future requirements

The solution should consider the following future requirements that may arise:

* Support other **targets** for the secrets besides K8s Secrets, such as files
* Support other **sources** for secrets paths besides conjur-map in K8s Secrets
* Support **sources** varying while Secrets Provider runs

## Solution
The solution is to allow different deployments and behave differently based on the use-case chosen.

The following decision tree depicts customer uses-cases for Secrets Provider deployments:

![Secrets Provider flavors desicion flow chart](https://user-images.githubusercontent.com/31179631/85747023-975bf500-b70f-11ea-8e26-1134068fe655.png)

This decision tree shows 6 different variations of Secrets Provider:

| Secrets Provider variation                            | Business value                                               | Implementation status |
| ----------------------------------------------------- | ------------------------------------------------------------ | --------------------- |
| Run as **Init Container** and sync to **K8s Secrets** | Supply secrets on application load, based on K8s secrets     | Implemented           |
| Run as **Job** and sync to **K8s Secrets**            | Supply secrets once for many applications, based on K8s secrets configurations | Milestone 1           |
| Run as **Deployment** and sync to **K8s Secrets**     | Supply secrets for many applications and support rotation, based on K8s secrets | Future (Milestone 2)  |
| Run as **Sidecar** and sync to **K8s Secrets**        | Support secrets rotation, based on K8s secrets               | Future (Milestone 2)  |
| Run as **Init Container** and sync to **files**       | Supply secrets on application load, pushed as files to shared volume | Future (Milestone 3)  |
| Run as **Sidecar** and sync to **files**              | Supply secrets for one application and support rotation, pushed as files to shared volume | Future (Milestone 3)  |

These variations will be supported using the same Secrets Provider image, but the behavior will vary dynamically depending on the chosen deployment.

*In this document we will focus on **Milestone 1**.*



### Milestone 1: Serve K8s secrets to multiple applications once (no rotation)

### Design

Run Secrets Provider as a **Job**.
Once the Job's pod is spun up, it will authenticate to Conjur/DAP via authn-k8s.
It will then fetch all the K8s secrets denoted with a specific label and update them with the Conjur secrets they require and terminate upon completion.

###### How does a Job answer the requirements?

Running as a **Job** allows separation from the applications' deployment and serve multiple applications at once.
When Secrets Provider finishes updating the K8s secrets, the **Job** completes successfully and the pod terminates.

###### What benefits does this solution provide?

1. **Clarity**
   Allowing customers to select the deployment to get the desired value
2. **Maintainability**
   Using one image for all variations simplifies code maintenance 
3. **Reusability**
   Using one image for all variations allows reuse of existing code across all variations
4. **Extensibility**
   Supporting various flows using the same solution opens the door for more secrets sources and targets

###### What drawbacks does this solution have?

**Job** is not an existing application identity granularity.

As a result, one can define the host representing the Secrets Provider in Conjur policy using only **namespace** and/or **service account** granularities.

###### How the drawbacks can be handled?

To handle the missing **Job** granularity there are 2 options:

|      | Solution                                                     | Pros                                                         | Cons                                                         | Effort Estimation |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------- |
| 1    | Keep using only **namespace** and/or **service account** granularities. | - Since Secrets Provider runs as a separate entity, we suggest  creating service account dedicated for Secrets Provider.<br />Then, **service account** granularity serves exact match and **Job** granularity is redundant<br />- No code changes | - Service account might be reused, which allows other apps to authenticate using Secrets Provider host<br />- Break application identity convention | Free              |
| 2    | Add support for **Job** granularity in Conjur                | - Allows specific identification of the Secrets Provider **Job**, which enhances security | - Costly; requires code changes, verify compatibility, tests, docs<br />- Redundant if **service account** serves only the Secrets Provider | 10 days           |

*Decision*: Solution #1, use **namespace** and **service account** to define the Secrets Provider host in Conjur policy.
Recommend to the customer to create a dedicated service account for Secrets Provider.

### Customer Experience

1. Configure K8s Secrets with ***conjur-map\*** metadata defined 

   - *Optional:* add label to K8s Secret
2. Create Service Account for Secrets Provider Job
3. Create Role for the Service Accounts with **get**/**update** and also **list** privileges on K8s Secrets resources
4. Create RoleBinding and bind the Service Accounts to the Role from previous step
5. Create/Deploy Secrets Providers as Jobs in deployment manifest(s)
6. If [label filtering](#fetching-relevant-k8s-secrets) is defined, add `K8S_SECRETS_LABELS` in the Secrets Provider manifest(s)

The Secrets Provider host will resemble the following:

```yaml
- !policy
  id: secrets-accessors
  body:
  - !host
    id: secrets-provider-1
    annotations:
      authn-k8s/namespace: prod-namespace
      authn-k8s/service-account: secret-provider-sa
      authn-k8s/authentication-container-name: cyberark-secrets-provider

  - !group 

  - !permit
    resources: !variable super_secret
    role: !group secrets-accessors
    privileges: [ read, execute ]
 
  - !grant
    role: !group secrets-accessors
    members:
      - !host conjur/authn-k8s/secrets-provider-1
```

The Secrets Provider manifest will be defined as the following:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: secrets-provider-job
  namespace: my-namespace
spec:
  template:
    spec:
      serviceAccountName: secret-provider-sa
      containers:
      - image: secrets-provider:latest
        name: cyberark-secrets-provider-1
        env:
      	 # See enhancements
```

Service Account and secret definitions can be found [below](#permission-definitions).

### Enhancements

For this Milestone we recommend also tackling the following enhancements:

1. Update how we fetch Conjur secrets
2. Enhance how we fetch relevant K8s Secrets
3. Differentiate between deployment types using K8s API
4. Remove use of downward API
5. Do not update K8s Secret that with the same value

#### Update how we fetch Conjur secrets

At current, the Secrets Provider code looks at the `K8S_SECRETS` environment variable in each of the pod manifests and preforms a batch retrieval against the Conjur API to get the relevant Conjur Secrets. If there is a failure in retrieving one of the secrets, the request fails and no secrets are returned to the request Pod. 

This is problematic for us at this stage because now the Secrets Provider is sitting separately and will need to perform a batch retrieval of all the secrets in the namespace. In other words, if the Secrets Provider fails to fetch one Conjur secret, the whole request will fail and applications will not get their secrets. 

##### Code changes

To avoid this, we will parse over each K8s Secret and perform a batch retrieval request. That way, if there is a failure in retrieving a Conjur secret (a failure in permissioning for example), it will be contained to that secret and not impact all the secrets in the namespace.

```pseudocode
for each k8s-secret
  variable-ids = k8s-secret.parseConjurMap() # list of paths of key in ConjurMap
  GET /secrets{?variable-ids}
  # Log failure or success
```

Variable IDs will resemble the following:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
type: Opaque
stringData:
  conjur-map: |-
    username: secrets-accessors/db_username # the value is one element in the variable-ids list
    password: secrets-accessors/db_password # the value is one element in the variable-ids list
```

#### Fetching how we fetch relevant K8s Secrets

With the decoupling of Secrets Provider and application pod, we can know what types of K8s Secrets the pod requires by either of the following ways: 

|      | Fetch/Update K8s Secret                                      | Pros                                                         | Cons                                                         | UX                                                           | Effort estimation |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------- |
| 1    | Get K8s secrets with the names specified in `K8S_SECRETS`    | - No code changes<br /><br />- Backwards compatible          | - Repetitive work<br />- Not general solution so we are still K8s bound<br />- Not scalable | List K8S Secrets one-by-one                                  | Free              |
| 2    | Read all K8s Secrets that are marked with labels specified in `K8S_SECRETS_LABEL` env var | - Scalable grouping <br /><br />- Less intervention on our part as a customer can define which types of secrets we should parse<br /><br />- K8s native, as-is solution | - Not general solution so we are still K8s bound<br />- Demands SA has "list" privileges on secrets | Add "list" to SA<br />`verbs: ["get", "update", "list"]`<br />List label grouping in Job manifest | 3 days            |

Labels in K8s are key/value pairs that are attached to K8s objects. They are used to identify objects using attributes.

*Decision*: Solution #2, Read all K8s Secrets that are marked with labels. It would be the most K8s native and avoids the repetitive work of having to list all the K8s Secrets each app needs.

Listing each K8s Secret in the ENV of the Secrets Provider manifest is not a scalable solution, especially for customers with hundreds of K8s Secrets. Customers might already use labels for grouping related K8s Secrets so we will benefit from this enhancement by integrating more with a customer environment.

Even with this new addition, for backwards compatibility, we will still be supporting customers using `K8S_SECRETS`. Flow will be discussed in [code changes](#code-changes-1) below.

In order to allow for customer feedback we recommend that this feature be listed as a beta feature.

##### **Customer Experience**

A customer would add a label on a K8s Secret like so:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-secret
  labels:
    configurable-key: env-prod # label name can be configurable
  type: Opaque
stringData:
  conjur-map: |-
    username: secrets-accessors/db_username
    password: secrets-accessors/db_password
```

These labels do not have to be Conjur-specific and the customer would decide which label to attach to the K8s secrets. They would define for us which secrets to iterate over by detailing `K8S_SECRETS_LABELS` env variable in their Secrets Provider manifest. The Secrets Provider will perform a search for K8s Secrets with that label key-value pair against the [K8s](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#secret-v1-core)/[OC](https://docs.openshift.com/container-platform/4.4/rest_api/index.html#list-14) API and search for the "conjur_map" on the results.

Note that just because the customer added a label for us to filter over doesn't mean that all secret entries in the K8s Secret are stored in Conjur but it gives us a way to perform a more focused search for "conjur-map" in the K8s Secrets.

Secrets Provider Manifest with the `K8S_SECRETS_LABELS` environment variable:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: secrets-provider-job
spec:
  template:
    spec:
      serviceAccountName: secret-provider-sa
      containers:
      - image: secrets-provider:latest
        name: cyberark-secrets-provider-1
   ...
      env:
        - name: K8S_SECRETS_LABELS
          value: 'configurable-key:env-prod,environment:prod'
          
# Rest of environment variables for Pod to run    
```

These labels will have an OR inclusive relationship, not an AND relationship. A customer can also write '*' to imply fetching all K8s secrets in the namespace. 

To be able to read the labels set on K8s Secrets, the Service Account used for the Secrets Provider will need "list" privileges on K8s Secrets resources.

###### Permission definitions

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
  verbs: ["get", "update", "list"] # new privilege here

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

We recommend that the admin define `Role` and not `ClusterRole` to follow the principle of least privilege on resources (manifests are documented below).

##### Code changes

Although we will now offer `K8S_SECRETS_LABELS` for easy filtering on K8s Secrets, we will still support the `K8S_SECRETS` environment variable if the customer prefers to list the K8s Secrets individually.

Customers will be able to use a combination of `K8S_SECRETS` and `K8S_SECRETS_LABELS` if there are K8s Secrets without existing labels on them. 

Changes will take place in `config.go` of codebase where we will check for the existance of either `K8S_SECRETS` or `K8S_SECRETS_LABELS`. If `K8S_SECRETS_LABELS`, we will make an additional request to the K8s API to fetch all K8s Secrets with the defined labels. The K8s Secrets we get back will be used to fill the `requiredK8sSecret` field in the Config object.

The flow is as follows:

```pseudocode
K8S_SECRETS exists in manifest?
	YES? K8S_SECRETS_LABELS exists in manifest?
    YES? Fetch k8s secrets defined under K8S_SECRETS
         Fetch k8s secrets with label defined under K8S_SECRETS_LABELS
    NO? Fetch k8s secrets defined under K8S_SECRETS
  NO? Log failure and exit
```

#### Differentiate between deployment types using K8s API

**Current solution**: Customer supplies this value in `CONTAINER_MODE` env var to Secrets Provider.

**Enhanced solution**: Use K8s API `GET namespaces/{namespace}/pods/{pod}` to get the pod's manifest and derive the deployment used for Secrets Provider.

**Motivation**: better UX, no need to maintain another env var. Also, ensures correct value and prevent mistakes.
**Requirements**: Add `get` rights for `pods` to Secrets Provider's service account in K8s. This change will need to be done in the *authn-client.*

**Backwards compatibility:** 

```pseudocode
CONTAINER_MODE exists?
	YES? Set authnConfig.ContainerMode from environment variable
	NO? 
	  Service account has "get" privileges on Pods? 
	  	YES? Set authnConfig.Mode from API
	  	NO? Log failure and exit
```

#### Get `MY_POD_NAME` / `MY_POD_NAMESPACE`

**Current solution**: Customer supplies these values using [Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/) for `MY_POD_NAME` and `MY_POD_NAMESPACE` env vars in Secrets Provider manifest.

**Enhanced solution**: Get pod's namespace from `/var/run/secrets/kubernetes.io/serviceaccount/namespace` file inside the container.
Get pod's name from `HOSTNAME` env var (as documented in [K8s docs](https://kubernetes.io/docs/concepts/containers/container-environment/#container-information)) 

**Motivation**: better UX, no need to maintain another env vars.

**Limitation**: `HOSTNAME` is the pod's name only if customer didn't [supply a hostname explicitly](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-hostname-and-subdomain-fields).

**Solution**: Use `HOSTNAME` if `MY_POD_NAME` is not supplied.

#### Do not update K8s secret with the same value

**Current solution**: For each K8s secret, pull its values from Conjur and write them into the K8s secret.

**Enhanced solution**: Before writing the K8s secret, compare it with the existing value. If the value is the same, skip its writing.

**Motivation**:

1. Less stress on K8s API in case of many k8s secrets that are already updated.
2. Remove redundant notifications because every change to K8s resource sends a notification to all watchers.
3. Improved performance due to less API calls



### Order of Deployment

Steps to follow for successful deployment

1. Add all necessary K8s Secrets and their labels to the Secrets Provider manifest
2. Run the Secrets Provider Job**
3. Run application pods**

**The order at which the Secrets Provider and application pod are deployed does not matter because the [application pod will not start](https://kubernetes.io/docs/concepts/configuration/secret/#details) until it has received the K8s Secrets it references (secretKeyRef). Until the 

### Lifecycle/Deletion

The lifecycle of the Job is the length of the process. In other words, the time it takes to retrieve the Conjur secrets value and update them in the K8s Secrets. The customer can configure the Job to stay "alive" by adding **ttlSecondsAfterFinished**, with the desired about of seconds, to their Secrets Provider yaml. Because of the nature of a Job, a manual delete is not necessary.

### Backwards compatibility

Milestone 1 of Phase 2 will be built ontop of the current init container solution. We will deliver the same image and will not break current functionality.

*K8S_SECRETS_LABELS*

Although we will now offer `K8S_SECRETS_LABELS` for easy filtering on K8s Secrets, we will still support the `K8S_SECRETS` environment variable if the customer prefers to list the K8s Secrets they need values for from Conjur. If so, they will not need to add "list" permissions on their Service Account. 

Customers will be able to use a combination of `K8S_SECRETS` and `K8S_SECRETS_LABELS` if there are K8s Secrets without existing labels on them. 

### Performance
*See performance tests.* 

We will test and document how many K8s Secrets can be updated in 5 minutes on average. A secret should be either extreme long password.

### Affected Components

- K8s authenticator client (`CONTAINER_MODE` enhancement)

## Security
## Test Plan

### Integration tests

|      | **Title**                                                    | **Given**                                                    | **When**                                                     | **Then**                                                     | **Comment**                                |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------ |
| 1    | *Vanilla flow*, Secret Provider Job successfully updates K8s Secrets | - Conjur is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and K8s Secrets are configured<br />- Service Account has correct permissions (get/update/list) <br />- Secrets Provider Job manifest is defined<br />- `K8S_SECRETS_LABELS` (or `K8S_SECRETS`) env variable is configured | Secrets Provider runs as a Job                               | - Secrets Provider pod authenticates and fetches Conjur secrets successfully<br />- All K8s Secrets with label(s) defined in `K8S_SECRETS_LABELS` are updated with  Conjur value <br />- App pods receive K8s Secret with Conjur secret values as environment variable<br />- Secrets Provider Job terminates on completion of task<br />- Verify logs |                                            |
| 2    | Secret Provider Job updates K8s Secrets                      | - Without `K8S_SECRETS_LABELS` <br />- Without `K8S_SECRETS` env variable configured in Job manifest | Secrets Provider runs as a Job                               | Failure on missing environment variable. Either `K8S_SECRETS_LABELS` or `K8S_SECRETS` must be provided<br />- Failure is logged |                                            |
| 2.1  | Empty `K8S_SECRETS_LABELS` value list                        | - `K8S_SECRETS_LABELS` env variable configured but the list is empty<br />- Without `K8S_SECRETS` env variable | Secrets Provider runs as a Job                               | Failure on missing env variable value<br />- Failure is logged |                                            |
| 2.2  | `K8S_SECRETS_LABELS` with value                              | - `K8S_SECRETS_LABELS` env variable configured with label that is not attached to any K8s Secret | Secrets Provider runs as a Job                               | Failure because no secret exists with that label<br />- Failure is logged |                                            |
| 2.3  | Empty `K8S_SECRETS_LABEL` value list with `K8S_SECRETS`      | - `K8S_SECRETS_LABEL` env variable configured but the list is empty<br />- `K8S_SECRETS` env variable configured | Secrets Provider runs as a Job                               | - `K8S_SECRETS` takes precedence. All K8s Secrets defined under `K8S_SECRETS` will be updated |                                            |
| 2.4  | K8S_SECRETS ***backwards compatibility***                    | - `K8S_SECRETS` and `K8S_SECRETS_LABELS` env variable configured | Secrets Provider runs as a Job                               | - K8s Secrets defined under `K8S_SECRETS` and all K8s Secrets that have the label defined under `K8S_SECRETS_LABELS`will be updated<br />- Verify logs |                                            |
| 3    | Secret Provider Service Account has insuffient privileges ("list") | - Service Account lacks "list" permissions on K8s Secrets<br />`K8S_SECRETS_LABELS`<br />and `K8S_SECRETS` is *not* | Secrets Provider runs as a Job                               | - Failure on retrieving K8s Secret due to incorrect permissions given to Service Account<br />- Failure is logged |                                            |
| 3.1  | Service Account with insuffient privileges ("list")          | - Service Account lacks "list" permissions on K8s Secrets<br />- `K8S_SECRETS_LABELS` env variable is *not* configured<br />and `K8S_SECRETS` is | Secrets Provider runs as a Job                               | - All K8s Secrets defined under `K8S_SECRETS` environment variable in Job manifest will be updated<br />- Verify logs |                                            |
| 4    | Batch retrieval failure                                      | - Service Account has correct permissions (get/update/list)<br />- `K8S_SECRETS_LABELS` env variable or `K8S_SECRETS` is configured<br /> | Secrets Provider runs as a Job<br />- Host doesn't have permissions on Conjur secret | - Failure to fetch *specific* secret without harming batch retrieval for rest of the secrets <br />- Failure is logged defining which Conjur secret(s) failed |                                            |
| 5    | *Vanilla flow*, non-conflicting Secrets Provider<br />       | Two Secrets Providers have access to different Conjur secret | 2 Secrets Provider runs as a Job in same namespace           | - All relevant K8s secrets are updated<br />- Verify logs    |                                            |
| 6    | Non-conflicting Secrets Provider 1 namespace same K8s Secret | Two Secrets Providers have access to same  secret            | 2 Secrets Provider runs as a Job in same namespace           | - No race condition and Secrets Providers will not  override each other<br />- Verify logs |                                            |
| 7    | *Regression tests*                                           |                                                              |                                                              | All regression tests should pass                             | All init container tests should still pass |

### Unit tests

|      | Given                                                        | When                                                         | Then                                                 | Comment                                                    |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ---------------------------------------------------- | ---------------------------------------------------------- |
| 1    | `K8S_SECRETS_LABEL` defined in env but no ‘list’ permissions on the 'secrets' k8s resource | When Secrets Provider attempts to fetch labels               | Fail and return proper permissions error             |                                                            |
| 2    | Given `K8S_SECRETS_LABEL` label list returns no K8s secrets  | When Secrets Provider attempts to fetch labels               | Fail and return proper error                         |                                                            |
| 3    | Given one K8s secret with conjur-map                         |                                                              | Validate content of conjur-map (encoded, size)       | *Security test* that we are properly handling input values |
| 4    | *Update existing UT*<br />Missing environment variable (`K8S_SECRETS_LABEL` or `K8S_SECRETS`) | Secrets Provider Job is run                                  | Fail and return proper missing environment var error |                                                            |
| 5    | Given one K8s secret with a Conjur secret value              | When Secrets Provider Job runs a second time and attempts to update | Returns proper "already up-to-date" info log         |                                                            |

### Performance

|      | **Title**                 | **Given**                                                    | **When**                       | **Then**                                                     | **Comment** |
| ---- | ------------------------- | ------------------------------------------------------------ | ------------------------------ | ------------------------------------------------------------ | ----------- |
| 1    | Performance investigation | - 1000 Secrets Providers defined<br />- Conjur secrets with max amount of characters | Secrets Provider runs as a Job | Evaluate how many K8s Secrets can be updated with Conjur secrets in 5 minutes |             |
| 2    | Performance investigation | - 1000 Secrets Providers defined<br />- Conjur secret with average amount of characters | Secrets Provider runs as a Job | Evaluate how many K8s Secrets can be updated with Conjur secrets in 5 minutes |             |


## Logs

| **Scenario**                                                 | **Log message**                                              | Type  |
| ------------------------------------------------------------ | ------------------------------------------------------------ | ----- |
| Job spins up                                                 | Kubernetes Secrets Provider v\*%s\* starting up... (Already exists) | Info  |
| Job has completed and is terminating                         | Kubernetes Secrets Provider has finished task and is terminating… | Info  |
| Admin supplies `K8S_SECRETS_LABELS` value or *               | Fetching all k8s secrets with label %s in namespace…         | Info  |
| Admin does not provide `K8S_SECRETS_LABELS` or `K8S_SECRETS` environment variables | Environment variable `K8S_SECRETS_LABELS` or `K8S_SECRETS` must be provided... | Error |
| Secrets Provider tries to update K8s Secret but value is up-to-date. Details number of K8s secrets that are being skipped. | Already up-to-date. Skipping update for %s k8s secret(s) from namespace '%s'... | Info  |
| Secrets Provider tries to update K8s Secret but value is up-to-date | Already up-to-date. Skipping update for k8s secret(s) [%s, %s..] from namespace '%s'... | Debug |
| Admin provides label that does not return K8s Secrets (empty list) | Failed to retrieve k8s secrets with label %s                 | Error |
| `K8S_SECRETS_LABELS` key-value pairs has invalid character, K8s API has problem with value (400 error) | Invalid characters in `K8S_SECRETS_LABELS`                   | Error |
| Secrets Provider was unable to provide K8s secret with Conjur value | Failed to retrieve Conjur secrets. Reason: %s (Already exists) | Error |

### Audit 
All fetches on a Conjur Resource are individually audited, creating its own audit entry. Therefore there will be no changes in audit behavior for this Milestone. 

## Documentation
[//]: # "Add notes on what should be documented in this solution. Elaborate on where this should be documented. If the change is in open-source projects, we may need to update the docs in github too. If it's in Conjur and/or DAP mention which products are affected by it"

## Open questions
[//]: # "Add any question that is still open. It makes it easier for the reader to have the open questions accumulated here istead of them being acattered along the doc"

## Implementation plan
### Delivery plan

- [ ] Solution design approval + Security review approval
- [ ] Golang Ramp-up ***(~3 days)\***
  - [ ] Get familiar with Secrets Provider code and learn Go and Go Testing
- [ ] Create dev environment ***(~5 days)\***
  - [ ] Research + Implement HELM
- [ ] Implement Phase 2 Milestone 1 functionality ***(~2 days)\***
- [ ] Enhancements, improve existing codebase ***(~7 days)\***
  - [ ] `K8S_SECRETS_LABELS`
  - [ ] Batch retrival enhancement
  - [ ] References to downward API in manifest (`MY_POD_NAME` and  `MY_POD_NAMESPACE`) and other ENV vars (`CONTAINER_MODE` - *TBD*)
- [ ] Implement test plan (Integration + Unit + Performance tests align with SLA) ***(~7 days)\***
- [ ] Security items have been taken care of (if they exist) ***(TBD)\***
- [ ] Logs review by TW + PO ***(~1 day)\*** 
- [ ] Documentation has been given to TW + approved ***(~2 days)\***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(~1 day)\***
- [ ] Create demo for Milestone 1 functionality ***(~1 day)\***
- [ ] Versions are bumped in all relevant projects (if necessary) ***(~1 days)\***

**Total:** ~30 days of non-parallel work **(~6 weeks)**

*Risks that could delay project completion*

1. New language (Golang), new testing (GoConvey), and new platform (K8S/OC)
2. Cross-project work/dependency
   - Some changes may involve changes in authn-client
   - Mitigation: explore / outline impact our changes will have on dependent projects and raise them early