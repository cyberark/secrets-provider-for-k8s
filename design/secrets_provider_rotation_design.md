# Secrets Provider - Phase 2

## Table of Contents
## Glossary

| **Term**                                                     | **Description**                                              |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) | K8s entity that ensures multiple replications of a pod are running all the time. If a pod terminates, another one will be created.<br />In OpenShift it is called **DeploymentConfig**. |

## Useful links

| Name                | Link                                                         |
| ------------------- | ------------------------------------------------------------ |
| Phase 2 Feature Doc | https://github.com/cyberark/secrets-provider-for-k8s/issues/102 |

## Background

The Cyberark Secrets Provider for K8s enables customers to use secrets stored and managed in Conjur/DAP vault and consume them as K8s Secrets for their application containers.

During Phase 1, an *init container* was deployed in the same Pod as the customer's applications to populate the K8s secrets defined in the pod application's manifest.

### Motivation

Secrets Provider as an *init container* only supports one application pod, making deployment of the Secrets Provider deeply coupled to the customer's applications.

Due to the nature of an init container, once a K8s secret value is rotated while the application is running it will not get the new secret. Because of this, another solution should be created to support this use; support multiple applications and secrets rotation.

### Requirements

The solution should support the following capabilities:

Milestone 1 *(current)*

- Secrets Provider runs as a separate entity, serving multiple application containers that run on multiple pods
- Solution needs to be native to Kubernetes and stay up, idle even after process completion
- Lifecycle should support update/removal of deployment
- Provide a way for customer to understand the state of the Secret Provider - when it finished initializing

Milestone 2

- Secrets Provider support secret rotation

Even though we are currently on Milestone 1 we need to provide a seamless solution, without customer intervention between the configuration of both Milestones. Therefore, we will also address how we plan to implement rotation (Milestone 2) at a high-level as part of this document.

## Solution
To support multiple application pods, the Secrets Provider is to run using **Deployment** (**DeploymentConfig** in OpenShift) until the customer .

To support rotation, the Secrets Provider will listen on Conjur secret rotation events from Conjur.

### Design

Run Secrets Provider as a **Deployment** (**DeploymentConfig** in OpenShift).
Once the Deployment's Secrets Provider pod is spun up, it will authenticate to Conjur/DAP via authn-k8s.
It will then fetch all the relevant K8s Secrets and update them with the Conjur secrets they require.
Then, the Secrets Provider will connect to Pub/Sub channel in Conjur and subscribe to rotation events on the relevant secrets.

#### How does a it answer the requirements?

Running as a **Deployment** allows separation from the applications' deployments and serve multiple application pods at once.

Subscribing to Conjur secret rotation events on the relevant secrets allows it to trigger secrets retrieval and update them in the relevant K8s secrets.

#### What benefits does this solution provide?

1. **Clarity**
   Allowing customers to select the deployment to get the desired value
2. **Maintainability**
   Using one image for all variations simplifies code maintenance 
3. **Reusability**
   Using one image for all variations allows reuse of existing code across all variations
4. **Extensibility**
   Supporting various flows using the same solution opens the door for more secrets sources and targets
5. **Efficiency**
   Subscribing to rotation events allows better resource management and prevent unnecesary code execution.
6. **Performance**
   Subscribing to rotation events ensures rotated secrets will be updated very close to rotation occurrence.

#### What drawbacks does this solution have?

1. Currently there's no events mechanism in Conjur, so it needs to be developed as part of this solution.
2. Current solution does batch retrieval for all Conjur secrets and do not update any K8s secret if the batch does not succeed.
   When serving multiple apps this behavior will block apps from loading due to irrelevant issue in secrets of another app.
   Failure examples can be insufficient rights for the secret in Conjur, secret does not exist or secret does not have a value.

#### How the drawbacks can be handled?

