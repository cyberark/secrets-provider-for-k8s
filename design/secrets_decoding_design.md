# Secrets Provider - Base64 Secrets Decoding Design

## Table of Contents

- [Secrets Provider - Base64 Secrets Decoding Design](#secrets-provider---base64-secrets-decoding-design)
  * [Table of Contents](#table-of-contents)
  * [Useful links](#useful-links)
  * [Background](#background)
  * [Solution](#solution)
    + [Design](#design)
    + [Backwards compatibility](#backwards-compatibility)
  * [Test Plan](#test-plan)
    + [Annotation Tests](#annotation-tests)
    + [Decoding Tests](#decoding-tests)
    + [e2e Tests](#e2e-tests)
    + [Audit](#audit)
  * [Documentation](#documentation)
  * [Open questions](#open-questions)
  * [Tasks](#tasks)
    + [Add a struct for storing content-types of individual Conjur secrets](#add-a-struct-for-storing-content-types-of-individual-conjur-secrets)
    + [Update K8s Secret mode config to handle pod annotations](#update-k8s-secret-mode-config-to-handle-pod-annotations)
    + [Add decoding of secrets with 'base64' content-types](#add-decoding-of-secrets-with--base64--content-types)
    + [Update e2e tests and dev environment scripts/manifests](#update-e2e-tests-and-dev-environment-scripts-manifests)
    + [Create/update docs](#create-update-docs)
    + [Docs walkthrough](#docs-walkthrough)

<small><i><a href='http://ecotrust-canada.github.io/markdown-toc/'>Table of contents generated with markdown-toc</a></i></small>

## Useful links

| Name                                     | Link                                                         |
| -----------------------------------------| ------------------------------------------------------------ |
| Secrets Provider Configuration Reference | https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ref.htm |
| Feature Functional Spec (internal)       | https://{cyberark_confluence_host}/display/rndp/Secrets+Provider%3A+Secrets+decoding+for+k8s+Functional+Specification |

## Background
Secrets Provider provides Kubernetes-based applications with access to secrets that are stored and managed in Conjur. These secrets can be any binary/string data. In some cases it may be beneficial, or necessary, to store secret values in Conjur as base64 encoded strings depending on the nature and origin of the secret. Vault also supports multiline and base64 encoded strings which may then be synchronized as Conjur secrets in their encoded state.

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
      path: policy/path/base64_user
      content-type: base64
```

### P2F Mode
Kubernetes Pod annotations are currently used to define which Conjur secrets are to be retrieved when using P2F (push-to-file) mode. [See annotations reference](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/Latest/en/Content/Integrations/k8s-ocp/cjr-k8s-secrets-provider-ref.htm?tocpath=Integrations%7COpenShift%252FKubernetes%7CApp%20owner%253A%20Set%20up%20workloads%20in%20Kubernetes%7CSet%20up%20workloads%20(JWT-based%20authn)%7CSecrets%20Provider%20for%20Kubernetes%7C_____3#AdditionalannotationsforPushtoFilemode). The schema for the `conjur.org/conjur-secrets.{secret-group}` annotations can be extended to include an optional content-type attribute. This could look like:
```
conjur.org/conjur-secrets.groupname: |
  - user: policy/path/base64_user     # valid annotation with content-type
    content-type: base64
  - policy/path/base64_password:      # this is also valid when no alias is needed
      content-type: base64
```

P2F already leverages Kubernetes' Downward API and volumeMounts to pass these annotations into the Secrets Provider container. This will make it possible to determine which secrets should be decoded before pushing them to the Kubernetes secret file. 

## Design
The goal of this design is to make usage of this feature as consistent as possible across different configurations. Due to the nature of configuring Conjur secrets in K8s secrets mode versus P2F mode, it makes the most sense to add the configuration in-place where Conjur secrets are already defined.
* Kubernetes Secrets mode - add content-type to the existing K8s secrets manifest
* P2F mode - add content-type to the existing pod annotations

In both modes the solution will need to be backwards compatible with existing annotation formats. 

The logic and structs should added for this feature should be shared between both modes when possible. For example, P2F mode uses [SecretSpec struct](../pkg/secrets/common/secret_spec.go) which could be updated with a ContentType field. Ideally both YAML formats can be unmarshaled into a common struct since they will contain the same data:
* Alias
* Path
* ContentType
  * Allowed values = ['text', 'base64']
  * Default = 'text'

### Kubernetes Secrets Mode
In order to keep Conjur secrets configuration from creeping into other locations, we will add the content-type under the existing `conjur-map` annotation of the secrets manifest. This requires adding an additional supported multiline format:
```
conjur-map: |-
  <alias_value>: <path_value>                 # supported
  <alias_value>:                              # to be added
    path: <path_value>
    content-type: <content_type_value>    
```

### P2F Mode
For P2F, we can update the existing `conjur.org/conjur-secrets.{secret-group}` annotations with the following allowed formats:
```
conjur.org/conjur-secrets.groupname: |
  - <alias_value>: <path_value>               # supported
  - <path_value>                              # supported
  - <alias_value>: <path_value>               # to be added
    content-type: <content_type_value>
  - <path_value>:                             # to be added
      content-type: <content_type_value>
```

### Backwards compatibility
This enhancement will not modify the default behavior of Secrets Provider. It will continue to assume that secrets should be delivered "as-is" unless it specifically finds a content-type annotation which indicates that the secret value should be decoded.

## Test Plan
### Annotation Tests (K8s Secrets)
Ensure that the 'base64' content-type of a Secret is captured for valid configuration:
```
conjur-map: |-
  user: 
    path: policy/path/base64_user
    content-type: base64
```

Ensure that the 'text' content-type of a Secret is captured by default or when specified:
```
conjur-map: |-
  user: 
    path: policy/path/base64_user
    content-type: text
```
```
conjur-map: |-
  user: policy/path/user
```

Unit tests should also be implemented for error messages that result from invalid annotations, parsing errors, etc.

### Annotation Tests (P2F)
Ensure that the 'base64' content-type of a Secret is captured for each valid configuration:
```
conjur.org/conjur-secrets.groupname: |
  - user: policy/path/base64_user     # valid annotation with content-type
    content-type: base64
```
```
conjur.org/conjur-secrets.groupname: |
  - policy/path/base64_password:      # this is also valid (no alias)
      content-type: base64
```

Ensure that the 'text' content-type of a Secret is captured by default or when specified:
```
conjur.org/conjur-secrets.groupname: |
  - user: policy/path/user            # explicit 'text' content-type is valid
    content-type: text
```
```
conjur.org/conjur-secrets.groupname: |
  - policy/path/text_secret:          # explicit 'text' content-type is valid (no alias)
  content-type: text
```
```
conjur.org/conjur-secrets.groupname: |
  - url: policy/path/url              # will default to 'text' content type
  - policy/path/some_other_url        # will default to 'text' content type
```

Unit tests should also be implemented for error messages that result from invalid annotations, parsing errors, etc.

### Decoding Tests
Unit tests for any helper functions added to assist with decoding secrets.

Update integration tests in both [K8s secrets mode](../pkg/secrets/k8s_secrets_storage/provide_conjur_secrets_test.go) and [P2F mode](../pkg/secrets/pushtofile/provide_conjur_secrets_test.go) to validate that an encoded variable with base64 content-type is decoded.

### e2e Tests
At least one e2e test should be added for validating that an encoded secret with valid annotations is provided and decoded to match the expected value.

### Audit 
The solution should produce an audit entry for each secret that was decoded as a result of the secrets configuration.

## Documentation
Since this feature is activated via the manifest, it would be reasonable to add a note to any sample manifests in existing documentation about the optional 'content-type' field for secret annotations. Both K8s Secrets and P2F documentation should have a brief mention of where and how configuration is needed to activate the feature respectively.

## Open questions
* Conjur variables support a mime-type attribute and custom annotations. See: [Conjur policy variable attributes](https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-DAP/12.4/en/Content/Operations/Policy/statement-ref-variable.htm#Attributes). Either of these could potentially be used to determine that an individual Conjur secret is encoded. This likely wouldn't be compatible with the current implementation using Conjur API's batch retrieval endpoint, but it would allow the decoding to be handled in the API instead of Secrets Provider. Could also have compatility issues with secrets that come from synchronizer and/or require manual updating of secret metadata in Conjur.


## Tasks
* ### Add a struct for storing content-types of individual Conjur secrets (K8s Secrets)
  * Add marshal/unmarshal logic to correctly parse YAML annotations into the new struct.
  * Add/update tests for the new struct to include the expected content-type of secrets.
* ### Add/update a struct for storing content-types of individual Conjur secrets (P2F)
  * If possible, reuse the struct from K8s secrets mode
  * Add marshal/unmarshal logic to correctly parse YAML annotations into the struct.
  * Add/update tests for the struct to include the expected content-type of secrets.
* ### Add decoding of secrets with 'base64' content-types
  * Add logic to decode secrets based on content-type
  * Likely want to handle this in or just beneath the Provide() function for each provider type (k8s_secrets and p2f)
  * Unit tests for any helper functions added here
  * Update integration tests to include decoded variables and expected values
* ### Update e2e tests and dev environment scripts/manifests
  * We should have at least one e2e test verifying a secret being decoded via this feature
  * Dev environments for both K8s secrets and P2F mode should include a decoded secret example
* ### Create/update docs
  * Update existing docs and sample manifests where appropriate
* ### Docs walkthrough
  * Manual UX testing
