# Solution Design - Template

[//]: # "Change the title above from "Template" to your design's title"

## Table of Contents

[//]: # "You can use this tool to generate a TOC - https://ecotrust-canada.github.io/markdown-toc/"

## Glossary

| **Term**                                                     | **Description**                                              |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [Job](*https://kubernetes.io/docs/concepts/workloads/controllers/job/*) | K8s entity that runs a pod for one-time operation. Once the pod exists, the Job terminates |
| [Deployment](*https://kubernetes.io/docs/concepts/workloads/controllers/deployment/*) | K8s entity that ensures multiple replications of a pod are running all the time. If a pod terminates, another one will be created. |
| [Kubernetes Helm](https://helm.sh/)                          | A tool that streamlines and organizes the management of Kubernetes installation, deployment, and upgrade processes. |

## Useful links

| Name                | Link                                                         |
| ------------------- | ------------------------------------------------------------ |
| Phase 2 Feature Doc | https://github.com/cyberark/secrets-provider-for-k8s/issues/102 |

## Background

The Cyberark Secrets Provider for K8s enables customers to use secrets stored and managed in Conjur/DAP vault and consume them as K8s Secrets for their application containers.

During Phase 1, an *init container* was deployed in the same Pod as the customer's applications to populate the K8s secrets defined in the pod application's manifest.

### Motivation

The current implementation of the Secrets Provider only supports one application and makes the deployment of the Secrets Provider deeply coupled to the customer's applications.



## Requirements

The solution should support the following capabilities:

####  Milestone 1 *(current)*

- Secrets Provider runs as a separate entity, serving multiple application containers that run on multiple pods
- Solution needs to be native to Kubernetes
- Lifecycle should support update/removal of deployment
- Provide a way for customer to understand the state of the Secret Provider - when it finished initializing

###  Milestone 2

- Secrets Provider support secret rotation

For seamless shift between milestones, we will deploy using Helm.

## Solution

The solution is to allow different deployments and behave differently based on the use-case chosen.

The following decision tree depicts customer uses-cases for Secrets Provider deployments:

 ![Secrets Provider flavors decision flow chart](https://user-images.githubusercontent.com/31179631/85747023-975bf500-b70f-11ea-8e26-1134068fe655.png) 

These variations will be supported using the same Secrets Provider image, but the behavior will vary dynamically depending on the chosen deployment method that is configured via Helm Chart.

*In this document we will focus on **Milestone 1**.*

### Milestone 1: Serve K8s secrets to multiple applications once (no rotation)

### Design

Configure the Secrets Provider via **Helm** as a **Job**.
Once the Job's pod is spun up, it will authenticate to Conjur/DAP via authn-k8s.
It will then fetch all the K8s secrets update them with the Conjur secrets they require and terminate upon completion.

Kubernetes Helm is a tool that streamlines and organizes the management of Kubernetes installation, deployment, and upgrade processes. That way, we can provide our customer's a one-click solution even though the way at which we deploy changes.

#### How does a Job answer the requirements?

Running as a **Job** allows separation from the applications' deployment and serve multiple applications at once. 

Because for this Milestone we are concerned with updating K8s Secrets once at intial spin up, a **Job** is the most native solution. It will terminate upon task completion and not waste customer's resources.

TODO: See upgrade section for a detailed explanation of how we will provide a seemlessly solution between Milestones when the requirements change.

#### What drawbacks does this solution have?

**Job** is not an existing application identity granularity.

 As a result, one can define the host representing the Secrets Provider in Conjur policy using only **namespace** and/or **service account** granularities.

To handle the missing **Job** granularity there are 2 options:

|      | Solution                                                     | Pros                                                         | Cons                                                         | Effort Estimation |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------- |
| 1    | Keep using only **namespace** and/or **service account** granularities. | \- Since Secrets Provider runs as a separate entity, we suggest creating a dedicated service account for Secrets Provider.<br />That way, the **service account** granularity serves as an exact match and **Job** granularity is redundant<br />- No code changes | \- Service account might be reused, which allows other apps to authenticate using Secrets Provider host<br />- Break application identity convention | Free              |
| 2    | Add support for **Job** granularity in Conjur                | Allows specific identification of the Secrets Provider **Job**, which enhances security | \- Costly; requires code changes, verify compatibility, tests, docs<br />- Redundant if **service account** serves only the Secrets Provider | 10 days           |

*Decision*: Solution #1, use **namespace** and **service account** to define the Secrets Provider host in Conjur policy. We would recommend to the customer to create a dedicated service account for Secrets Provider.

#### Customer experience

Deployment for the Secrets Provider will be done using Helm. That way, if the 

1. Configure K8s Secrets with ***conjur-map\*** metadata defined

2. Create `values.yaml` that will contain the following parameters and their defaults:

   2.1. Service Account for Secrets Provider (default: `secrets-provider-account`)

   2.2. Role (default: `secrets-provider-role`)

   2.3. RoleBinding Name (default: `secrets-provider-role-binding`)

   2.4. K8S_SECRET (default: empty list)

   *When parameters do not have are not given values, their defaults will be used*

3. Install Helm Chart for the Secrets Provider

We will have a `values.yaml` file where all default variables for configurable parameters are collected. Our `values.yml` will resemble the following:

```
# K8s manifest defaults
serviceAccountName: secrets-provider-account
image: cyberark/secrets-provider-for-k8s
name: cyberark-secrets-provider

# K8s permissioning defaults
Role: secrets-provider-role
Rolebinding: secrets-provider-role-binding

# Conjur defaults
CONJUR_APPLIANCE_URL: TBD
CONJUR_AUTHN_URL: TBD
CONJUR_ACCOUNT: TBD
CONJUR_SSL_CERTIFICATE: TBD
DEBUG=true
CONJUR_AUTHN_LOGIN: TBD
K8S_SECRETS=[]
CONTAINER_MODE=init
SECRETS_DESTINATION=k8s_secrets
```



These variables can be applied for our project as follows:

`templates/secrets-access-role.yml`

*TBD*

`templates/secrets-access-role-binding.yml`

*TBD*

`templates/secrets-provider.yml`

*TBD*



### Milestone 2: Rotation

*TBD*



### Order of Deployment

#### Milestone 1

Steps to follow for successful deployment

1. Add all necessary K8s Secrets to the Secrets Provider manifest
2. Run the Secrets Provider Deployment (*)
3. Run application pods (*)

 (*) The order at which the Secrets Provider and application pod are deployed does not matter because the [application pod will not start](https://kubernetes.io/docs/concepts/configuration/secret/#details) until it has received the keys from the K8s Secrets it references (secretKeyRef).

### Lifecycle, update, and deletion

#### Milestone 1

##### Lifecycle

The lifecycle of the Secrets Provider is independent of the application pods. The application pods that detail K8s Secrets in their environment [will not start](*https://kubernetes.io/docs/concepts/configuration/secret/#details*) until the K8s Secrets they require are populated with the expected key. 

 To notify the customer that the Secrets Provider has started and finished it's work, the process will be logged.

##### Update

Because we will be using Helm Charts, we can easily supply customers with the Helm Charts for the current Milestone. As we progress and release new versions for the project, we can supply the customer with the necessary Helm Charts. All they would need to do is switch out their current Chart for the new one.

##### Delete

The lifecycle of the Job is the length of the process. In other words, the time it takes to retrieve the Conjur secrets value and update them in the K8s Secrets. The customer can configure the Job to stay "alive" by adding **ttlSecondsAfterFinished**, with the desired about of seconds, to their Secrets Provider yaml. Because of the nature of a Job, a manual delete is not necessary.

### Backwards compatibility

#### Milestone 1

Milestone 1 of Phase 2 will be built ontop of the current init container solution. We will deliver the same image and will not break current functionality.

### Performance

#### Milestone 1

TODO: See performance tests

 We will test and document how many K8s Secrets can be updated in 5 minutes on average. A secret should be either extreme long password.

### Affected Components

## Security

#### Security boundary

##### Kubernetes/Openshift security boundary

The security boundary is the namespace at which the Secrets Provider and application pods run. The namespace provides an isolation for access control restrictions and network policies. The interaction between accounts and K8s resources is limited by the Service Account that runs in the namespace.

##### Conjur security boundary

The security boundary of the Secrets Provider is the Host identity it uses to authenticate to Conjur via the Conjur Kubernetes Authenticator Client. For full details on how the authentication process works, [please visit](*https://github.com/cyberark/conjur-authn-k8s-client*).

#### Controls

The value for *`stringData`* in the K8s Secret resource is a String of user input values. To guarantee that this field is not manipulated for malicious purposes, we are validating this input.

## Test Plan

### Integration tests

| **Title** | **Given**                                                    | **When**                                                     | **Then**                                      | **Comment**                                                  |
| --------- | ------------------------------------------------------------ | ------------------------------------------------------------ | --------------------------------------------- | ------------------------------------------------------------ |
| 1         | *Vanilla flow*, Secret Provider Job successfully updates K8s Secrets | - Conjur is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and K8s Secrets are configured<br />- Service Account has correct permissions (get/update/list) <br />- Secrets Provider Job manifest is defined<br />- `K8S_SECRETS_LABELS` (or `K8S_SECRETS`) env variable is configured | Secrets Provider runs as a Job                | - Secrets Provider pod authenticates and fetches Conjur secrets successfully<br />- All K8s Secrets are updated with  Conjur value <br />- App pods receive K8s Secret with Conjur secret values as environment variable<br />- Secrets Provider Job terminates on completion of task<br />- Verify logs |
| 2         | *Vanilla flow*, non-conflicting Secrets Provider<br />       | Two Secrets Providers have access to different Conjur secret | 2 Secrets Provider Jobs run in same namespace | - All relevant K8s secrets are updated<br />- Verify logs    |
| 3         | Non-conflicting Secrets Provider 1 namespace same K8s Secret * | Two Secrets Providers have access to same secret             | 2 Secrets Provider Jobs run in same namespace | - No race condition and Secrets Providers will not override each other<br />- Verify logs |
| 4         | *Regression tests*                                           | All regression tests should pass                             |                                               |                                                              |

### Performance tests

| Title | Given                     | When                                                         | Then                           | Comment                                                      |
| ----- | ------------------------- | ------------------------------------------------------------ | ------------------------------ | ------------------------------------------------------------ |
| 1     | Performance investigation | - 1000 Secrets Providers defined<br />- Conjur secrets with max amount of characters | Secrets Provider runs as a Job | Evaluate how many K8s Secrets can be updated with Conjur secrets in 5 minutes |
| 2     | Performance investigation | - 1000 Secrets Providers defined<br />- Conjur secret with average amount of characters | Secrets Provider runs as a Job | Evaluate how many K8s Secrets can be updated with Conjur secrets in 5 minutes |



## Logs

| **Scenario**                         | **Log message**                                              |
| ------------------------------------ | ------------------------------------------------------------ |
| Secrets Provider spins up            | Kubernetes Secrets Provider v\*%s\* starting up as a %s... (Job/Deployment) |
| Job has completed and is terminating | Kubernetes Secrets Provider has finished task and is terminating… |

### Audit 

All fetches on a Conjur Resource are individually audited, creating its own audit entry. Therefore there will be no changes in audit behavior for this Milestone.

## Documentation



## Open questions



## Implementation plan

#### Delivery Plan (Milestone 1)

- [ ] Solution design approval + Security review approval
- [ ] Implement Phase 2 Milestone 1 functionality ***(~2 days)\***
- [ ] Implement test plan (Integration + Unit + Performance tests align with SLA) ***(~4 days)\***
- [ ] Security items have been taken care of (if they exist) ***(TBD)\***
- [ ] Logs review by TW + PO ***(~1 day)\*** 
- [ ] Documentation has been given to TW + approved ***(~2 days)\***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(~1 day)\***
- [ ] Create demo for Milestone 1 functionality ***(~1 day)\***
- [ ] Versions are bumped in all relevant projects (if necessary) ***(~1 days)\***

 **Total:** ~10 days of non-parallel work **(~2 weeks)**
