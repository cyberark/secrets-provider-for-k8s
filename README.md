# Table of Contents

- [CyberArk Secrets Provider for Kubernetes](#cyberArk-secrets-provider-for-kubernetes)
    - [Supported services](#supported-services)
- [Releases](#releases)
- [Development](#development)
- [Maintainers](#maintainers)
- [Community](#community)
- [License](#license)

# CyberArk Secrets Provider for Kubernetes

The CyberArk Secrets Provider for Kubernetes enables you to use secrets stored and managed in the CyberArk Vault 
using DAP and consume them in your Kubernetes application containers. To do so, the CyberArk Secrets 
Provider for Kubernetes image runs as an init container and provides the Conjur secrets, required by the pod, 
from DAP.

## Supported services

- DAP 11.1+
- Openshift 3.9 and 3.11

# Releases

The primary source of CyberArk Secrets Provider for Kubernetes releases is our Dockerhub.

Each time the `master` build is green, we push a `<version>-<git_version>` (i.e. `0.2.0-d9494c1`) image to our internal registry.

When we are releasing a version, we push the following images to our registry and then to Dockerhub.
1. Latest
2. Major.Minor.Build
3. Major.Minor
4. Major

## Stable release definition

The CyberArk Secrets Provider for Kubernetes is considered stable when it meets the core acceptance criteria:

- Documentation exists that clearly explains how to set up and use the provider as well as providing troubleshooting
information for anticipated common failure cases.
- A suite of tests exist that provides excellent code coverage and possible use cases.
- The CyberArk Secrets Provider for Kubernetes has had a security review and all known high and critical issues have been addressed.
Any low or medium issues that have not been addressed have been logged in the GitHub issue backlog with a label of the form `security/X`
- The CyberArk Secrets Provider for Kubernetes is easy to setup.
- The CyberArk Secrets Provider for Kubernetes is clear about known limitations and bugs if they exist.
- Anything else we consider to be a prerequisite for stability (?)
- Any more security standards? STRIDE threat modeling? (?)

# Development

We welcome contributions of all kinds to Cyberark Secrets Provider for Kubernetes. For instructions on
how to get started and descriptions of our development workflows, please see our
[contributing guide](CONTRIBUTING.md). 

# Maintainers

[Oren Ben Meir](https://github.com/orenbm)

[Nessi Lahav](https://github.com/nessiLahav)

[Sigal Sax](https://github.com/sigalsax)

[Moti Cohen](https://github.com/moticless)
 
[Dekel Asaf](https://github.com/tovli)

# Community

Interested in checking out more of our open source projects? See our [open source repository](https://github.com/cyberark/)!

# License

The Cyberark Secrets Provider for Kubernetes is licensed Apache License 2.0 - see [`LICENSE.md`](LICENSE) for more details.