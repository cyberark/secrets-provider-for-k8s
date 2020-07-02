# Secrets Provider - Phase 2

## Table of Contents
## Glossary

| **Term**                                                     | **Description**                                              |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) | K8s entity that ensures multiple replications of a pod are running all the time. If a pod terminates, another one will be created.<br />In OpenShift it is called **DeplonmentConfig**. |

## Useful links

| Name                | Link                                                         |
| ------------------- | ------------------------------------------------------------ |
| Phase 2 Feature Doc | https://github.com/cyberark/secrets-provider-for-k8s/issues/102 |

## Background

The Cyberark Secrets Provider for K8s enables customers to use secrets stored and managed in Conjur/DAP vault and consume them as K8s Secrets for their application containers.

During Phase 1, an *init container* was deployed in the same Pod as the customer's applications to populate the K8s secrets defined in the pod application's manifest.

### Motivation

Secrets Provider as *init container* only supports one application and makes the deployment of the Secrets Provider deeply coupled to the customer's applications.
Also, in case a secret was rotated while the application was running it won't get the new secret.

So another solution should be created to support multiple applications, and support rotation.

### Requirements

The solution should support the following capabilities:

[//]: #	"TODO fill requirements"

## Solution
To support many applications, the solution is to run secrets provider using **Deployment** (**DeploymentConfig** in OpenShift).

To support rotation, the secrets provider will listen on events from Conjur saying any of the relevant secrets is rotated.

### Design

Run Secrets Provider as a **Deployment** (**DeploymentConfig** in OpenShift).
Once the Deployment's pod is spun up, it will authenticate to Conjur/DAP via authn-k8s.
It will then fetch all the relevant K8s secrets and update them with the Conjur secrets they require.
Then, it will connect to pub/sub channel in Conjur and subscribe to rotation events on the relevant secrets.

#### How does a it answer the requirements?

Running as a **Deployment** allows separation from the applications' deployments and serve multiple applications at once.

Subscribing to rotation events on the relevant secrets allows it to trigger secrets retrieval and update them in the relevant k8s secrets.

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

[ // ]: # "TODO:elaborate"

#### Retrieve secrets from Conjur for each K8s secret separately

### Order of Deployment

Steps to follow for successful deployment

1. Add all necessary K8s Secrets to the Secrets Provider manifest
2. Run the Secrets Provider Deployment (*)
3. Run application pods (*)

(*) The order at which the Secrets Provider and application pod are deployed does not matter because the [application pod will not start](https://kubernetes.io/docs/concepts/configuration/secret/#details) until it has received the keys from the K8s Secrets it references (secretKeyRef).

### Lifecycle/Deletion

[ // ]: # "TODO: fill"

### Backwards compatibility

Milestone 1 of Phase 2 will be built ontop of the current init container solution. We will deliver the same image and will not break current functionality.



### Performance
*See [performance tests](#Performance Tests).* 

We will test and document how many K8s Secrets can be updated in 5 minutes on average. A secret should be either extreme long password.

### Affected Components

- Conjur - Add events mechanism

## Security

[ // ]: # "TODO: fill"

## Test Plan

[ // ]: # "TODO: update/change/add relevant tests and logs"

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

### Performance Tests

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

[ // ]: # "TODO: add events audit"

## Documentation
[//]: # "Add notes on what should be documented in this solution. Elaborate on where this should be documented. If the change is in open-source projects, we may need to update the docs in github too. If it's in Conjur and/or DAP mention which products are affected by it"

## Open questions
[//]: # "Add any question that is still open. It makes it easier for the reader to have the open questions accumulated here istead of them being acattered along the doc"

## Implementation plan
### Delivery plan

[ // ]: # "TODO: update"

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
- [ ] Documentation has been given to TW + approved ***(~2 days)***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(~1 day)\***
- [ ] Create demo for Milestone 1 functionality ***(~1 day)\***
- [ ] Versions are bumped in all relevant projects (if necessary) ***(~1 days)\***

**Total:** ~30 days of non-parallel work **(~6 weeks)**

*Risks that could delay project completion*

1. New language (Golang), new testing (GoConvey), and new platform (K8S/OC)
2. Cross-project work/dependency
   - Some changes may involve changes in authn-client
   - Mitigation: explore / outline impact our changes will have on dependent projects and raise them early