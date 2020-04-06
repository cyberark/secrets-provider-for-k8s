# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Bumped the authn-k8s client version to 0.16.1
  (cyberark/conjur-authn-k8s-client#70)

### Fixed
- Fixed issue with providing complex Conjur secrets (#77)

## [0.4.0] - 2020-01-23

### Changed
- Using a new conjur-authn-k8s-client version that enables authentication of
  hosts that have their application identity defined in annotations.

## [0.3.0] - 2019-12-26
### Changed
- Using a new authn-client version that sends the full host-id in the CSR  equest so we have this capability in this project. This enables users to authenticate with hosts that are defined anywhere in the policy tree.

## [0.2.0] - 2019-09-19

### Added
- Logs
  - Logging in different log levels (info, debug, warn)
  - Capability to Enable debug logs via the env
  - More messages to increase UX and supportability
  - An end-to-end integration test

### Changed
  - Escape secrets with backslashes before patching in k8s

[Unreleased]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cyberark/secrets-provider-for-k8s/releases/tag/v0.2.0
