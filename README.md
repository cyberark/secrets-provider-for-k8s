**Status**: GA

The CyberArk Secrets Provider for Kubernetes is currently in GA

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

- Conjur 5.4.8+ (?)
- DAP 11.1+ 
- Kubernetes 1.9+ (?)
- Openshift 3.9 and 3.11

# Releases

## Docker images

The primary source of CyberArk Secrets Provider for Kubernetes releases can be found in our (?)

## Github releases

# Development

We welcome contributions of all kinds to Cyberark Secrets Provider for Kubernetes. For instructions on
how to get started and descriptions of our development workflows, please see our
[contributing guide](CONTRIBUTING.md). 

# Maintainers

[Oren Ben Meir](https://github.com/orenbm)

[Nessi Lahav](https://github.com/nessiLahav)

[Sigal Sax](https://github.com/sigalsax)

[Moti Cohen](https://github.com/moticless)
 
[Roee Refael](https://github.com/rrefael)

# Community

Interested in checking out more of our open source projects? See our [open source repository](https://github.com/cyberark/)!

# License

The Cyberark Secrets Provider for Kubernetes is licensed Apache License 2.0 - see [`LICENSE.md`](licenses/LICENSE.md) for more details.