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

   For a full list of required parameters, see `values.yaml` below

   *When parameters are not configured by customer, their defaults are used*

3. Install Helm Chart for the Secrets Provider

We will supply the customer with a  `values.yaml` file where all default parameters are collected. Our `values.yaml` will resemble the following:

```
# K8s permissioning defaults
permission:
	serviceAccountName: secrets-provider-account
	roleName: secrets-provider-role
	roleBinding: secrets-provider-role-binding

# Secrets Provider pod defaults
secrets-provider:
	image: cyberark/secrets-provider-for-k8s
	name: cyberark-secrets-provider
	namespace: default
	ttlSecondsAfterFinished: 0	# Length at which Job should stay alive even after finishing task. Used for logging.
	
# Secrets Provider environment variable defaults
secrets-provider-environment:
	conjurApplianceUrl: https://conjur-follower.default.svc.cluster.local
	conjurAuthnUrl: https://conjur-follower.default.svc.cluster.local/authn-k8s/authn-default
	conjurAuthnLogin: host/conjur/authn-k8s/authn-default/default
	conjurAccount: default
	conjurSslCertificate: conjur-master-ca-env
	debug=true
	k8sSecrets=[]
	containerMode=init
	secretsDestination=k8s_secrets
	rotationSupport=false
```

Customers can override these defaults by loading the Chart with `custom-values.yaml`. Helm will first look for values in the customer defined `custom-values.yaml`. If they do not exist, Helm will resort to the defaults configured in `values.yaml` and populate the manifest with those values. 

For more detailed information on how a customer will install our Chart, see the section on [Installation](#installation).

Variables from the `values.yaml` / `custom-values.yml `are applied for our project as follows:

*templates/secrets-access-role.yaml*

```
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.permission.serviceAccount }}

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Values.permission.roleName }}
rules:
  - apiGroups: [""]
    resources: {{ .Values.permission.rules.resources }}
    verbs:
    	rules:
			resources: secrets
			verbs: [ "get", "update" ]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: {{ .Values.secrets-provider.namespace }}
  name: {{ .Values.permission.roleBinding }}
subjects:
  - kind: ServiceAccount
    namespace: {{ .Values.secrets-provider.namespace }}
    name: {{ .Values.permission.serviceAccountName }}
roleRef:
  kind: Role
  apiGroup: rbac.authorization.k8s.io
  name: {{ .Values.permission.roleName }}
```

*templates/secrets-provider.yaml*

The following template takes into account the anticipated flows for Milestone 1 and 2.

```yaml
---
{{- if eq .Values.secrets-provider-environment.rotationSupport "false" }}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Values.secrets-provider.namespace }}
spec:
	ttlSecondsAfterFinished: {{ .Values.secrets-provider.ttlSecondsAfterFinished }}
{{ else }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Values.secrets-provider.namespace }}
spec:
{{- end }}
  template:
    serviceAccountName: {{ .Values.serviceAccount }}
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
          value:  {{ .Values.secrets-provider-environment.conjurApplianceUrl }}

        - name: CONJUR_AUTHN_URL
          value:  {{ .Values.secrets-provider-environment.conjurAuthnUrl }}

        - name: CONJUR_ACCOUNT
          value:  {{ .Values.secrets-provider-environment.serviceAccount }}

        - name: CONJUR_SSL_CERTIFICATE
       	  valueFrom:
            configMapKeyRef:
        	  name: {{ .Values.secrets-provider-environment.conjurSslCertificate }}
       		  key: ssl-certificate

        - name: DEBUG
       	  value:  {{ .Values.secrets-provider-environment.debug }}

        - name: CONJUR_AUTHN_LOGIN
       	  value:  {{ Values.secrets-provider-environment.conjurAuthnLogin }}

        {{- if eq .Values.secrets-provider-environment.rotationSupport "true" }}
        - name: SYNC_INTERVAL_TIME
          value: {{ .Values.secrets-provider-environment.syncInternalTime }}
        {{- end }}

        {{- if .Values.secrets-provider-environment.k8SecretsLabels }}
        - name: K8S_SECRETS_LABELS
          value: {{ .Values.secrets-provider-environment.k8sSecretsLabels }}
        {{- end }}
```

Note that when we transition to Milestone 2 we will need to keep the following in mind:

`rotationSupport=false` default will need to be updated to`rotationSupport=true`. 

We will need to add `syncIntervalTime=300` (5 minutes) to the default `values.yaml` where `syncIntervalTime` will represent how often the Secrets Provider will attempt to fetch new Conjur secrets. If the customer would like to change this parameter, they will need to do so manually by *reinstalling* *the Secrets Provider Chart*. 

`ttlSecondsAfterFinished` will need to be removed from defaults

All these changes *demands* that the customer manually deletes and installs the latest Helm Charts

#### Packaging Helm Charts

Every Helm project requires a `Chart.yml` file that describes the project. The Secrets Provider `Chart.yaml` wil resemble the following:

```yaml
apiVersion: v1
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

The customer will be able to override these defaults by creating their own `custom-values.yml` and passing it when installing our Helm chart. For example:

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

It is recommended that customers will delete the Helm Chart in its entirety and individual manifests. For example, we recommend that the customer not delete the individual Job manifest itself.

The lifecycle of the Job is the length of the process. In other words, the time it takes to retrieve the Conjur secrets value and update them in the K8s Secrets. To examine the Secrets Provider Job logs, the customer can configure the Job to stay "alive" by adding **ttlSecondsAfterFinished**, with the desired about of seconds, to their Secrets Provider yaml. Because of the nature of a Job, a manual delete is not necessary.

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

1. Once the Job terminates, there is no way to access the Job's logs. **ttlSecondsAfterFinished** allows the Job to stay alive even after the task terminates. Do we want to supply a **ttlSecondsAfterFinished** default for customers so that they can evaluate Job logs even after the Job terminates?
2. Should customers be given two different Helm Charts for K8s and Openshift?

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
