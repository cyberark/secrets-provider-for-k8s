Secrets Provider Phase 2 Milestone Job - Solution Design

## Table of Contents

* [Glossary](#glossary)

  * [Useful links](#useful-links)
  * [Background](#background)
    + [Motivation](#motivation)
  * [Requirements](#requirements)
    + [Milestone Job *(current)*](#milestone-1---current--)
    + [Milestone Rotation](#milestone-2)
  * [Solution](#solution)
    + [Milestone Job: Serve K8s secrets to multiple applications once (no rotation)](#milestone-1--serve-k8s-secrets-to-multiple-applications-once--no-rotation-)
    + [Design](#design)
      - [How does a Job configured via Helm answer the requirements?](#how-does-a-job-configured-via-helm-answer-the-requirements-)
      - [What drawbacks does this solution have?](#what-drawbacks-does-this-solution-have-)
      - [Customer experience](#customer-experience)
      - [Packaging Helm Charts](#packaging-helm-charts)
    + [Milestone Rotation](#milestone-2--rotation)
    + [Lifecycle](#lifecycle)
      - [Milestone Job](#milestone-1)
        * [Installation](#installation)
        * [Lifecycle](#lifecycle-1)
        * [Update](#update)
        * [Delete](#delete)
    + [Network Disruptions](#network-disruptions)
    + [Order of Deployment](#order-of-deployment)
      - [Milestone Job](#milestone-1-1)
    + [Backwards compatibility](#backwards-compatibility)
      - [Milestone Job](#milestone-1-2)
    + [Performance](#performance)
      - [Milestone Job](#milestone-1-3)
    + [Affected Components](#affected-components)
  * [Security](#security)
  * [Test Plan](#test-plan)
    + [Integration tests](#integration-tests)
    + [Performance tests](#performance-tests)
  * [Logs](#logs)
  * [Documentation](#documentation)
  * [Open questions](#open-questions)
  * [Implementation plan](#implementation-plan)

## Glossary

| **Term**                                                     | **Description**                                              |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| [Job](*https://kubernetes.io/docs/concepts/workloads/controllers/job/*) | K8s entity that runs a pod for one-time operation. Once the pod exists, the Job terminates |
| [Deployment](*https://kubernetes.io/docs/concepts/workloads/controllers/deployment/*) | K8s entity that ensures multiple replications of a pod are running all the time. If a pod terminates, another one will be created. |
| [Kubernetes Helm](https://helm.sh/)                          | A tool that streamlines and organizes the management of Kubernetes installation and deployment processes. |
| [Helm Chart](https://helm.sh/docs/topics/charts/)            | A collection of files that describe a related set of Kubernetes resources. A chart can be used to describe a simple pod of a complex application. |

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

###  Milestone Job *(current)*

- Secrets Provider runs as a separate entity, serving multiple application containers that run on multiple pods
- Solution needs to be native to Kubernetes
- Lifecycle should support update/removal of deployment
- Provide a way for customer to understand the state of the Secret Provider - when it finished initializing

###  Milestone Rotation

- Secrets Provider support secret rotation

For seamless shift between milestones, we will deploy using Helm.

## Solution

The solution is to allow different deployments and behave differently based on the use-case chosen. The following decision tree depicts customer uses-cases for Secrets Provider deployments:

 ![Secrets Provider flavors decision flow chart](https://user-images.githubusercontent.com/31179631/85747023-975bf500-b70f-11ea-8e26-1134068fe655.png) 

These variations will be supported using the same Secrets Provider image, but the behavior will vary dynamically depending on the chosen deployment method that is configured via Helm Chart.

*In this document we will focus on **Milestone Job***, deploying the Secrets Provider as a Job using Helm

### Milestone Job: Serve K8s secrets to multiple applications once (no rotation)

### Design

Customer will install our **Helm Chart** to deploy the Secrets Provider as a **Job**. Once installed, the Job spins up and authenticates to Conjur/DAP via authn-k8s.
It will then fetch all the K8s secrets update them with the Conjur secrets they require and terminate upon completion.

#### How does a Job installed via Helm answer the requirements?

Running as a **Job** allows separation from the applications' deployment and serve multiple applications at once. 

Because for this Milestone we are concerned with updating K8s Secrets once at intial spin up, a **Job** is the most native solution. It will terminate upon task completion and not waste customer's resources.

Kubernetes Helm is a tool that streamlines and organizes the management of Kubernetes installation and deployment processes. When we use Helm for deployment and lifecycle management, we can provide our customer's a one-click solution even though the way at which we deploy changes.

#### What drawbacks does this solution have?

**Job** is not an existing application identity granularity.

As a result, one can define the host representing the Secrets Provider in Conjur policy using only **namespace** with/without **service account** granularities.

To handle the missing **Job** granularity there are 2 options:

|      | Solution                                                     | Pros                                                         | Cons                                                         | Effort Estimation |
| ---- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------- |
| 1    | Keep using only **namespace** and/or **service account** granularities. | \- Since Secrets Provider runs as a separate entity, we suggest creating a dedicated service account for Secrets Provider.<br />That way, the **service account** granularity serves as an exact match and **Job** granularity is redundant<br />- No code changes | \- Service account might be reused, which allows other apps to authenticate using Secrets Provider host<br />- Break application identity convention | Free              |
| 2    | Add support for **Job** granularity in Conjur                | Allows specific identification of the Secrets Provider **Job**, which enhances security | \- Costly; requires code changes, verify compatibility, tests, docs<br />- Redundant if **service account** serves only the Secrets Provider<br />- ***Not*** a ***seamless*** experience between Milestones | 10 days           |

Implementing the Job granularity would mean that the experience in transitioning between the Milestones is not a smooth one. If a customer uses the Job granularity, they will need to update this in Milestone Rotation because the deployment type will no longer be a Job. 

*Decision*: Solution #1, use **namespace** and **service account** to define the Secrets Provider host in Conjur policy. We would recommend to the customer to create a dedicated service account for Secrets Provider.

#### Customer experience

Deployment for the Secrets Provider will be done using Helm. The customer experience is as follows:

1. Configure K8s Secrets with ***conjur-map*** metadata defined

2. Create `custom-values.yaml` that will contain the required parameters and defaults to override to deploy the Secrets Provider 

   For a full list of default and required parameters, see `values.yaml` and `custom-values.yaml` below

   *When parameters are not configured by customer, their defaults are used*

3. Install Helm Chart for the Secrets Provider, passing in their `custom-values.yaml` like so: `helm install -f custom-values.yaml secrets-provider https://github.com/cyberark/secrets-provider-for-k8s/helm/secrets-provider-1.0.0.tgz`

In Helm, default values are collected in `values.yaml` files. We will supply the customer this file where all default parameters will be defined for them.

*values.yaml*

```yaml
global:
  namespace: default
	
# K8s permission defaults
rbac:
  serviceAccountName: secrets-provider-service-account
  provideExistingServiceAccount: false
  roleName: secrets-provider-role
  roleBinding: secrets-provider-role-binding

# Secrets Provider pod defaults
secrets-provider:
  image: cyberark/secrets-provider-for-k8s:1.0.0 # Version will need to be aligned with package version
  name: cyberark-secrets-provider
	
# Secrets Provider environment variable defaults
secrets-provider-environment:
  conjurSslCertificateName: secrets-provider-ssl-config-map
  secretsDestination: k8s_secrets
  debug: false
  supportRotation: false
```

`custom-values.yaml` will hold both mandatory variables that the customer is required to fill and all defaults they want to override. The customers supplies Helm with these values by loading the Chart with `custom-values.yaml`. For more detailed information on how a customer will install our Chart, see the section on [Installation](#installation).

Helm will first look for values in the customer defined `custom-values.yaml`. If they do not exist, Helm will resort to the defaults configured in `values.yaml` and populate the manifest with those values upon rendering. 

*custom-values.yaml*

```yaml
secrets-provider-environment:
  conjurAccount:
  conjurApplianceUrl:
  conjurAuthnUrl:
  conjurAuthnLogin: 
  conjurSslCertificateValue:
  k8sSecrets:		# Format needs to be array
```
*templates/secrets-access-role.yaml*

```yaml
{{- if eq .Values.rbac.provideExistingServiceAccount "false" }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.rbac.serviceAccountName }}
{{- end }}

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Values.rbac.roleName }}
rules:
  - resources: {{ .Values.rbac.rules.resources }}
    verbs: [ "get", "update" ]

---
{{- if eq .Values.rbac.provideExistingServiceAccount "false" }}
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: {{ .Values.global.namespace }}
  name: {{ .Values.rbac.roleBinding }}
subjects:
  - kind: ServiceAccount
    namespace: {{ .Values.global.namespace }}
    name: {{ .Values.rbac.serviceAccountName }}
roleRef:
  kind: Role
  apiGroup: rbac.authorization.k8s.io
  name: {{ .Values.rbac.roleName }}
{{- end}}
```

Instead of creating a new Service Account, customers can provide us with an already existing Service Account. If so, a new K8s Resource will not be deployed and in custom-values.yaml they will provide us with the name of the existing Service Account. To enable this, the customer will run the following:

```yaml
helm install --set provideExistingServiceAccountName=true custom-values.yaml secrets-provider https://github.com/cyberark/secrets-provider-for-k8s/helm/secrets-provider-1.0.0.tgz
```

*templates/conjur-master-cert.yaml*

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.secrets-provider-environment.conjurSslCertificateName }}
  labels:
    app: app-name
data:
  ssl-certificate: |
    {{ .Values.secrets-provider-environment.conjurSslCertificateValue }}
```

*templates/secrets-provider.yaml*

```yaml
---
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Values.global.namespace }}
spec:
  template:
    serviceAccountName: {{ .Values.serviceAccountName }}
    containers:
    - image: {{ .Values.secrets-provider.image }}
        imagePullPolicy: Always
        name: {{ .Values.secrets-provider.name }}
        env:
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        
        - name: CONJUR_APPLIANCE_URL
          value:  {{ required "conjurApplianceUrl is a required configuration" .Values.secrets-provider-environment.conjurApplianceUrl | quote }}
        
        - name: CONJUR_AUTHN_URL
          value:  {{ required "conjurAuthnUrl is a required configuration"  .Values.secrets-provider-environment.conjurAuthnUrl | quote }}
        
        - name: CONJUR_ACCOUNT
          value:  {{ required "serviceAccount is a required configuration"   .Values.secrets-provider-environment.serviceAccount | quote }}
        
        - name: CONJUR_SSL_CERTIFICATE
           valueFrom:
             configMapKeyRef:
               name: {{ .Values.secrets-provider-environment.conjurSslCertificateName | quote }}
               key: ssl-certificate
                 
        - name: CONJUR_AUTHN_LOGIN
          value:  {{ required "conjurAuthnLogin is a required configuration" Values.secrets-provider-environment.conjurAuthnLogin | quote }}
        
        - name: SECRETS_DESTINATION
          value: k8s_secrets
                    
        - name: SUPPORT_ROTATION			# this variable will allow us to determine if we should exit the Pod or not for later Milestones
          value: {{ required "supportRotation is a required configuration"  .Values.secrets-provider-environment.supportRotation | quote }}
          
        - name: SUPPORT_MULTIPLE_APPS
          value: true
					
        - name: K8S_SECRETS
          value: {{ required "k8sSecrets is a required configuration"  .Values.secrets-provider-environment.k8sSecrets }}
        
        {{- if eq .Values.secrets-provider-environment.debug "true" }}
        - name: DEBUG
          value: true
        {{- end }} 
```

Where `required` declares a value entry as required. If a customer does not supply a value for the required parameters, rendering will fail with the provided error.  `quote` casts yaml values as strings rather than accepting vanilla user input.

There will be one chart for both Openshift and K8s flows. We will differentiate between the two using Helm's Built-in [Capabilities](https://helm.sh/docs/chart_template_guide/builtin_objects/) Object.

Every Secrets Provider entity will need it's own Chart and `custom-values.yaml`.

#### Limitations

- In order for a customer to add a new K8s Secret to the `k8sSecrets`, they will need to tear down the Chart and reinstall it with the updated list of `custom-values.yaml`

#### Packaging Helm Charts

Every Helm project requires a `Chart.yaml` file that describes the project. The Secrets Provider `Chart.yaml` wil resemble the following:

```yaml
apiVersion: v2
description: A Helm chart for CyberArk Secrets Provider for Kubernetes
home: https://www.cyberark.com
icon: https://xebialabs-clients-iglusjax.stackpathdns.com/assets/files/logos/CyberArkConjurLogoWhiteBlue.png
keywords:
- security
- secrets management
maintainers:
- email: conj_maintainers@cyberark.com
  name: Conjur Maintainers
name: secrets-provider
version: 1.0.0
```

Helm Charts are packaged in `tgz` format. 

For each release, we will package our Chart (`helm package secrets-provider`) and the output will be `secrets-provider-<version>.tgz`. This `.tgz` is what we will push to Github.

For more detailed information on how a customer will install our Chart, see the section on [Installation](#installation).

#### Concerns

*Batch retrival*

The Secrets Provider will no longer be running as an init container, rather a separate pod and serving multiple apps. 

*Previously* because each Secrets Provider was sitting within the same Deployment as the app, when a batch retrieval request was made, the requests were dispersed across many call to the Conjur server. If one K8s Secret was not able to be updated with a Conjur value (for example due to a permission error), then the batch request would fail and only that pod would not spin up successfully. 

*Now* that the Secrets Provider is running as a separate entity, a single batch retrieval request will be made to Conjur for *all* application. So if there is a failure in receiving one of the values, then the whole request would fail. In other words, ***no application*** will spin up in the namespace.

To overcome this concern, we will create a new Conjur API endpoint that will return a list of Conjur variables and their responses. That is, secret value for success or error status for failure for each secret. In doing so, we will not fail the whole response if there was an error in retrieving one of the Conjur secrets.

In the case of partial success, the Job will be marked with failure status and another Pod will not be spun up in its place. All secrets that were unable to be fetched will be written to logs. See [logs](#logs) for more detail.

##### Backwards Compatibility

This behavior will be relevant only for supporting multiple applications. If the `SUPPORT_MULTIPLE_APPS` environment variable is not true, we will use the old batch retrieval API endpoint.

In the case where the customer has an older Conjur version and the new Conjur API endpoint does not exist, we fallback to the old batch retrieval API endpoint and write warning in logs.

For a detailed breakdown of the decision process see [here](batch_retrieval_design.md).

### Milestone Rotation

The following section will be dedicated to a high-level design discussion on rotation for Milestone Rotation.

The following high-level solutions for rotation were evaluated:

|      | Solution                               | Elaboration                                                  | Pros                                                         | Cons                                                         | Effort estimation |
| ---- | -------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ | ----------------- |
| 1    | Client-side polling for secrets        | Every configurable amount of time, contact Conjur API server.<br />Possible solutions can be:<br />**a.** Retrieve all secrets<br />**b.** Check if there's new value for any of the secrets available for the requesting host<br />**c.** Retrieve secrets that were changed since last request | - Straight-forward<br />- Simple<br />- Uses existing HTTP API server in Conjur | - Regularly sends API calls even if nothing has changed<br />- Load on server to handle multiple secrets providers<br />- Requires Conjur upgrade to use new endpoint | 15 days           |
| 2    | Event-driven mechanism using pub-sub   | Secrets providers subscribes to relevant events in the server. Server publishes a secret-changed event if a secret was changed | - Efficient, client calls the server only when needed<br />- Allows serving many secrets providers without too much load on the server | - Maintains active connection in the background<br />- Introduces Events mechanism that does not exist<br />- Complex<br />- Need to open ports for allowing connection | 40 days           |
| 3    | Event-driven mechanism using web hooks | Secrets providers registers a URL for calling back when a secret has changed. | - Efficient, client calls the server only when needed<br />- No active connection is held in the background. A connection is made by the server only when needed. | - Complex<br />- Requires allowing connection from server to secrets providers<br />- Follower cannot keep callback URL in the DB, so no guarantee it will be used when needed<br />- Introduces Events mechanism that does not exist | 30 days           |

We decided to go with Solution #1.
Low level design of the solution will be introduced in a separate document.

#### Customer experience

This solution introduces an additional requirement for the customer. They must upgrade their Conjur server to access the new API.
This is not mandatory and if not done so, we will provide a fallback solution **(1.a)** but at a price of efficiency.

For this Milestone, the Secrets Provider Chart will be using `Deployment` in-place of `Job`. New default variables may be introduced but no additional steps will needed to migrate from Milestone Job to Rotation.

#### Concerns

*Load on Server*

To assuage this concern we will test performance and supply our customers with clear limitations, detailing how many Secrets Providers and Followers can be used for optimal performance.

### Lifecycle

#### Milestone Job

##### Installation

The customer will be able to override our provided defaults by creating their own `custom-values.yml` and passing it when installing our Helm chart. To install the customer will run the following: 

`helm install -f custom-values.yaml secrets-provider-for-k8s https://github.com/cyberark/secrets-provider-for-k8s/helm/secrets-provider-1.0.0.tgz`

Where `custom-values.yaml` is the yaml file that the customer creates to override our defaults and where `https://github.com/cyberark/secrets-provider-for-k8s/helm/secrets-provider-1.0.0.tgz ` is the path to our repository where we will push the packaged Helm Chart.

##### Lifecycle

The lifecycle of the Secrets Provider is independent of the application pods. The lifecycle of the Job is the length of the process. In other words, the time it takes to retrieve the Conjur secrets value and update them in the K8s Secrets. After the Pod completes successfully, [the Job and Pod will remain](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup) and the customer can fetch logs on the Job. 

To notify the customer that the Secrets Provider has started and finished it's work, the process will be logged.

##### Upgrade

Because we will be using Helm Charts, we can easily supply customers with the Helm Charts for the current Milestone. All the customer would need to do is update their current Secrets Provider Chart. `helm upgrade -f custom-values.yaml secrets-provider secrets-provider-<version>` 

##### Delete

Customers can uninstall the Secrets Provider release by uninstalling the Helm Chart: `helm uninstall secrets-provider `

It is recommended that customers will delete the Helm Chart in its entirety and ***not*** individual manifests. For example, we recommend that the customer not delete the individual Job manifest itself.

### Network Disruptions

Our customers may experience disruptions in their network connection so it is important to address how we plan to behavior under these circumstances. In the Secrets Provider there is a Timeout/Retry mechanism where we will retry connecting to the Follower before returning an `Retransmission backoff exhausted` error. 

### Order of Deployment

#### Milestone Job

Steps to follow for successful deployment

1. Add all necessary K8s Secrets to `custom-values.yaml`
2. Install Secrets Provider Chart passing in `custom-values.yaml` (*)
3. Run application pods (*)

 (*) The order at which the Secrets Provider and application pod are deployed does not matter because the [application pod will not start](https://kubernetes.io/docs/concepts/configuration/secret/#details) until it has received the keys from the K8s Secrets it references (secretKeyRef).

### Backwards compatibility

#### Milestone Job

Introducing Helm will enhance the user experience and not impact the previous init container solution.

### Performance

#### Milestone Job

We will test and document how many K8s Secrets can be updated in 5 minutes on average. A secret should be either extremely long password. See [Performance tests](#performance-tests) for further explanation.

### Affected Components

- Conjur server for new batch retrieval API

## Security

#### Security boundary

##### Kubernetes/Openshift security boundary

The security boundary is the namespace at which the Secrets Provider and application pods run. The namespace provides an isolation for access control restrictions and network policies. The interaction between accounts and K8s resources is limited by the Service Account that runs in the namespace.

##### Conjur security boundary

The security boundary of the Secrets Provider is the Host identity it uses to authenticate to Conjur via the Conjur Kubernetes Authenticator Client. For full details on how the authentication process works, please see [Conjur Kubernetes Authenticator Client](*https://github.com/cyberark/conjur-authn-k8s-client*).

#### Controls

We accept user input via the *`stringData`* in the K8s Secret resource. We will need to escape/encode this information before using them in the backend. 

The values placed in `custom-values.yaml` are determined by the user. To guarantee that the parameters we accept have not been manipulated for malicious purposes, we will supply a JSON schema to impose an expected structure on input.

## Test Plan

### Integration tests

| **Title** | Description                                                  | Given                                                        | When                                          | Then                                                         |
| --------- | ------------------------------------------------------------ | ------------------------------------------------------------ | --------------------------------------------- | ------------------------------------------------------------ |
| 1         | *Vanilla flow*, Secret Provider Job successfully updates K8s Secrets | - Conjur is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and K8s Secrets are configured<br />- All mandatory values are defined in `custom-values.yaml`<br />- Customer installs Secrets provider Chart | Secrets Provider runs as a Job                | - Secrets Provider pod authenticates and fetches Conjur secrets successfully<br />- All K8s Secrets are updated with  Conjur value <br />- App pods receive K8s Secret with Conjur secret values as environment variables<br />- Verify in logs that deployment kind is listed <br />- Secrets Provider Job runs to completion<br />- Verify logs |
| 2         | Validate new batch retrieval functionality *(for Milestone Batch)* | Secrets Provider host does not have permissions on Conjur secrets / secret is missing / secret value doesn't exist | Secrets Provider runs as a Job                | - Verify in logs that a list of  failed Conjur secrets and those K8s Secrets that were skipped are noted |
| 3         | *Vanilla flow*, non-conflicting Secrets Provider             | Two Secrets Providers have access to different Conjur secret | 2 Secrets Provider Jobs run in same namespace | - All relevant K8s secrets are updated<br />- Verify logs    |
| 4         | Multiple Secrets Providers in same namespace have access to same K8s Secret | Two Secrets Providers have access to same secret             | 2 Secrets Provider Jobs run in same namespace | - No error will be returned and both Secrets Providers will be able to access and update K8s Secret <br />- Verify logs |
| 5         | Check the integrity of all user input from `values.yaml` / `custom-values.yaml` |                                                              |                                               | - Verify all input values are escaped/encoded                |
| 6         | Service Account does not exist                               | `provideExistingServiceAccount=true` but Service Account provided does not exist in namespace | Secrets Provider Chart is installed           | Job will fail because Service Account does not exist         |
| 7         | Add retry check in all regression tests                      |                                                              |                                               | - Verify retry is also written in logs                       |
| 8         | *Regression tests*                                           | All regression tests should pass (both Conjur Open Source/Enterprise)         |                                               |                                                              |
| 9         | *Performance tests* according to SLA                         |                                                              |                                               |                                                              |
| 10        | *Add support for OC 4.3*                                     | All integration tests should pass on OC 4.3                  |                                               | *Not final*                                                  |

### Performance tests

The performance tests should answer the following question:

* *How many secrets can 1 Follower support?*

More specifically, regarding secrets provider that pulls secrets every given amount of time, the question is:

* *How many secrets can 1 Follower retrieve every 1 minute?*

To answer it, we will measure what is the maximum number of secrets 1 Follower can serve.
To measure it, we will launch 100, 1000 and 10K goroutines, each sending batch secret retrieval API call to the follower repeatedly during 1 minute. We measure the total amount of secrets received during 1 minute for each, and the result is the maximum number of secrets received.

Since this number may vary depending on many variables, we will do this test for each of the following variables independently:

| Variable                                    | Const value while testing other variables | Values to test              |
| ------------------------------------------- | ----------------------------------------- | --------------------------- |
| Number of secrets in each batch request     | 20                                        | 1, 20, 100, 500, 2000, 10K  |
| Number of Secrets Providers                 | 1                                         | 1, 10, 50, 100, 500, 1000   |
| Secret path length                          | 20 chars                                  | 10, 20, 50, 100, 200 chars  |
| Secret value length                         | 30 chars                                  | 30, 50, 100, 200, 500 chars |
| Follower distance                           | None (Same K8S cluster)                   | None, Medium, Far           |
| Number of Followers behind L4 Load Balancer | 1                                         | 1, 2, 3, 5, 10              |

From these performance tests we will be able to detail our limitations in our documentation and provide recommended deployments.

## Logs

| **Scenario**                                                 | **Log message**                                              | Log level |
| ------------------------------------------------------------ | ------------------------------------------------------------ | --------- |
| Secrets Provider spins up                                    | Kubernetes Secrets Provider v\*%s\* starting up as a %s... (Job/Deployment) | Info      |
| Secrets Provider batch retrieval has partial success         | Failed to retrieve Conjur secrets '%s'                       | Error     |
| Secrets Provider batch retrieval has partial success *(for Milestone Batch)* | Skipped on K8s Secrets '%s'. Reason: failed to retrieve their Conjur secrets | Debug     |
| Secrets Provider success *(for Milestone Batch)*             | Successfully updated '%d' out of '%d' K8s Secrets            | Info      |
| Old batch retrieval endpoint *(for Milestone Batch)*         | Warning: Secrets Provider cannot efficiently run because Conjur server is not up-to-date. Please consider upgrading to '%d' | Warn      |
| Job has completed and is terminating                         | Kubernetes Secrets Provider Job has completed…               | Info      |
| Acknowledge when a retry is taking place                     | Retrying '%d' out of '%d' ...                                | Info      |

### Audit 

All fetches on a Conjur Resource are individually audited, creating its own audit entry. Therefore there will be no changes in audit behavior for this Milestone.

## Documentation

- Add instruction for how to deploy Secrets Provider Phase 2
- Add limitations and best practices to our documentation based off of performance tests
- It is up to user if the Secrets Provider and Application will use the same Service Account. We need to recommend/show in our documentation examples of the same service account shared between application and Secrets Provider.

## Open questions

1. Should customers be given two different Helm Charts for K8s and Openshift? *1 chart (with flag)*
2. What versions of OC/K8S should we support? *TBD*
3. What version of Helm should we support/deploy with? *Helm 3.2.4*

## Implementation plan

### Delivery Plan 

#### Milestone Job

- [x] Solution design approval + Security review approval
- [ ] Implement Phase 2 Milestone Job functionality 
  - [ ] Build Helm charts and bring up the Secrets Provider ***(3 days)***
- [ ] Implement test plan
  - [ ] Integration ***(3 days)***
  - [ ] Deliver recommended customer architecture - not in pipeline ***(2 days)***
- [ ] Security items have been taken care of
  - [ ] StringData encoding/escaping 
  - [ ] JSON schema for ` values`/`custom-values.yaml` validations ***(2 days)***
- [ ] Logs review by TW + PO and add to codebase ***(1 day)***
- [ ] Documentation has been given to TW + approved ***(2 days)***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(1 day)***
- [ ] Create demo for Milestone Job functionality ***(1 day)***
- [ ] Versions are bumped in all relevant projects (if necessary) and automate the Helm package and release process ***(2 days)***

 **Total:** ~17 days **(~3.5 weeks)**

Note that the above estimation does not include OC 4.3 testing and pipeline additions



#### Milestone Batch

- [ ] Batch Retrieval, create new Conjur API endpoint 
  - [ ] Work in Conjur ***(10 days)***
  - [ ] Work in Secrets Provider for K8s ***(3 days)***
- [ ] Implement test plan
  - [ ] Integration tests in Secrets Provider ***(2 days)***
  - [ ] Integration tests in Conjur ***(3 days)***
  - [ ] Performance tests align with SLA ***(5 days)***
- [ ] Update documentation, updating Batch retrieval limitation ***(1 days)***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(1 day)***
- [ ] Create demo for Milestone Batch ***(1 day)***
- [ ] Versions are bumped in all relevant projects (if necessary) ***(1 day)***

**Total:** ~27 days **(~5.5 weeks)**

*Risks that could delay project completion*

- Cross-team dependency (conjur-core) designing the new Conjur API endpoint for batch retrieval

  It is possible that architecturally we will decide against creating a new API and build ontop of the existing batch retrieval endpoint. If so, this might delay the project completion by ***5 days***.

  - Mitigation: Early communication with the conjur-core team and architects. 


### Milestone Rotation

- [ ] Rotation endpoint, create new Conjur API endpoint
  - [ ] Work in Conjur ***(10 days)***
  - [ ] Work in Secrets Provider for K8s ***(3 days)***
- [ ] Implement test plan
  - [ ] Integration tests in Secrets Provider ***(2 days)***
  - [ ] Integration tests in Conjur ***(3 days)***

- [ ] Update documentation, supporting Rotation ***(2 days)***
- [ ] Logs review by TW + PO and add to codebase ***(1 day)***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(1 day)***
- [ ] Create demo for Milestone Rotation functionality ***(1 day)***
- [ ] Versions are bumped in all relevant projects (if necessary) ***(1 day)***

**Total:** ~24 days **(~5 weeks)**

Estimated to complete Milestones on November 12
