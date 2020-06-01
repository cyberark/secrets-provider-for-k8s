# Table of Contents

- [Table of Contents](#table-of-contents)
- [CyberArk Secrets Provider for Kubernetes](#cyberark-secrets-provider-for-kubernetes)
  - [Supported services](#supported-services)
- [Releases](#releases)
  - [Stable release definition](#stable-release-definition)
- [Development](#development)
- [Documentation](#documentation)
- [Maintainers](#maintainers)
- [Community](#community)
- [License](#license)

# CyberArk Secrets Provider for Kubernetes

The CyberArk Secrets Provider for Kubernetes enables you to use secrets stored and managed in the CyberArk Vault 
using DAP or Conjur and consume them in your Openshift application containers. To do so, the CyberArk Secrets 
Provider for Kubernetes image runs as an init container and provides the Conjur secrets, required by the pod, 
from DAP or Conjur.

## Supported services

- Openshift 3.9, 3.10, and 3.11
- DAP 11.1+
- Conjur OSS v1.4.2+

# Releases

The primary source of CyberArk Secrets Provider for Kubernetes releases is our [Dockerhub](https://hub.docker.com/repository/docker/cyberark/secrets-provider-for-k8s).

When we release a version, we push the following images to Dockerhub:
1. Latest
1. Major.Minor.Build
1. Major.Minor
1. Major

In addition, each time the `master` build is green, we push a `<version>-<git_version>` (i.e. `0.2.0-d9494c1`) image to our internal repo.

## Stable release definition

The CyberArk Secrets Provider for Kubernetes is considered stable when it meets the core acceptance criteria:

- Documentation exists that clearly explains how to set up and use the provider and includes troubleshooting information to resolve common issues.
- A suite of tests exist that provides excellent code coverage and possible use cases.
- The CyberArk Secrets Provider for Kubernetes has had a security review and all known high and critical issues have been addressed.
Any low or medium issues that have not been addressed have been logged in the GitHub issue backlog with a label of the form `security/X`
- The CyberArk Secrets Provider for Kubernetes is easy to setup.
- The CyberArk Secrets Provider for Kubernetes is clear about known limitations and bugs, if they exist.

# Development

We welcome contributions of all kinds to Cyberark Secrets Provider for Kubernetes. For instructions on
how to get started and descriptions of our development workflows, see our [contributing guide](CONTRIBUTING.md).

# Documentation
You can find official documentation on [our site](https://docs.conjur.org/Latest/en/Content/Integrations/Kubernetes_deployApplicationsConjur-k8s-Secrets.htm)

# Maintainers

[Oren Ben Meir](https://github.com/orenbm)

[Nessi Lahav](https://github.com/nessiLahav)

[Sigal Sax](https://github.com/sigalsax)

[Moti Cohen](https://github.com/moticless)
 
[Dekel Asaf](https://github.com/tovli)

[Inbal Zilberman](https://github.com/InbalZilberman)

# Community

Interested in checking out more of our open source projects? See our [open source repository](https://github.com/cyberark/)!

# License

The Cyberark Secrets Provider for Kubernetes is licensed under the Apache License 2.0 - see [`LICENSE`](LICENSE.md) for more details.
