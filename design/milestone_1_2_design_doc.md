# Secrets Provider Phase 2 Milestone 1 - Solution Design

## Table of Contents

* [Glossary](#glossary)

  * [Useful links](#useful-links)
  * [Background](#background)
    + [Motivation](#motivation)
  * [Requirements](#requirements)
    + [Milestone 1 *(current)*](#milestone-1---current--)
    + [Milestone 2](#milestone-2)
  * [Solution](#solution)
    + [Milestone 1: Serve K8s secrets to multiple applications once (no rotation)](#milestone-1--serve-k8s-secrets-to-multiple-applications-once--no-rotation-)
    + [Design](#design)
      - [How does a Job configured via Helm answer the requirements?](#how-does-a-job-configured-via-helm-answer-the-requirements-)
      - [What drawbacks does this solution have?](#what-drawbacks-does-this-solution-have-)
      - [Customer experience](#customer-experience)
      - [Packaging Helm Charts](#packaging-helm-charts)
    + [Milestone 2: Rotation](#milestone-2--rotation)
    + [Lifecycle](#lifecycle)
      - [Milestone 1](#milestone-1)
        * [Installation](#installation)
        * [Lifecycle](#lifecycle-1)
        * [Update](#update)
        * [Delete](#delete)
    + [Network Disruptions](#network-disruptions)
    + [Order of Deployment](#order-of-deployment)
      - [Milestone 1](#milestone-1-1)
    + [Backwards compatibility](#backwards-compatibility)
      - [Milestone 1](#milestone-1-2)
    + [Performance](#performance)
      - [Milestone 1](#milestone-1-3)
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

###  Milestone 1 *(current)*

- Secrets Provider runs as a separate entity, serving multiple application containers that run on multiple pods
- Solution needs to be native to Kubernetes
- Lifecycle should support update/removal of deployment
- Provide a way for customer to understand the state of the Secret Provider - when it finished initializing

###  Milestone 2

- Secrets Provider support secret rotation

For seamless shift between milestones, we will deploy using Helm.

## Solution

The solution is to allow different deployments and behave differently based on the use-case chosen. The following decision tree depicts customer uses-cases for Secrets Provider deployments:

 ![Secrets Provider flavors decision flow chart](https://user-images.githubusercontent.com/31179631/85747023-975bf500-b70f-11ea-8e26-1134068fe655.png) 

These variations will be supported using the same Secrets Provider image, but the behavior will vary dynamically depending on the chosen deployment method that is configured via Helm Chart.

*In this document we will focus on **Milestone 1***, deploying the Secrets Provider as a Job

### Milestone 1: Serve K8s secrets to multiple applications once (no rotation)

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
| 2    | Add support for **Job** granularity in Conjur                | Allows specific identification of the Secrets Provider **Job**, which enhances security | \- Costly; requires code changes, verify compatibility, tests, docs<br />- Redundant if **service account** serves only the Secrets Provider<br />- ***Not*** a ***seamless*** experience between Milestones because i | 10 days           |

Implementing the Job granularity would mean that the experience in transitioning between the Milestones is not a smooth one. If a customer uses the Job granularity, they will need to update this in Milestone 2 because the deployment type will no longer be a Job. 

*Decision*: Solution #1, use **namespace** and **service account** to define the Secrets Provider host in Conjur policy. We would recommend to the customer to create a dedicated service account for Secrets Provider.

#### Customer experience

Deployment for the Secrets Provider will be done using Helm. The customer experience is as follows:

1. Configure K8s Secrets with ***conjur-map\*** metadata defined

2. Create `custom-values.yaml` that will contain the required parameters to deploy the Secrets Provider 

   For a full list of required parameters, see `values.yaml` and `custom-values.yaml` below

   *When parameters are not configured by customer, their defaults are used*

3. Install Helm Chart for the Secrets Provider, passing in their `custom-values.yaml` like so: `helm install conjur -f custom-values.yaml https://github.com/cyberark/secrets-provider-for-k8s/helm/secrets-provider-1.0.0.tgz`



In Helm, default values are collected in `value.yaml` files. We will supply the customer this file where all default parameters will be defined for them.

*values.yaml*

```yaml
global:
  namespace: default
	
# K8s permission defaults
rbac:
  serviceAccountName: secrets-provider-service-account
  roleName: secrets-provider-role
  roleBinding: secrets-provider-role-binding

# Secrets Provider pod defaults
secrets-provider:
  image: cyberark/secrets-provider-for-k8s
  name: cyberark-secrets-provider
	
# Secrets Provider environment variable defaults
secrets-provider-environment:
  conjurSslCertificateName: secrets-provider-ssl-config-map
  containerMode: init
  secretsDestination: k8s_secrets
  debug: false
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
Variables from the `values.yaml` / `custom-values.yaml ` are applied for our project as follows:

*templates/secrets-access-role.yaml*

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.rbac.serviceAccountName }}

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Values.rbac.roleName }}
rules:
  - resources: {{ .Values.rbac.rules.resources }}
    verbs: [ "get", "update" ]

---
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
```

*templates/conjur-master-cert.yaml*
```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.secrets-provider-environment.conjurSslCertificateName }}
  labels:
    app: test-env
data:
  ssl-certificate: |
    {{ .Values.secrets-provider-environment.conjurSslCertificateValue }}
```

*templates/secrets-provider.yaml*
```yaml
---
apiVersion: batch/v1
kind: Job
apiVersion: apps/v1
kind: Deployment
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
                    
        - name: CONTAINER_MODE
          value: init
					
        - name: K8S_SECRETS
          value: {{ required "k8sSecrets is a required configuration"  .Values.secrets-provider-environment.k8sSecrets }}
        
        {{- if eq .Values.secrets-provider-environment.debug "true" }}
        - name: DEBUG
          value: true
        {{- end }} 
```

Where `required` declares a value entry as required. If a customer does not supply a value for the required parameters, rendering will fail with the provided error.  `quote` casts yaml values as strings rather than accepting vanilla user input.

#### Limitations

- In order for a customer to add a new K8s Secret to the K8S_SECRETS, they will need to tear down the Chart and reinstall it with the updated list of `custom-values.yaml`

#### Packaging Helm Charts

Every Helm project requires a `Chart.yml` file that describes the project. The Secrets Provider `Chart.yaml` wil resemble the following:

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

### Milestone 2: Rotation

*TBD*

### Lifecycle

#### Milestone 1

##### Installation

The customer will be able to override our provided defaults by creating their own `custom-values.yml` and passing it when installing our Helm chart. To install the customer will run the following: 

`helm install conjur -f custom-values.yaml https://github.com/cyberark/secrets-provider-for-k8s/helm/secrets-provider-1.0.0.tgz`

Where `custom-values.yaml` is the yaml file that the customer creates to override our defaults and where `https://github.com/cyberark/secrets-provider-for-k8s/helm/secrets-provider-1.0.0.tgz ` is the path to our repository where we will push the packaged Helm Chart.

##### Lifecycle

The lifecycle of the Secrets Provider is independent of the application pods. The application pods that detail K8s Secrets in their environment [will not start](*https://kubernetes.io/docs/concepts/configuration/secret/#details*) until the K8s Secrets they require are populated with the expected key. 

To notify the customer that the Secrets Provider has started and finished it's work, the process will be logged.

##### Update

Because we will be using Helm Charts, we can easily supply customers with the Helm Charts for the current Milestone. All the customer would need to do is delete their current Secrets Provider Chart and install the new one.

It is possible that future Milestones will require additional configurations. If so, the customer can add the necessary parameter with relative easy by manually defining it for us in their `custom-values.yaml` file. If not supplied, we will take our default.

##### Delete

Customers can uninstall the Secrets Provider release by uninstalling the Helm Chart: `helm uninstall secrets-provider `

It is recommended that customers will delete the Helm Chart in its entirety and ***not*** individual manifests. For example, we recommend that the customer not delete the individual Job manifest itself.

The lifecycle of the Job is the length of the process. In other words, the time it takes to retrieve the Conjur secrets value and update them in the K8s Secrets. After the Pod terminates successfully, [the Job will remain](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup) and the customer can fetch logs on the Job. 

### Network Disruptions

Our customers may experience disruptions in their network connection so it is important to address how we plan to behavior under these circumstances. In the Secrets Provider we will develop a Timeout/ Retry mechanism where we will retry to connect to the Follower for 10 seconds and retry twice before returning an `Retransmission backoff exhausted` error. 

### Order of Deployment

#### Milestone 1

Steps to follow for successful deployment

1. Add all necessary K8s Secrets to `custom-values.yaml`
2. Install Secrets Provider Chart passing in `custom-values.yaml` (*)
3. Run application pods (*)

 (*) The order at which the Secrets Provider and application pod are deployed does not matter because the [application pod will not start](https://kubernetes.io/docs/concepts/configuration/secret/#details) until it has received the keys from the K8s Secrets it references (secretKeyRef).

### Backwards compatibility

#### Milestone 1

Introducing Helm will enhance the user experience and not impact the previous init container solution.

### Performance

#### Milestone 1

We will test and document how many K8s Secrets can be updated in 5 minutes on average. A secret should be either extreme long password. See [Performance tests](#performance-tests) for further explanation.

### Affected Components

## Security

#### Security boundary

##### Kubernetes/Openshift security boundary

The security boundary is the namespace at which the Secrets Provider and application pods run. The namespace provides an isolation for access control restrictions and network policies. The interaction between accounts and K8s resources is limited by the Service Account that runs in the namespace.

##### Conjur security boundary

The security boundary of the Secrets Provider is the Host identity it uses to authenticate to Conjur via the Conjur Kubernetes Authenticator Client. For full details on how the authentication process works, [please visit](*https://github.com/cyberark/conjur-authn-k8s-client*).

#### Controls

The value for *`stringData`* in the K8s Secret resource is a String of user input values. To guarantee that this field is not manipulated for malicious purposes, we are validating this input.

The values placed in `custom-values.yaml` are determined by the user. To guarantee that the parameters we accept have not been manipulated for malicious purposes, inside the Helm template we cast each value as a string via `quote` and we will validate this input.

## Test Plan

### Integration tests

| **Title** | **Given**                                                    | **When**                                                     | **Then**                                      | **Comment**                                                  |
| --------- | ------------------------------------------------------------ | ------------------------------------------------------------ | --------------------------------------------- | ------------------------------------------------------------ |
| 1         | *Vanilla flow*, Secret Provider Job successfully updates K8s Secrets | - Conjur is running<br />- Authenticator is defined<br />- Secrets defined in Conjur and K8s Secrets are configured<br />- Service Account has correct permissions (get/update/list) <br />- Secrets Provider Job manifest is defined<br />-  `K8S_SECRETS` env variable is configured | Secrets Provider runs as a Job                | - Secrets Provider pod authenticates and fetches Conjur secrets successfully<br />- All K8s Secrets are updated with  Conjur value <br />- App pods receive K8s Secret with Conjur secret values as environment variable<br />- Verify in logs that deployment type is listed <br />- Secrets Provider Job terminates on completion of task<br />- Verify logs |
| 2         | *Vanilla flow*, non-conflicting Secrets Provider<br />       | Two Secrets Providers have access to different Conjur secret | 2 Secrets Provider Jobs run in same namespace | - All relevant K8s secrets are updated<br />- Verify logs    |
| 3         | Multiple Secrets Providers in same have access to same K8s Secret | Two Secrets Providers have access to same secret             | 2 Secrets Provider Jobs run in same namespace | - No race condition and Secrets Providers will not override each other<br />- Verify logs |
| 4         | Check the integrity of all user input from `values.yaml` / `custom-values.yaml` |                                                              |                                               | - Validate no malicious input was received and escape/encode |
| 4         | *Regression tests*                                           | All regression tests should pass (both Conjur / DAP)         |                                               |                                                              |
| 5         | *Add support for OC 4.3*                                     | All integration tests should pass on OC 4.3                  |                                               |                                                              |

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



## Concerns

*Impact on user experience due to batch retrieval* *functionality*

The Secrets Provider will no longer be running as an init container, rather a separate pod and serving multiple apps. 

*Previously* because each Secrets Provider was sitting within the same Deployment as the app, when a batch retrieval request was made, the requests were dispersed across many call to the Conjur server. If one K8s Secret was not able to be updated with a Conjur value (for example due to a permission error), then the batch request would fail and only that pod would not spin up successfully. 

*Now* that the Secrets Provider is running as a separate entity, a single batch retrieval request will be made to Conjur for *all* application. So if there is a failure in receiving one of the values, then the whole request would fail. In other words, ***no application*** will spin up in the namespace.

## Open questions

1. Should customers be given two different Helm Charts for K8s and Openshift? *TBD*
2. What versions of OC/K8S should we support? *TBD*
3. What version of Helm should we support/deploy with? *Helm 3.2.4*

## Implementation plan

#### Delivery Plan (Milestone 1)

- [ ] Solution design approval + Security review approval
- [ ] Implement Phase 2 Milestone 1 functionality ***(~2 days)\***
- [ ] Implement test plan (Integration + Unit + Performance tests align with SLA) ***(~4 days)\***
- [ ] Add support for OC 4.3 in our pipeline ***(~2 days)\***
- [ ] Security items have been taken care of (if they exist) ***(TBD)\***
- [ ] Logs review by TW + PO ***(~1 day)\*** 
- [ ] Documentation has been given to TW + approved ***(~2 days)\***
- [ ] Engineer(s) not involved in project use documentation to get end-to-end ***(~1 day)\***
- [ ] Create demo for Milestone 1 functionality ***(~1 day)\***
- [ ] Versions are bumped in all relevant projects (if necessary) and automate the Helm package and release process ***(~1 days)\***

 **Total:** ~12 days of non-parallel work **(~2.5 weeks)**
