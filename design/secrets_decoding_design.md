# Secrets Provider - Base64 Secrets Decoding Design

## Table of Contents

- [Secrets Provider - Base64 Secrets Decoding Design](#secrets-provider---base64-secrets-decoding-design)
  * [Table of Contents](#table-of-contents)
  * [Useful links](#useful-links)
  * [Background](#background)
  * [Solution](#solution)
    + [Kubernetes Secrets Mode](#kubernetes-secrets-mode)
    + [P2F Mode](#p2f-mode)
  * [Design](#design)
    + [Kubernetes Secrets Mode](#kubernetes-secrets-mode-1)
    + [P2F Mode](#p2f-mode-1)
    + [Backwards compatibility](#backwards-compatibility)
  * [Test Plan](#test-plan)
    + [Annotation Tests (K8s Secrets)](#annotation-tests--k8s-secrets-)
    + [Annotation Tests (P2F)](#annotation-tests--p2f-)
    + [Decoding Tests](#decoding-tests)
    + [e2e Tests](#e2e-tests)
    + [Audit](#audit)
  * [Documentation](#documentation)
  * [Open questions](#open-questions)
  * [Tasks](#tasks)
    + [Add a struct for storing content-types of individual Conjur secrets (K8s Secrets)](#add-a-struct-for-storing-content-types-of-individual-conjur-secrets--k8s-secrets-)
    + [Add/update a struct for storing content-types of individual Conjur secrets (P2F)](#add-update-a-struct-for-storing-content-types-of-individual-conjur-secrets--p2f-)
    + [Secrets with 'base64' content-types are decoded (K8s Secrets)](#secrets-with--base64--content-types-are-decoded--k8s-secrets-)
    + [Secrets with 'base64' content-types are decoded (P2F)](#secrets-with--base64--content-types-are-decoded--p2f-)
    + [Add e2e secrets decoding test and update manifests for the following cases:](#add-e2e-secrets-decoding-test-and-update-manifests-for-the-following-cases-)
    + [Create/update docs](#create-update-docs)
    + [Create artifact for tech writers](#create-artifact-for-tech-writers)
    + [Docs walkthrough](#docs-walkthrough)

## Useful links

| Name                                     | Link                                                         |
| -----------------------------------------| ------------------------------------------------------------ |
| Secrets Provider Configuration Reference | https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ref.htm |
| Feature Functional Spec (internal)       | https://{cyberark_confluence_host}/display/rndp/Secrets+Provider%3A+Secrets+decoding+for+k8s+Functional+Specification |

## Background
Secrets Provider provides Kubernetes-based applications with access to secrets that are stored and managed in Conjur. These secrets can be any binary/string data. In some cases it may be beneficial, or necessary, to store secret values in Conjur as base64 encoded strings depending on the nature and origin of the secret. For example, Vault/EPV requires that binary secrets are stored with base64 encoding which can result in these encoded secrets being synchronized to Conjur.

We would like to increase flexibility of consuming applications by introducing a mechanism for Secrets Provider to decode individual secrets on the fly. This would allow Secrets Provider to work with applications where it may be impossible to decode the values, i.e. when running compiled 3rd party applications.

Currently it is possible to decode secrets only when using Push-to-file mode (P2F), and when defining custom templates for the secrets file. See: [P2F - Additional Template Functions](https://github.com/cyberark/secrets-provider-for-k8s/blob/main/PUSH_TO_FILE.md#additional-template-functions). This should be extended to work with K8s secrets mode via a common notation to simplify usage of the feature, as well as decode secrets without needing a custom template definition in P2F mode.

## Solution
### Kubernetes Secrets Mode
Kubernetes secrets mode relies on a secrets manifest to configure Conjur secrets, rather than pod annotations. In this case we should extend the schema for secrets manifests to include an optional content-type on conjur secrets. This could look like:
```
apiVersion: v1
kind: Secret
metadata:
  name: test-k8s-secret
type: Opaque
stringData:
  conjur-map: |-
    secret: secrets/test_secret
    user: 
      id: policy/path/base64_user
      content-type: base64
```

### P2F Mode
Kubernetes Pod annotations are currently used to define which Conjur secrets are to be retrieved when using P2F (push-to-file) mode. [See annotations reference](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ref.htm?tocpath=Integrations%7COpenShift%252FKubernetes%7CApp%20owner%253A%20Set%20up%20workloads%20in%20Kubernetes%7CSet%20up%20workloads%20(JWT-based%20authn)%7CSecrets%20Provider%20for%20Kubernetes%7C_____3#AdditionalannotationsforPushtoFilemode). The schema for the `conjur.org/conjur-secrets.{secret-group}` annotations can be extended to include an optional content-type attribute. This could look like:
```
conjur.org/conjur-secrets.groupname: |
  - user: policy/path/base64_user     # valid annotation with content-type
    content-type: base64
```

P2F already leverages Kubernetes' Downward API and volumeMounts to pass these annotations into the Secrets Provider container. This will make it possible to determine which secrets should be decoded before pushing them to the Kubernetes secret file. 

## Design
The goal of this design is to make usage of this feature as consistent as possible across different configurations. Due to the nature of configuring Conjur secrets in K8s secrets mode versus P2F mode, it makes the most sense to add the configuration in-place where Conjur secrets are already defined.
* Kubernetes Secrets mode - add content-type to the existing K8s secrets manifest
* P2F mode - add content-type to the existing pod annotations

In both modes the solution will need to be backwards compatible with existing annotation formats. 

The logic and structs should added for this feature should be shared between both modes when possible. For example, P2F mode uses [SecretSpec struct](https://github.com/cyberark/secrets-provider-for-k8s/tree/main/pkg/secrets/common/secret_spec.go) which could be updated with a ContentType field. Ideally both YAML formats can be unmarshaled into a common struct since they will contain the same data:
* Alias
* Variable ID / Path
* ContentType
  * Allowed values = ['text', 'base64']
  * Default = 'text'

### Kubernetes Secrets Mode
In order to keep Conjur secrets configuration from creeping into other locations, we will add the content-type under the existing `conjur-map` annotation of the secrets manifest. This requires adding an additional supported multiline format:
```
conjur-map: |-
  <alias_value>: <path_value>                 # supported
  <alias_value>:                              # to be added
    id: <path_value>
    content-type: <content_type_value>    
```

In the case where multiple Kubernetes secrets reference the same Conjur secret, we should support a different content-type definition for each instance of the secret. i.e. 'k8s-secret-decode' can reference `path/to/somevar` with a content-type of base64, and `k8s-secret-dont-decode` can reference `path/to/somevar` with a content-type of text, and each K8s secret will be updated with the correct decoded or non-decoded secret value.

### P2F Mode
For P2F, we can update the existing `conjur.org/conjur-secrets.{secret-group}` annotations with the following allowed formats:
```
conjur.org/conjur-secrets.groupname: |
  - <alias_value>: <path_value>               # supported
  - <path_value>                              # supported
  - <alias_value>: <path_value>               # to be added
    content-type: <content_type_value>
```

In the case where multiple secret groups reference the same Conjur secret, we should support a different content-type definition for each instance of the secret. i.e. `conjur.org/conjur-secrets.decode-this-group` can reference `path/to/somevar` with a content-type of base64, and `conjur.org/conjur-secrets.dont-decode-this-group` can reference `path/to/somevar` with a content-type of text, and each secret group's output file will be updated with the correct decoded or non-decoded secret value.

### Backwards compatibility
This enhancement will not modify the default behavior of Secrets Provider. It will continue to assume that secrets should be delivered "as-is" unless it specifically finds a content-type annotation which indicates that the secret value should be decoded.

## Test Plan
### Annotation Tests (K8s Secrets)
Ensure that the 'base64' content-type of a Secret is captured for valid configuration:
```
conjur-map: |-
  user: 
    id: policy/path/base64_user
    content-type: base64
```

Ensure that the 'text' content-type of a Secret is captured by default, when specified, or when an invalid content-type is provided:
```
conjur-map: |-
  user: 
    id: policy/path/base64_user
    content-type: text
```
```
conjur-map: |-
  user: policy/path/user
```
```
conjur-map: |-
  user: 
    id: policy/path/base64_user
    content-type: invalidtype
```

Unit tests should also be implemented for error messages that result from invalid annotations, parsing errors, etc.

### Annotation Tests (P2F)
Ensure that the 'base64' content-type of a Secret is captured for valid configuration:
```
conjur.org/conjur-secrets.groupname: |
  - user: policy/path/base64_user     # valid annotation with content-type
    content-type: base64
```

Ensure that the 'text' content-type of a Secret is captured by default, when specified, or when an invalid content-type is provided::
```
conjur.org/conjur-secrets.groupname: |
  - user: policy/path/user            # explicit 'text' content-type is valid
    content-type: text
```
```
conjur.org/conjur-secrets.groupname: |
  - url: policy/path/url              # will default to 'text' content type
```
```
conjur.org/conjur-secrets.groupname: |
  - user: policy/path/user            # invalid content-type defaults to 'text'
    content-type: invalidtype
```

Unit tests should also be implemented for error messages that result from invalid annotations, parsing errors, etc.

### Decoding Tests
Unit tests for any helper functions added to assist with decoding secrets.

Update integration tests in both [K8s secrets mode](https://github.com/cyberark/secrets-provider-for-k8s/tree/main/pkg/secrets/k8s_secrets_storage/provide_conjur_secrets_test.go) and [P2F mode](https://github.com/cyberark/secrets-provider-for-k8s/tree/main/pkg/secrets/pushtofile/provide_conjur_secrets_test.go) to validate that an encoded variable with base64 content-type is decoded.

### e2e Tests
In addition to the unit/integration tests, we likely will want five e2e tests to cover the following cases with secrets decoding:
* K8s Secrets
* K8s Secrets with rotation
* Push-to-file
* Push-to-file with rotation
* Decoding secrets > 65k characters

### Audit 
The solution should produce an audit entry for each secret that was decoded as a result of the secrets configuration. It should also log errors/warnings on the following conditions:

Failure on configuration:
  - Malformed secret (alias/path)
    - Log YAML parsing error
    - Secrets Provider fails to start
  - Malformed content-type
    - If YAML can still be parsed, create a secret with default content type (text)
    - Log (warning)

Failure on providing secrets:
- Fail to decode an encoded secret (invalid base64)
   - Log (warning)
   - Provide original value
   - Secrets provider continues to run

## Documentation
Since this feature is activated via the manifest, it would be reasonable to add a note to any sample manifests in existing documentation about the optional 'content-type' field for secret annotations. Both K8s Secrets and P2F documentation should have a brief mention of where and how configuration is needed to activate the feature respectively.

## Open questions
* (Out of scope) Conjur variables support a mime-type attribute and custom annotations. See: [Conjur policy variable attributes](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/12.4/en/Content/Operations/Policy/statement-ref-variable.htm#Attributes). Either of these could be used to determine that an individual Conjur secret is encoded. This likely wouldn't be compatible with the current implementation using Conjur API's batch retrieval endpoint, but it would allow the decoding to be handled in the API instead of Secrets Provider. Could also have compatility issues with secrets that come from synchronizer and/or require manual updating of secret metadata in Conjur.


## Tasks
* ### Add a struct for storing content-types of individual Conjur secrets (K8s Secrets)
  * Updated K8sProvider config which can store content-type of secrets
  * Marshal/unmarshal logic to correctly parse new YAML annotations
  * Add/update tests to check for the expected content-type of secrets
* ### Add/update a struct for storing content-types of individual Conjur secrets (P2F)
  * Updated P2FProviderConfig which can store content-type of secrets for each secret group
  * Marshal/unmarshal logic to correctly parse new YAML annotations
  * Add/update tests to check for the expected content-type of secrets
* ### Secrets with 'base64' content-types are decoded (K8s Secrets)
  * Decode secrets based on content-type
  * Update integration tests to include decoded variables and expected values
  * Update dev environments to allow easy demonstration of base64 decoding
* ### Secrets with 'base64' content-types are decoded (P2F)
  * Decode secrets based on content-type
  * Update integration tests to include decoded variables and expected values
  * Update dev environments to allow easy demonstration of base64 decoding
* ### Add e2e secrets decoding test and update manifests for the following cases:
  * K8s Secrets + rotation
  * Push-to-file + rotation
  * Decoding secrets > 65k characters (may be acceptable to document limits rather than adding a separate test)
* ### Create/update docs
  * Update existing docs and sample manifests where appropriate.
* ### Create artifact for tech writers
  * Create doc artifact to hand over to TW for updating the docs.
* ### Docs walkthrough
  * Manual UX testing
