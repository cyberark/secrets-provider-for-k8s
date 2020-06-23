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
  * [Milestone 2: Update K8s secret with Conjur secret](#milestone-2--update-k8s-secret-with-conjur-secret)
    + [Design](#design-1)
    + [Flow](#flow-1)
    + [Customer experience](#customer-experience)
    + [Code changes](#code-changes-1)
  * [Order of Deployment](#order-of-deployment)
      - [Milestone 1 and 2](#milestone-1-and-2)
  * [Backwards compatibility](#backwards-compatibility)
      - [Milestone 1 and 2](#milestone-1-and-2-1)
  * [Performance](#performance)
  * [Affected Components](#affected-components)
      - [Milestone 1](#milestone-1)
      - [Milestone 2](#milestone-2)
- [Security](#security)
- [Test Plan](#test-plan)
- [Logs](#logs)
  * [Audit](#audit)
- [Documentation](#documentation)
- [Open questions](#open-questions)
- [Implementation plan](#implementation-plan)



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
2. Rotation, update K8s Secret with Conjur secret values
3. Support removal of deployment

## Solution
This solution will be divided into milestones.

- Milestone 1: Deployment, deploy as as separate entity
- Milestone 2: Handle rotation, updating K8s Secrets with Conjur secret value

### Milestone 1: Deployment

#### Design

The Secrets Provider for K8s will no longer run as an *init* container attached to the application pod, rather run as a separate single Job in a namespace. A Job creates a Pod to perform a task and terminates upon completion. Once the Pod is spun up, it will authenticate to Conjur/DAP in the same way it does today, via authn-k8s. The Pod will then fetch the K8s secret denoted with a label and update them with the Conjur secrets they require.

|      | Deployment                  | Elaboration                                                  | Pros                                                         | Cons                                                         |
| ---- | --------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 1    | Run as Job                  | Run as a K8s Job that deploys a Pod that terminates after task completion | - Minimal resources usage<br />- Minimal code changes<br />- Explicit user experience (user expects Job to run to completion quickly) | - "Job" is not an existing application identity granularities. Adding it can take low priority since customers can run the Job using Namespace / Service Account granularities<br /><br />- Deployment will need to be changed (from Job → Pod) for Milestone 2 (support secrets rotation) |
| 2    | Run as non-terminating  Pod | Run as a Pod that does not terminate automatically after task completion | - Simple to below<br />                                      | - Wastes resources because we aren't updating K8s secrets and up all the time<br /><br />- For **deployment**, **deploymentConfig** and **statefulSet** segregation of duties, this solution is not interchangeable with our other secrets retrieval solutions without policy changes. (e.g. secrets provider as init-container, conjur authn k8s secrets, any of our clients or Conjur REST API) |

**Decision:** Run as a Job

We decided to run as a Job instead of a Pod because for Milestone 1 we are concerned with delivering Conjur values only once. At this time we are not supporting updating K8s Secrets with Conjur secret across multiple runs so deploying a Pod that will run continuously in the environment of a customer is a waste of their resources. Especially because the process of retrieving secrets will only happen once, at the inital spin up of the Secrets Provider.

#### Flow

The flow for Milestone 1 is similar to the user experience from Phase 1. For each step, we will go into more detail below.

1. Configure K8s Secrets with ***conjur-map\*** metadata defined 

   - *Optional:* add label to K8s Secret ***(Milestone 1)***

2. Create Service Account for Secrets Provider 

3. Create Role / ClusterRole for the Service Accounts with **get**/**update** privileges

4. Create RoleBinding / ClusterRoleBinding and bind the Service Accounts to the role from previous step

5. Create/Deploy Secrets Providers as Jobs in deployment manifest(s) ***(Milestone 1)***
   - If label filtering is defined, add "K8S_SECRET_LABEL" in the Secrets Provider manifest(s) ***(Milestone 1)***



#### Customer Experience

In order to preserve restrictions on Conjur secrets, a customer can decide to either:

1. Deploy 1 Secrets Provider per namespace, giving the Secrets Provider access to all secrets
2. Deploy 1 Secrets Provider per app / (group of apps), giving the Secrets Provider only access to the secrets they have been given access to in Conjur. Should the user require further separation of duties on Conjur secrets, they will need to deploy another Secrets Provider

The Secrets Provider host will resemble the following:

```yaml
- !host
  id: secrets-provider-1
  annotations:
    authn-k8s/namespace: prod-namespace
    authn-k8s/service-account: prod-sa
    authenticator-container-name: cyberark-secrets-provider
 
- !permit
  resources: !variable super_secret
  role: !layer secrets
  privileges: [ read, execute ]
 
- !grant
  role: !layer secrets
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

// Rest of environment variables for Pod to run 
```



#### Code changes

1. Support Job application identity 
2. Enhance Batch retrieval

*Support Job application identity*

To authenticate to Conjur, the Secrets Provider application identity (or Resource Restrictions) will be the characteristics of the Pod that it is running in. Currently, we do not have a "Job" application identity granularity so to allow the Secrets Provider the ability authenticate to Conjur, the customer will need to choose between the Namespace or Service Account granularities. If there is a requirement to support a "Job" granularity this can be done by adding code to the authenticator in Conjur.

*Enhance Batch retrieval*

At current, the Secrets Provider code looks at the "K8S_SECRET" environment variable in each of the Pods and preforms a batch retrieval against the Conjur API to get the relevant Conjur Secrets. If there is a failure in retrieving one of the secrets, the request fails and no secrets are returned to the request Pod.

This is problematic for us at this stage because now the Secrets Provider is sitting separately and will need to perform a batch retrieval of all the secrets in the namespace. In other words, if the Secrets Provider fails to fetch one Conjur secret, the whole request will fail and applications will not get their secrets. To avoid this, we will update the code so that if a failure takes place, it will be noted in the logs and a new request will be made, skipping over the Conjur secret that caused the error.

```pseudocode
for each k8s-secret
  variable-ids = k8s-secret.parseConjurMap()
  GET /secrets{?variable-ids}
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
    conjur-secret: env-prod # label name must be 'conjur-secret'; value is supplied by the customer
  type: Opaque
stringData:
  conjur-map: |-
    username: secrets/my_username
```

These labels do not have to be Conjur-specific and the customer would decide which label they would like to put in the K8s secrets. They would define which secrets to iterate over by "K8S_SECRET_LABEL" env variable in their Secrets Provider manifest. The Secrets Provider will perform a search for secrets with that label against the [K8s](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#secret-v1-core)/[OC](https://docs.openshift.com/container-platform/4.4/rest_api/index.html#secret-v1-core) API and search for the "conjur_map" on the results.

Note that just because the customer added a label for us to filter over doesn't mean that all secret entries in the K8s Secret are stored in Conjur but it gives us a way to perform a more focused search for "conjur-map" in the K8s Secrets.

Secrets Provider Manifest with K8S_SECRET_LABEL environment variable

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

These labels will have an OR relationship not an AND relationship. In other words, a K8s Secret can either have a CONJUR label or PROD label. A customer can also write '*' to imply all K8s secrets in the namespace. If 'K8S_SECRET_LABELS' is not supplied, by default, the Secrets Provider will fetch all K8s Secrets in the namespace.

Decision: **Solution #1, Read all K8s Secrets that are marked with labels**

We decided against listing K8s Secrets as configuration in the ENV of the Secrets Provider manifest because that is not a scalable solution, especially for customers with hundreds of K8s Secrets. Customers might already use labels for grouping related K8s Secrets so we will benefit by integrating more with a customer environment and utilizing that.



### Milestone 2: Update K8s secret with Conjur secret

#### Design

Once a secret has been updated in Conjur, this will trigger an update in K8s Secrets values. For Milestone 2, we are building ontop of Milestone 1, **not** in-place of.  The Secrets Provider deployment will need to be a Pod that does not terminate (instead of a Job). Every change in Conjur secret, will trigger an event to update the K8s Secret. 

Note that we are not introducing a breaking change between Milestones. If a customer does not want to upgrade to Milestone 2, they are not required to. If they do, the image they will use will be the same one used in Milestone 1.

|      | Trigger for Secret Rotation                                  | Elaboration                                                  | Pros                                                         | Cons                                                         |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 1    | *Every 5 minutes,* retrieve secrets every constant, configurable amount of time. Possible options:<br /><br />a. Run constantly and wake up when time is right <br />b. Spin-up every 5 minutes, run using [cron job](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/) based on K8s secrets update (Get trigger upon secret change) | *Flow*<br />1. Every 5 minutes, Secrets Provider checks if a change/a new K8s Secret was created <br />2. If so, update Secrets Provider local cache of state for K8s | - Easily integrate with current solution since retry mechanism already exists<br />- No need to change anything in Conjur | -  Polling is a bad way for changes notification mechanism.<br />- Option (a) is inefficient management of resources.<br />- Option (b) is supported only since K8s version 1.18 and is currently in beta. |
| 2    | *Outside trigger,* register to 3rd party pub/sub message handler to be notified of secrets change.<br />Ex: <br />- ZMQ (no agent)<br />- RabbitMQ |                                                              | - Do the job only when really needed - very efficient<br />- Immediate update of K8s Secret on secret change in Conjur<br />- Integrated option for registration only on relevant changes | - Need development in Conjur side to update the 3rd party component with changes<br />- Requires another component to deploy and maintain (if not ZMQ) |
| 3    | *Outside trigger,* retrieve secrets when notified from Follower by webhook registered API | *Concern*<br />Follower does not save state and cannot save the webhook URL | - Efficient because only do the job when really needed<br />- Immediate update of K8s Secret on secret change in Conjur | - Need development in Conjur side to support notification mechanism to notify only necessary secrets providers<br />- Follower may not be able to reach secrets provider (need to save URL in Master Database)<br />- Significant development required because it requires a new event mechanism on server |
| 4    | *Outside trigger,* retrieve secrets when notified by websocket connection | Websocket connection is opened and when there is a change on the server, it will send update | - Efficient because only do the job when really needed<br />- Immediate update of K8s Secret on secret change in Conjur | - Need development in Conjur side to support notification mechanism to notify only necessary secrets providers<br />- Very expensive to hold a socket open just for few rare notifications<br />- Significant development required because it requires a new event mechanism on server |

Decision: **Solution #2, Outside trigger using pub/sub message
**

The Pub/Sub pattern is a pattern where senders publish messages without knowing who is subscribing to those messages. Once an event is emitted or a message is published, all receivers that subscribe to that event will get a notification. For our purposes, we propose to use a 3rd party pub/sub message handlers, ZMQ, the publisher, that will trigger an event whenever a Conjur secret has been updated. The Secrets Provider, the subscriber, will listen on a port for these events/messages. Once a message is received this will prompt the Secrets Provider to update the K8s Secrets with the updated Conjur Secrets.

We chose to use the pub/sub pattern over waiting for a configurable amount of time because this delivers a nicer customer experience. The customer's K8s Secrets will be updated immediately which in turn avoids situations where customer applications fail to connect to their endpoints. 

#### Flow

1. Configure K8s Secrets with ***conjur-map\*** metadata defined 

   - *Optional:* add label to K8s Secret 

2. Create Service Account for Secrets Provider 

3. Create Role / ClusterRole for the Service Accounts with **get**/**update** privileges

4. Create RoleBinding / ClusterRoleBinding and bind the Service Accounts to the role from previous step

5. Create/Deploy Secrets Providers as Pods in deployment manifest(s) **(\*Milestone 2)\***

6. - If label filtering is defined, add "K8S_SECRET_LABEL" in the Secrets Provider manifest(s)

7. When the Secrets Provider receives a message that Conjur secrets have changed, the process of fetching and updating the K8s Secret(s) repeats



#### Customer experience

Because the Secrets Provider needs to now update K8s Secrets with Conjur Secrets continuously, it can no longer run as a Job and will require that the customer deploy the Secrets Provider as a Pod (kind: Pod instead of kind: Job)

```yaml
apiVersion: app/v1
kind: Pod
metadata:
  name: secrets-provider
spec:
  template:
    spec:
      serviceAccountName: prod-sa
      containers:
      - image: secrets-provider:latest
        name: cyberark-secrets-provider-1
....
```



#### Code changes

Add publishing mechanism to Conjur server. Work TBD.



### Order of Deployment

##### Milestone 1 and 2 

To ensure that on first run our Secrets Provider runs first, we will request from the customer follow the following setup order:

1. Add all necessary K8s Secrets and their labels to the Secrets Provider manifest

2. Run the Secrets Provider

3. Run application pods

   

### Lifecycle/Deletion

##### Milestone 1

The lifecycle of the Job is the length of the process. In other words, the time it takes to retrieve the Conjur secrets value and update them in the K8s Secrets. The customer can configure the Job to stay "alive" by adding **ttlSecondsAfterFinished**, with the desired about of seconds, to their Secrets Provider yaml. Because of the nature of a Job, a manual delete is not necessary.

##### Milestone 2

Because we are upgrading to a Pod that does not terminate upon task completion in Milestone 2, if the customer wants to stop the Secrets Provider process, they will need to delete the pod.

API endpoint for deleting Pod:

```yaml
delete /api/v1/namespaces/{namespace}/pods/{name}
```

### Backwards compatibility

Milestone 2 will be built ontop of Milestone 1, **not** in-place of. If a customer wants to stay with the Milestone 1 capability they can do so. If a customer wants to upgrade to Milestone 2 they will be able to use the same image they used for Milestone 1.

### Performance
##### Milestone 1 and 2 

We will test and document how many secrets can be updated in 5 minutes on average where a secret should be either extreme long password or one vault account which is 5 vars username address port password dns

### Affected Components
##### Milestone 1

- Conjur/DAP: Adding support for Job application identity granularity 

##### Milestone 2

- Conjur/DAP: Adding publishing messaging ability upon Conjur secret change

## Security
[//]: # "Are there any security issues with your solution? Even if you mentioned them somewhere in the doc it may be convenient for the security architect review to have them centralized here"

## Test Plan
[//]: # "Fill in the table below to depict the tests that should run to validate your solution"
[//]: # "You can use this tool to generate a table - https://www.tablesgenerator.com/markdown_tables#"

| **Title** | **Given** | **When** | **Then** | **Comment** |
|-----------|-----------|----------|----------|-------------|
|           |           |          |          |             |
|           |           |          |          |             |

## Logs
[//]: # "Fill in the table below to depict the log messages that can enhance the supportability of your solution"
[//]: # "You can use this tool to generate a table - https://www.tablesgenerator.com/markdown_tables#"

| **Scenario** | **Log message** |
|--------------|-----------------|
|              |                 |
|              |                 |

### Audit 
[//]: # "Does this solution require additional audit messages?"

## Documentation
[//]: # "Add notes on what should be documented in this solution. Elaborate on where this should be documented. If the change is in open-source projects, we may need to update the docs in github too. If it's in Conjur and/or DAP mention which products are affected by it"

## Open questions
[//]: # "Add any question that is still open. It makes it easier for the reader to have the open questions accumulated here istead of them being acattered along the doc"

## Implementation plan
[//]: # "Break the solution into tasks"
