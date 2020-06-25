# Secrets Provider - Phase 2

## Table of Contents
- [Useful links](#useful-links)
- [Background](#background)
  * [Motivation](#motivation)
  * [Requirement](#requirement)
- [Solution](#solution)
  * [Milestone 1: Deployment](#milestone-1--deployment)
    + [Design](#design)
    + [Flow](#flow)
    + [Customer Experience](#customer-experience)
    + [Code changes](#code-changes)
    + [Fetching Relevant K8s Secrets](#fetching-relevant-k8s-secrets)
      - [**Customer Experience**](#--customer-experience--)
  * [Order of Deployment](#order-of-deployment)
  * [Backwards compatibility](#backwards-compatibility)
  * [Performance](#performance)
  * [Affected Components](#affected-components)
- [Security](#security)
- [Test Plan](#test-plan)
- [Logs](#logs)
  * [Audit](#audit)
- [Documentation](#documentation)
- [Open questions](#open-questions)
- [Implementation plan](#implementation-plan)



## Glossary

| **Term**   | **Description**                                              |
| ---------- | ------------------------------------------------------------ |
| Job        | Task that creates one or more pods that exits once task is successful |
| Deployment | Controllers for managing pods that ensures that its pods are always running. If a pod fails, the controller restarts it |

## Useful links

| Name                | Link                                                         |
| ------------------- | ------------------------------------------------------------ |
| Phase 2 Feature Doc | https://github.com/cyberark/secrets-provider-for-k8s/issues/102 |

## Background

The Cyberark Secrets Provider for K8s enables customers to use secrets stored and managed in Conjur/DAP vault and consume them as K8s Secrets for their application containers. During Phase 1, an *init* container was deployed in the same Pod as the customer's applications and populate the K8s secrets defined in the customer's manifest. 

### Motivation

The current implementation makes the deployment of the Secrets Provider does not support secret rotation and makes the deployment of the Secrets Provider deeply coupled to the customer's applications which not all customers want.

### Requirement

The solution should support the following capabilities:

1. Deployment, deploy as separate Conjur entity; serve K8s Secrets to many Applications
2. Support removal of deployment

## Solution
This solution will be divided into milestones.

- Milestone 1: Serve multiple applications, deploy as as separate entity
- Milestone 2: Handle rotation, updating K8s Secrets with Conjur secret value
- Milestone 3: Support reading secrets from files
- Milestone 4: Listen for K8s Secret creation
- Milestone 5: Trigger K8s Secret update on Conjur secret value update

For the purposes of this document, we will *only* be addressing the first Milestone.



### Milestone 1: Serve Multiple Applications

#### Design

The Secrets Provider for K8s will no longer run as an *init* container attached to the application pod, rather run as a separate single Job in a namespace. A Job creates a Pod to perform a task and terminates upon completion. Once the Pod is spun up, it will authenticate to Conjur/DAP in the same way it does today, via authn-k8s. The Pod will then fetch the K8s secret denoted with a label and update them with the Conjur secrets they require.

|      | Deployment                  | Elaboration                                                  | Pros                                                         | Cons                                                         |
| ---- | --------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 1    | Run as Job                  | Run as a K8s Job that deploys a Pod that terminates after task completion | - Minimal resources usage<br />- Minimal code changes<br />- Explicit user experience (user expects Job to run to completion quickly) | - "Job" is not an existing application identity granularity. <br /><br />- Deployment will need to be changed (from Job → Deployment) for Milestone 2 (support secrets rotation)<br /><br />- For **deployment**, **deploymentConfig** and **statefulSet** segregation of duties, this solution is not interchangeable with our other secrets retrieval solutions without policy changes. (e.g. secrets provider as init-container, conjur authn k8s secrets, any of our clients or Conjur REST API) |
| 2    | Run as non-terminating  Pod | Run as a Pod that does not terminate automatically after task completion | - Simple to deploy<br />                                     | - Wastes resources because we aren't updating K8s secrets and up all the time<br /><br />- For **deployment**, **deploymentConfig** and **statefulSet** segregation of duties, this solution is not interchangeable with our other secrets retrieval solutions without policy changes. (e.g. secrets provider as init-container, conjur authn k8s secrets, any of our clients or Conjur REST API) |

**Decision:** Run as a Job

At current, a "Job" is not an existing application identity granularity. In terms of granularities, a "Job" is at the same level as "Deployment". From our understanding customers do not regularly use "Deployment" as their chose granularity and tend to use "Service Account" / "Namespace". Because of this, support for a "Job" granularity might not provide value when customers can still use "Service Account" / "Namespace".

We decided to run as a Job instead of Deployment because for Milestone 1 we are concerned with delivering Conjur values only once. At this time we are not supporting updating K8s Secrets with Conjur secret across multiple runs so deploying a Pod that will run continuously in the environment of a customer is a waste of their resources. Especially because the process of retrieving secrets will only happen once, at the inital spin up of the Secrets Provider.

Additionally, when using Deployment to spin up a pod and the task finishes, the will container exit and be deleted. Now that the Secrets Provider needs to sit separately, when that container exists so too will the pod. Therefore, when using Deployment, Kuberentes will try to restart the Pod. After a few attempted restarts, the pod will error in a BackOff state. Because of this, we are unable to use Deployment to spin up a pod and must use a Job.

#### Flow

The flow for Milestone 1 is similar to the user experience from Phase 1. For each step, we will go into more detail below.

1. Configure K8s Secrets with ***conjur-map\*** metadata defined 

   - *Optional:* add label to K8s Secret ***(Milestone 1)***

2. Create Service Account for Secrets Provider 

3. Create Role for the Service Accounts with **get**/**update** and also **list** privileges on K8s Secrets resources ***(Milestone 1)***

4. Create RoleBinding and bind the Service Accounts to the role from previous step

5. Create/Deploy Secrets Providers as Jobs in deployment manifest(s) ***(Milestone 1)***
   - If label filtering is defined, add "K8S_SECRET_LABEL" in the Secrets Provider manifest(s) ***(Milestone 1)***

#### Customer Experience

In order to preserve restrictions on Conjur secrets, a customer can decide to either:

1. Deploy 1 Secrets Provider per namespace, giving the Secrets Provider access to all secrets
2. Deploy 1 Secrets Provider per app / (group of apps), giving the Secrets Provider only access to the secrets they have been given access to in Conjur. Should the user require further separation of duties on Conjur secrets, they will need to deploy another Secrets Provider

The Secrets Provider host will resemble the following:

```yaml
- !policy
  id: secrets
  body:
  - !host
    id: secrets-provider-1
    annotations:
      authn-k8s/namespace: prod-namespace
      authn-k8s/service-account: prod-sa
      authn-k8s/authenticator-container-name: cyberark-secrets-provider

  - !layer 

  - !permit
    resources: !variable super_secret
    role: !layer secrets-accessors
    privileges: [ read, execute ]
 
- !grant
  role: !layer secrets-accessors
  members:
    - !host conjur/authn-k8s/secrets-provider-1
```

The Secrets Provider manifest will be defined as the following:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: secrets-provider-job
spec:
  template:
    spec:
      serviceAccountName: prod-sa
      containers:
      - image: secrets-provider:latest
        name: cyberark-secrets-provider-1

# Rest of environment variables for Pod to run 
```

We recommend that the admin will define "Role" instead of "ClusterRole" to follow the principle of least privilege on resources.

#### Code changes

1. Support Job application identity 
2. Enhance Batch retrieval

*Support Job application identity*

To authenticate to Conjur, the Secrets Provider application identity (or Resource Restrictions) will be the characteristics of the Pod that it is running in. Currently, we do not have a "Job" application identity granularity so to allow the Secrets Provider the ability authenticate to Conjur, the customer will need to choose between the Namespace or Service Account granularities. If there is a requirement to support a "Job" granularity this can be done by adding code to the authenticator in Conjur.

*Enhance Batch retrieval*

At current, the Secrets Provider code looks at the "K8S_SECRET" environment variable in each of the Pods and preforms a batch retrieval against the Conjur API to get the relevant Conjur Secrets. If there is a failure in retrieving one of the secrets, the request fails and no secrets are returned to the request Pod. 

This is problematic for us at this stage because now the Secrets Provider is sitting separately and will need to perform a batch retrieval of all the secrets in the namespace. In other words, if the Secrets Provider fails to fetch one Conjur secret, the whole request will fail and applications will not get their secrets. To avoid this, we will parse over each K8s Secret and perform a batch retrieval request. That way, if there is a failure in retrieving a Conjur secret (permissioning for example), it will be contained to that secret and not impact all the secrets in the namespace.

Each API call to Conjur creates an entry in Audit. Now that the Secrets Provider is a separate entity from the application pods, all API calls will come the Secrets Provider and not from the the application Pod. Therefore, if we perform a single batch retrieval call, the Audit will be hard to follow as it will include all Conjur secrets fetched in a single audit entry. This solution will create separate Batch retrieval calls to Conjur for *each* K8s Secret, making the Audit more clear in terms of the behavior of the process.

```pseudocode
for each k8s-secret
  variable-ids = k8s-secret.parseConjurMap() # list of paths of key in ConjurMap
  GET /secrets{?variable-ids}
  # Log failure or success
```

Variable IDs will resemble the following:

```
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
type: Opaque
stringData:
  conjur-map: |-
    username: secrets/db_username # the value is one element in the variable-ids list
    password: secrets/db_password # the value is one element in the variable-ids list
```

#### Fetching Relevant K8s Secrets

With the decoupling of Secrets Provider and application pod, we can know what types of K8s Secrets the pod requires by either of the following ways: 

|      | Fetch/Update K8s Secret                                      | Elaboration                                                  | Pros                                                         | Cons                                        |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------- |
| 1    | Read all K8s Secrets that are marked with labels and update them with Conjur secrets |                                                              | - Scalable <br /><br />- Less intervention on our part as a customer can define which types of secrets we should parse<br /><br />- K8s native, as-is solution | - Not generalized so we are still K8s bound |
| 2    | Get K8s secrets name by configuration in Secrets Provider    | Define K8S_SECRETS in `cyberark-secrets-provider` manifest<br />` name: K8S_SECRET` | - No code changes<br /><br />- Backwards compatible          | - Repetitive work<br /><br />- Not scalable |

To know which K8s Secrets the apps in the namespace need we chose the first solution, Read all K8s Secrets that are marked with labels. It would be the most K8s native and avoids the repetitive work of having to list all the K8s Secrets each app needs.

It is important to note however, that we are responsible for providing a solution for updating the K8s secrets and *not* how those Pods will get those secrets.

##### **Customer Experience**

K8s offers the option of adding labels on K8s secrets. For example, a customer would add a label on a K8s Secret like so:

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
    username: secrets/my_username
```

These labels do not have to be Conjur-specific and the customer would decide which label they would like to put in the K8s secrets. They would define which secrets to iterate over by "K8S_SECRET_LABEL" env variable in their Secrets Provider manifest. The Secrets Provider will perform a search for secrets with that label key against the [K8s](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#secret-v1-core)/[OC](https://docs.openshift.com/container-platform/4.4/rest_api/index.html#secret-v1-core) API and search for the "conjur_map" on the results.

Note that just because the customer added a label for us to filter over doesn't mean that all secret entries in the K8s Secret are stored in Conjur but it gives us a way to perform a more focused search for "conjur-map" in the K8s Secrets.

Secrets Provider Manifest with the K8S_SECRET_LABEL environment variable:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secrets-provider-deployment
spec:
  replicas: 1
  template:
    spec:
      serviceAccountName: sa-1
      containers:
      - image: secrets-provider:latest
        name: cyberark-secrets-provider
   ...
      env:
        - name: K8S_SECRET_LABELS
          value: env-prod,env-test
```

These labels will have an OR inclusive relationship not an AND relationship. In other words, a K8s Secret can either have a CONJUR label or PROD label (or both). A customer can also write '*' to imply all K8s secrets in the namespace. If 'K8S_SECRET_LABELS' is not supplied, by default, the Secrets Provider will fetch all K8s Secrets in the namespace.

To read the labels set on K8s Secrets, the Service Account used for the Secrets Provider will need "list" privileges on K8s Secrets resources.

The Service Account / Role / RoleBinding will be defined as the following:

```yaml
# Service Account definition 
apiVersion: v1
kind: ServiceAccount
metadata:
  name: conjur-sa-1

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
    name: conjur-sa-1
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: secrets-access
  apiGroup: rbac.authorization.k8s.io
```

#### Code changes

Although we will now offer "K8S_SECRET_LABELS" for easy filtering on K8s Secrets, we will still support the "K8S_SECRET" environment variable if the customer prefers to list the K8s Secrets they need values for from Conjur. 



Decision: **Solution #1, Read all K8s Secrets that are marked with labels**

We decided against listing K8s Secrets as configuration in the ENV of the Secrets Provider manifest because that is not a scalable solution, especially for customers with hundreds of K8s Secrets. Customers might already use labels for grouping related K8s Secrets so we will benefit by integrating more with a customer environment and utilizing that.

### Order of Deployment

To ensure that on first run our Secrets Provider runs first, we will request from the customer follow the following setup order:

1. Add all necessary K8s Secrets and their labels to the Secrets Provider manifest

2. Run the Secrets Provider

3. Run application pods

### Lifecycle/Deletion

The lifecycle of the Job is the length of the process. In other words, the time it takes to retrieve the Conjur secrets value and update them in the K8s Secrets. The customer can configure the Job to stay "alive" by adding **ttlSecondsAfterFinished**, with the desired about of seconds, to their Secrets Provider yaml. Because of the nature of a Job, a manual delete is not necessary.

### Backwards compatibility

Milestone 1 of Phase 2 will be built ontop of the current init container solution. We will delivery the same image and will not be erasing code and any current functionality.

### Performance
We will test and document how many secrets can be updated in 5 minutes on average where a secret should be either extreme long password or one vault account which is 5 vars username address port password dns

### Affected Components
- Conjur/DAP: Adding support for Job application identity granularity 

## Security
[//]: # "Are there any security issues with your solution? Even if you mentioned them somewhere in the doc it may be convenient for the security architect review to have them centralized here"

## Test Plan
| **Title** | **Given** | **When** | **Then** | **Comment** |
|-----------|-----------|----------|----------|-------------|
|           |           |          |          |             |
|           |           |          |          |             |

## Logs
| **Scenario**                                                 | **Log message**                                              | Type  |
| ------------------------------------------------------------ | ------------------------------------------------------------ | ----- |
| Job spins up                                                 | Kubernetes Secrets Provider v\*%s\* starting up... (Already exists) | Info  |
| Job has completed and is terminating                         | Kubernetes Secrets Provider v\*%s\* has finished task and is terminating… | Info  |
| Admin does not provide K8S_SECRET_LABELS and does not provide K8S_SECRETS environment variables | Fetching all k8s secrets in namespace…                       | Warn  |
| Admin uses K8S_SECRETS                                       | Warning: K8S_SECRETS is deprecated. Consider using K8S_SECRETS_LABELS... | Warn  |
| Admin provides label that returns no K8s Secrets             | Failed to retrieve k8s secrets (Already exists)              | Error |
| Secrets Provider was unable to provide K8s secret with Conjur value | Failed to retrieve Conjur secrets. Reason: %s (Already exists) | Error |

### Audit 
As mentioned previously, because the Secrets Provider is a separate entity, all API calls will originate from the Secrets Provider and not the application Pod. Therefore Audit entries may be unclear for heavier processes such as batch retrieval. Making batch retrieval API calls for each K8s Secret will help assuage this concern.

## Documentation
[//]: # "Add notes on what should be documented in this solution. Elaborate on where this should be documented. If the change is in open-source projects, we may need to update the docs in github too. If it's in Conjur and/or DAP mention which products are affected by it"

## Open questions
[//]: # "Add any question that is still open. It makes it easier for the reader to have the open questions accumulated here istead of them being acattered along the doc"

## Implementation plan
[//]: # "Break the solution into tasks"