1. [Create events mechanism](#create-events-mechanism)

2. [Retrieve secrets from Conjur for each K8s secret separately](#retrieve-secrets-from-conjur-for-each-k8s-secret-separately)

### Customer Experience

### Code changes

#### Create events mechanism

To create the events mechanism in Conjur there are 3 options:

|      | Solution | Pros | Cons | Effort Estimation |
| ---- | -------- | ---- | ---- | ----------------- |
| 1    |          |      |      |                   |
| 2    |          |      |      |                   |
| 3    |          |      |      |                   |

*Decision*: Solution #1, use ZeroMQ.

[ // ]: # "TODO:elaborate and define ZeroMQ"

#### Retrieve secrets from Conjur for each K8s secret separately

### Order of Deployment

Steps to follow for successful deployment

1. Add all necessary K8s Secrets to the Secrets Provider manifest
2. Run the Secrets Provider Deployment (*)
3. Run application pods (*)

(*) The order at which the Secrets Provider and application pod are deployed does not matter because the [application pod will not start](https://kubernetes.io/docs/concepts/configuration/secret/#details) until it has received the keys from the K8s Secrets it references (secretKeyRef).

### Lifecycle/Deletion

*Lifecycle*

The lifecycle of the Secrets Provider is independent of the application pods. The application pods that detail K8s Secrets in their environment [will not start](https://kubernetes.io/docs/concepts/configuration/secret/#details) until the K8s Secrets they require are populated with the expected key. 

To notify the customer that the Secrets Provider has started and finished it's work, the process will be logged.

*Deletion*

For this Milestone, after the inital fetching and updating of K8s Secrets' values, the Secrets Provider will sit idle. To delete the deployment, the customer would have to manually delete it. 

```
oc delete deploymentConfig <name of deploymentConfig>
```

### Backwards compatibility

Milestone 1 of Phase 2 will be built ontop of the current init container solution. We will deliver the same image and will not break current functionality.

### Performance
*See [performance tests](#Performance Tests).* 

We will test and document how many K8s Secrets can be updated in 5 minutes on average. A secret should be either extreme long password.

### Affected Components

*Milestone 2*

- Conjur - Add Pub/Sub events mechanism

## Security

#### Security boundary

##### Kubernetes/Openshift security boundary

The security boundary is the namespace at which the Secrets Provider and application pods run. The namespace provides an isolation for access control restrictions and network policies. The interaction between accounts and K8s resources is limited by the Service Account that runs in the namespace.

##### Conjur security boundary

The security boundary of the Secrets Provider is the Host identity it uses to authenticate to Conjur via the Conjur Kubernetes Authenticator Client. For full details on how the authentication process works, [please visit](https://github.com/cyberark/conjur-authn-k8s-client).

#### Controls

The value for `stringData`  in the K8s Secret resource is a String of user input values. To guarantee that this field is not manipulated for malicious purposes, we are validating this input.

## Test Plan

### Integration tests (Milestone 1)

|      | **Title**                                                    | **Given**                                                    | **When**                                                     | **Then**                                                     | **Comment**                                |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------ |
| 1    | *Vanilla flow*, Secret Provider  successfully updates K8s Secrets | - Conjur is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and K8s Secrets are configured<br />- Service Account has correct permissions (get/update/list) <br />- Secrets Provider Job manifest is defined<br />-  `K8S_SECRETS` env variable is configured | Deployment spins up the Secrets Provider                     | - Secrets Provider pod authenticates and fetches Conjur secrets successfully<br />- All K8s Secrets defined in `K8S_SECRETS`are updated with  Conjur value <br />- App pods receive K8s Secret with Conjur secret values as environment variable<br />- Secrets Provider stays up<br />- Verify logs |                                            |
| 2    | Batch retrieval failure                                      | Secrets provider and SA has been properly configured         | Secrets Provider is running<br />- Host doesn't have permissions on Conjur secret | - Failure to fetch *specific* secret without harming batch retrieval for rest of the secrets <br />- Failure is logged defining which Conjur secret(s) failed |                                            |
| 3    | *Vanilla flow*, non-conflicting Secrets Provider<br />       | Two Secrets Providers have access to different Conjur secret | 2 Secrets Provider run in same namespace                     | - All relevant K8s secrets are updated<br />- Verify logs    |                                            |
| 4    | Non-conflicting Secrets Provider 1 namespace same K8s Secret | Two Secrets Providers have access to same  secret            | 2 Secrets Provider runs as a Job in same namespace           | - No race condition and Secrets Providers will not  override each other<br />- Verify logs |                                            |
| 5    | *Regression tests*                                           |                                                              |                                                              | All regression tests should pass                             | All init container tests should still pass |

### Integration tests (Milestone 2)

The following tests are for Milestone 2 and *are subject to change.*

|      | **Title**                                                    | **Given**                                                    | **When**                                              | **Then**                                                     | Comment |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------------------------------------------- | ------------------------------------------------------------ | ------- |
| 1    | *Vanilla flow*, Conjur secret is rotated and K8s Secret is updated with new Conjur secret | - Conjur is running<br />- Secrets Provider is running and properly configured<br />- Secrets Provider subscribes to Secret change messaging queue <br /> | - Change in Conjur Secret that K8s pod requires<br /> | - Conjur publishes change to messaging queue<br />- Secrets Provider receives event <br />- Secrets Provider  fetches Conjur secret<br />- K8s Secret is updated with Conjur value<br />- Verify logs |         |

### Unit tests

|      | Given                                | When | Then                                           | Comment                                                    |
| ---- | ------------------------------------ | ---- | ---------------------------------------------- | ---------------------------------------------------------- |
| 1    | Given one K8s secret with conjur-map |      | Validate content of conjur-map (encoded, size) | *Security test* that we are properly handling input values |

### Performance Tests

|      | **Title**                 | **Given**                                                    | **When**                       | **Then**                                                     | **Comment** |
| ---- | ------------------------- | ------------------------------------------------------------ | ------------------------------ | ------------------------------------------------------------ | ----------- |
| 1    | Performance investigation | - 1000 Secrets Providers defined<br />- Conjur secrets with max amount of characters | Secrets Provider runs as a Job | Evaluate how many K8s Secrets can be updated with Conjur secrets in 5 minutes |             |
| 2    | Performance investigation | - 1000 Secrets Providers defined<br />- Conjur secret with average amount of characters | Secrets Provider runs as a Job | Evaluate how many K8s Secrets can be updated with Conjur secrets in 5 minutes |             |

## Logs

*Milestone 1 Logs*

| **Scenario**                            | **Log message**                                              | Type  |
| --------------------------------------- | ------------------------------------------------------------ | ----- |
| Secrets Provider pod spins up           | Kubernetes Secrets Provider v\*%s\* starting up... (Already exists) | Info  |
| Pod has completed                       | Kubernetes Secrets Provider has process completed…           | Info  |
| Batch retrieval failure (404 Not Found) | Failed to retrieve Conjur secrets. Reason: 404 Not Found. Variable `secret/test_secret`. Proceeding to next secret... | Error |
| Batch retrieval failure (403 Forbidden) | Failed to retrieve Conjur secrets. Reason: 403 Forbidden. Proceeding to next secret... | Error |

*Milestone 2 Logs*

| **Scenario**                                                 | **Log message**                                              | Type  | Where            |
| ------------------------------------------------------------ | ------------------------------------------------------------ | ----- | ---------------- |
| Secrets Provider subscribes to message queue                 | Subscribing to secrets updates on k8s secrets [test-secret, test-secret-2]... | Info  | Secret Provider  |
| Secrets Providers attempt to subscribe to message queue is successful | Subscribing to secrets updates successful...                 | Info  | Secret Provider  |
| Secrets Providers attempt to subscribe to message queue is unsuccessful | Subscribing to secrets updates unsuccessful...               | Error | Secret Provider  |
| Secrets Provider has subscribed and is listening for Conjur secrets updates | Listening for secrets updates on k8s secrets [test-secret, test-secret-2]... | Info  | Secrets Provider |
| Received Secret rotation event from Conjur                   | Received Conjur secret rotation event. Updating k8s secrets [test-secret, test-secret-2]... | Info  | Secrets Provider |
| Conjur publishes Conjur secret change                        | Conjur secrets`[secrets/test-secret,secrets/test-secret2]` have been rotated. Publishing Conjur secrets... | Info  | Conjur           |

### Audit 

All fetches on a Conjur Resource are individually audited, creating its own audit entry. Therefore there will be no changes in audit behavior for this Milestone.

## Documentation
[//]: # "Add notes on what should be documented in this solution. Elaborate on where this should be documented. If the change is in open-source projects, we may need to update the docs in github too. If it's in Conjur and/or DAP mention which products are affected by it"

## Open questions
1. Do we need to add audit entries for all Pub/Sub tasks ("Connection made", "Subscribed", "Received Event")?
2. Does the solution need to include K8s Secret monitoring activity?

## Implementation plan
### Delivery plan

- [ ] Solution design approval + Security review approval
- [ ] Golang Ramp-up ***(~3 days)\***
  - [ ] Get familiar with Secrets Provider code and learn Go and Go Testing
- [ ] Create dev environment ***(~5 days)\***
  - [ ] Research + Implement HELM
- [ ] Implement Phase 2 Milestone 1 functionality ***(~10 days)\***
  - [ ] Update batch retrieval
  - [ ] Add trigger and pub/sub messaging
- [ ] Implement test plan (Integration + Unit + Performance tests align with SLA) ***(~7 days)\***
- [ ] Security items have been taken care of (if they exist) ***(TBD)\***
- [ ] Logs review by TW + PO ***(~1 day)\*** 
- [ ] Documentation has been given to TW + approved ***(~2 days)***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(~1 day)\***
- [ ] Create demo for Milestone 1 functionality ***(~1 day)\***
- [ ] Versions are bumped in all relevant projects (if necessary) ***(~1 days)\***

**Total:** ~ days of non-parallel work **(~ weeks)**

*Risks that could delay project completion*

1. New language (Golang), new testing (GoConvey), and new platform (K8S/OC)
2. Cross-project work/dependency
   - Some changes may involve changes in authn-client
   - Mitigation: explore / outline impact our changes will have on dependent projects and raise them early