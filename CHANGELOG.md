# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2021-11-30

### Added
- Adds validation for output filepaths and names in Push-to-File, requiring
  valid Linux filenames that are unique across all secret groups.
  [cyberark/secrets-provider-for-k8s#386](https://github.com/cyberark/secrets-provider-for-k8s/pull/386)
- Adds support for Push-to-File annotation `conjur.org/conjur-secrets-policy-path.{secret-group}`.
  [cyberark/secrets-provider-for-k8s#392](https://github.com/cyberark/secrets-provider-for-k8s/pull/392)

### Changed
- Push-to-File supports more intuitive output filepaths. Filepaths are
  no longer required to contain the hard-coded mount path `/conjur/secrets`, and
  can specify intermediate directories.
  [cyberark/secrets-provider-for-k8s#381](https://github.com/cyberark/secrets-provider-for-k8s/pull/381)

## [1.1.6] - 2021-10-29

### Added
- Adds support for Secrets Provider M1 Push-to-File feature, Community release.
  [cyberark/secrets-provider-for-k8s#358](https://github.com/cyberark/secrets-provider-for-k8s/pull/358)
  [cyberark/secrets-provider-for-k8s#359](https://github.com/cyberark/secrets-provider-for-k8s/pull/359)
  [cyberark/secrets-provider-for-k8s#362](https://github.com/cyberark/secrets-provider-for-k8s/pull/362)
  [cyberark/secrets-provider-for-k8s#363](https://github.com/cyberark/secrets-provider-for-k8s/pull/363)
  [cyberark/secrets-provider-for-k8s#364](https://github.com/cyberark/secrets-provider-for-k8s/pull/364)
  [cyberark/secrets-provider-for-k8s#366](https://github.com/cyberark/secrets-provider-for-k8s/pull/366)
  [cyberark/secrets-provider-for-k8s#367](https://github.com/cyberark/secrets-provider-for-k8s/pull/367)
  [cyberark/secrets-provider-for-k8s#368](https://github.com/cyberark/secrets-provider-for-k8s/pull/368)
  [cyberark/secrets-provider-for-k8s#376](https://github.com/cyberark/secrets-provider-for-k8s/pull/376)
  [cyberark/secrets-provider-for-k8s#377](https://github.com/cyberark/secrets-provider-for-k8s/pull/377)
  [cyberark/secrets-provider-for-k8s#378](https://github.com/cyberark/secrets-provider-for-k8s/pull/378)
- Support for OpenShift 4.8 has been added.
  [cyberark/secrets-provider-for-k8s#360](https://github.com/cyberark/secrets-provider-for-k8s/pull/360)

## [1.1.5] - 2021-08-13

### Added
- Adds Helm chart option to use an independently installed Conjur Connect
  ConfigMap instead of configuring Conjur connection parameters via environment
  variables.
  [cyberark/secrets-provider-for-k8s#349](https://github.com/cyberark/secrets-provider-for-k8s/pull/349)
- Adds Helm chart option to explicitly set the Secrets Provider Job name.
  [cyberark/secrets-provider-for-k8s#352](https://github.com/cyberark/secrets-provider-for-k8s/pull/352)

### Security
- Upgrades base Alpine image used for Secrets Provider container image to
  v3.14 to resolve CVE-2021-36159.
  [cyberark/secrets-provider-for-k8s#354](https://github.com/cyberark/secrets-provider-for-k8s/pull/354)

## [1.1.4] - 2021-06-30

### Changed
- Update RH base image to `ubi8/ubi` instead of `rhel7/rhel`.
  [PR cyberark/secrets-provider-for-k8s#328](https://github.com/cyberark/secrets-provider-for-k8s/pull/328)

## [1.1.3] - 2021-03-01

### Added
- Verified compatibility for OpenShift 4.6.
  [cyberark/secrets-provider-for-k8s#265](https://github.com/cyberark/secrets-provider-for-k8s/issues/302)
- Support for OpenShift 4.7 has been certified as of this release.

### Changed
- Updated k8s authenticator client version to
  [0.19.1](https://github.com/cyberark/conjur-authn-k8s-client/blob/master/CHANGELOG.md#0191---2021-02-08),
  which streamlines the parsing of authentication responses, updates the
  project Golang version to v1.15, and improves error messaging.

## [1.1.2] - 2020-01-29

### Added
- Support for OpenShift 4.5.
  [cyberark/secrets-provider-for-k8s#265](https://github.com/cyberark/secrets-provider-for-k8s/issues/265)

### Fixed
- The Secrets Provider helm templates are updated to correctly refer to
  `Release.Namespace` instead of `Release.namespace`. Previously, the namespace
  value wasn't being interpolated correctly because its name is case sensitive.
  [cyberark/secrets-provider-for-k8s#290](https://github.com/cyberark/secrets-provider-for-k8s/issues/290)

### Deprecated
- Support for OpenShift 3.9 and 3.10 is officially removed as of this release.
  [cyberark/secrets-provider-for-k8s#265](https://github.com/cyberark/secrets-provider-for-k8s/issues/265)

### Security
- Updated gogo/protobuf to v1.3.2 to address CVE-2021-3121.
  [cyberark/secrets-provider-for-k8s#285](https://github.com/cyberark/secrets-provider-for-k8s/pull/285)

## [1.1.1] - 2020-11-24
### Added
- An `edge` tag is published for every successful main build.
  [cyberark/secrets-provider-for-k8s#234](https://github.com/cyberark/secrets-provider-for-k8s/pull/234)

### Changed
- Uses logger from k8s authenticator client; its timestamp format contains milliseconds precision.
  [cyberark/secrets-provider-for-k8s#221](https://github.com/cyberark/secrets-provider-for-k8s/issues/221)
- Update k8s authenticator client version to
  [0.19.0](https://github.com/cyberark/conjur-authn-k8s-client/blob/master/CHANGELOG.md#0190---2020-10-08),
  which adds some fixes around cert injection failure (see also changes in
  [0.18.1](https://github.com/cyberark/conjur-authn-k8s-client/blob/master/CHANGELOG.md#0181---2020-09-13)).
  [cyberark/secrets-provider-for-k8s#247](https://github.com/cyberark/secrets-provider-for-k8s/pull/247)

### Fixed
- The version that is printed at the product's startup now includes the git commit
  hash instead of a hard-coded 'dev' string.
  [cyberark/secrets-provider-for-k8s#256](https://github.com/cyberark/secrets-provider-for-k8s/issues/256)

## [1.1.0] - 2020-09-15
### Added
- Helm chart to deploy the Secrets Provider Job using Helm.
  [cyberark/secrets-provider-for-k8s#165](https://github.com/cyberark/secrets-provider-for-k8s/pull/165)
- CONTAINER_MODE support for application containers in addition to init containers.
  [cyberark/secrets-provider-for-k8s#179](https://github.com/cyberark/secrets-provider-for-k8s/pull/179)
- Red Hat certified version of the secrets-provider-for-k8s image.
  [cyberark/secrets-provider-for-k8s#93](https://github.com/cyberark/secrets-provider-for-k8s/pull/93)

### Changed
- Bumped the authn-k8s client version to 0.18.1
  [cyberark/conjur-authn-k8s-client#223](https://github.com/cyberark/conjur-authn-k8s-client/issues/223)
- The retry backoff for the Secrets Provider is now constant instead of exponential ([cyberark/secrets-provider-for-k8s#174](https://github.com/cyberark/secrets-provider-for-k8s/issues/174))
- The secrets-provider-for-k8s now runs as a limited user in the Docker image
  instead of as root. This is considered a best security practice because it abides by the principle of least privilege
  [cyberark/secrets-provider-for-k8s#95](https://github.com/cyberark/secrets-provider-for-k8s/pull/95)

## [1.0.0] - 2020-05-19
### Changed
- Bumped the authn-k8s client version to 0.16.1
  [cyberark/conjur-authn-k8s-client#70](https://github.com/cyberark/conjur-authn-k8s-client/issues/70)

### Fixed
- Fixed issue with providing complex Conjur secrets. The secrets-provider
  now updates k8s secrets using `update` instead of `patch` so the service-account
  needs to have that permission [cyberark/secrets-provider-for-k8s#79](https://github.com/cyberark/secrets-provider-for-k8s/issues/79)

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

[Unreleased]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.1.6...v1.2.0
[1.1.6]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.1.5...v1.1.6
[1.1.5]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.1.4...v1.1.5
[1.1.4]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.1.3...v1.1.4
[1.1.3]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v0.4.0...v1.0.0
[0.4.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cyberark/secrets-provider-for-k8s/releases/tag/v0.2.0
