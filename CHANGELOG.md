# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.7.5] - 2025-09-09

### Changed
- Updated documentation to align with Conjur Enterprise name change to Secrets Manager. (CNJR-10982)

## [1.7.4] - 2025-04-01

### Security
- Upgrade Go dependencies

## [1.7.3] - 2025-01-10

### Security
- Upgrade multiple dependencies

## [1.7.2] - 2024-12-30

### Security
- Upgrade golang.org/x/net to 0.33.0 to address CVE-2024-45338

## [1.7.1] - 2024-12-04

### Fixed
- The sentinel file is updated correctly when multiple K8s secrets are defined so the
  liveness probe container restart behaves as expected in K8s secrets mode (CNJR-7253)

## [1.7.0] - 2024-11-07

### Added
- Added `properties` file template (CNJR-6964,
  [cyberark/secrets-provider-for-k8s#548](https://github.com/cyberark/secrets-provider-for-k8s/pull/548))
- Support fetching all secrets available to host (CNJR-6716)
- Updated Go to 1.23

## [1.6.5] - 2024-07-24

### Security
- Upgrade golang.org/x/net to v0.24.0 (CONJSE-1863)

## [1.6.4] - 2024-04-08

### Changed
- Testing and CI improvements (CNJR-4550)

## [1.6.3] - 2024-03-21

### Changed
- Use updated RedHat preflight scan tool v1.9.1 (CNJR-3914)
- Updated Go to 1.22 (CONJSE-1842)

## [1.6.2] - 2024-03-20

### Security
- Replace google.golang.org/grpc@v1.27.0, golang.org/x/crypto@v0.14.0,
  google.golang.org/protobuf@v1.31.0, and github.com/mattn/go-sqlite3@v1.14.15
  to eliminate vulnerabilities (CNJR-3914)

## [1.6.1] - 2023-07-27

### Security
- Updated go to 1.20, alpine to latest, and redhat UBI to ubi9 in main Dockerfile
  [cyberark/secrets-provider-for-k8s#541](https://github.com/cyberark/secrets-provider-for-k8s/pull/541)

## [1.6.0] - 2023-07-19
### Added
- Log level is now configurable using the `LOG_LEVEL` environment variable or `conjur.org/log-level` annotation.
  The existing `DEBUG` environment variable and `conjur.org/debug-logging` annotation is deprecated and will be removed in a future update.
  [cyberark/secrets-provider-for-k8s#534](https://github.com/cyberark/secrets-provider-for-k8s/pull/534)

### Security
- Upgrade google/cloud-sdk to v437.0.0-slim
  [cyberark/secrets-provider-for-k8s#533](https://github.com/cyberark/secrets-provider-for-k8s/pull/533)
- Upgrade google/cloud-sdk to v435.0.1 and google.golang.org/protobuf to v1.29.1
  [cyberark/secrets-provider-for-k8s#531](https://github.com/cyberark/secrets-provider-for-k8s/pull/531)

## [1.5.1] - 2023-05-26

### Security
- Forced github.com/emicklei/go-restful/v3 to use v3.10.2 to remove PRISMA-2022-0227 (found in Twistlock scan)
  and updated versions of gotelemetry.io/otel (to 1.16.0), github.com/stretchr/testify (to 1.8.3), and
  the k8s.io libraries (to 0.27.2)
  [cyberark/secrets-provider-for-k8s#526](https://github.com/cyberark/secrets-provider-for-k8s/pull/526)

## [1.5.0] - 2023-04-12

### Added
- Convert cmd/secrets-provider to unit testable entrypoint package.
  [cyberark/secrets-provider-for-k8s#507](https://github.com/cyberark/secrets-provider-for-k8s/pull/507)
- Adds support for binary secret values and values with special characters.
  [cyberark/secrets-provider-for-k8s#500](https://github.com/cyberark/secrets-provider-for-k8s/pull/500)
- Adds support for content-type annotation and base64 secrets decoding feature.
  [cyberark/secrets-provider-for-k8s#508](https://github.com/cyberark/secrets-provider-for-k8s/pull/508)
  [cyberark/secrets-provider-for-k8s#511](https://github.com/cyberark/secrets-provider-for-k8s/pull/511)
  [cyberark/secrets-provider-for-k8s#513](https://github.com/cyberark/secrets-provider-for-k8s/pull/513)
  [cyberark/secrets-provider-for-k8s#512](https://github.com/cyberark/secrets-provider-for-k8s/pull/512)
  [cyberark/secrets-provider-for-k8s#509](https://github.com/cyberark/secrets-provider-for-k8s/pull/509)
  [cyberark/secrets-provider-for-k8s#504](https://github.com/cyberark/secrets-provider-for-k8s/pull/504)
  [cyberark/secrets-provider-for-k8s#506](https://github.com/cyberark/secrets-provider-for-k8s/pull/506)
- Use Conjur CLI v8.0.
  [cyberark/secrets-provider-for-k8s#505](https://github.com/cyberark/secrets-provider-for-k8s/pull/505)
- Add ImagePullSecret to Helm deployment.
  [cyberark/secrets-provider-for-k8s#503](https://github.com/cyberark/secrets-provider-for-k8s/pull/503)

## [1.4.6] - 2023-01-26

### Security
- Updated replace statements in go.mod to remove vulnerable versions of golang.org/x/net
  [cyberark/secrets-provider-for-k8s#496](https://github.com/cyberark/secrets-provider-for-k8s/pull/496)

## [1.4.5] - 2022-09-26
### Changed
- Updated Go to 1.19
  [cyberark/secrets-provider-for-k8s#484](https://github.com/cyberark/secrets-provider-for-k8s/pull/484)
- Updated go.opentelmetry.io/otel to 1.10.0 and k8s.io/api, k8s.io/apimachinery,
  and k8s.io/client-go to latest versions
  [cyberark/secrets-provider-for-k8s#484](https://github.com/cyberark/secrets-provider-for-k8s/pull/484)

### Security
- More replace statements for golang.org/x/crypto, gopkg.in/yaml.v2, and golang.org/x/net
  [cyberark/secrets-provider-for-k8s#486](https://github.com/cyberark/secrets-provider-for-k8s/pull/486)
- Updated replace statements in go.mod to remove vulnerable versions of golang.org/x/net
  [cyberark/secrets-provider-for-k8s#484](https://github.com/cyberark/secrets-provider-for-k8s/pull/484)
  [cyberark/secrets-provider-for-k8s#485](https://github.com/cyberark/secrets-provider-for-k8s/pull/485)
- Updated replace statements in go.mod to remove vulnerable versions of golang.org/x/text
  [cyberark/secrets-provider-for-k8s#484](https://github.com/cyberark/secrets-provider-for-k8s/pull/488)

## [1.4.4] - 2022-07-12
### Changed
- Updated multiple go dependencies
  [cyberark/secrets-provider-for-k8s#477](https://github.com/cyberark/secrets-provider-for-k8s/pull/477)

### Security
- Add replace statements to go.mod to prune vulnerable dependency versions from the dependency tree.
  [cyberark/secrets-provider-for-k8s#478](https://github.com/cyberark/secrets-provider-for-k8s/pull/478)

### Fixed
- Fixes the following error seen on boot up when the status volumemount is not added
  "open /conjur/status/conjur-secrets-unchanged.sh: no such file or directory"
  [cyberark/secrets-provider-for-k8s#479](https://github.com/cyberark/secrets-provider-for-k8s/pull/479)

## [1.4.3] - 2022-07-07
### Removed
- Support for OpenShift v3.11 is officially removed as of this release.
  [cyberark/secrets-provider-for-k8s#474](https://github.com/cyberark/secrets-provider-for-k8s/pull/474)

### Security
- Add replace statements to go.mod to prune vulnerable dependency versions from the dependency tree.
  [cyberark/secrets-provider-for-k8s#470](https://github.com/cyberark/secrets-provider-for-k8s/pull/470)
  [cyberark/secrets-provider-for-k8s#471](https://github.com/cyberark/secrets-provider-for-k8s/pull/471)
- Update the Red Hat ubi image in Dockerfile.
  [cyberark/secrets-provider-for-k8s#469](https://github.com/cyberark/secrets-provider-for-k8s/pull/469)

## [1.4.2] - 2022-05-03
### Changed
- Updated dependencies in go.mod (github.com/stretchr/testify -> v1.7.2, go.opentelemetry.io/otel -> 1.7.0,
  gopkg.in/yaml.v3 -> v3.0.1, k8s.io/api -> 0.24.1, k8s.io/apimachinery -> 0.24.1, k8s.io/client-go -> 0.24.1).
  [cyberark/secrets-provider-for-k8s#468](https://github.com/cyberark/secrets-provider-for-k8s/pull/468)

## [1.4.1] - 2022-04-01
### Changed
- Update to automated release process. [cyberark/secrets-provider-for-k8s#455](https://github.com/cyberark/secrets-provider-for-k8s/pull/455)

### Added
- Secrets files are written in an atomic operation. [cyberark/secrets-provider-for-k8s#440](https://github.com/cyberark/secrets-provider-for-k8s/pull/440)
- Secret files are deleted when secrets are removed from Conjur or access is revoked. Can be disabled with annotation.
  [cyberark/secrets-provider-for-k8s#447](https://github.com/cyberark/secrets-provider-for-k8s/pull/447)
- Kubernetes Secrets are cleared when secrets are removed from Conjur or access is revoked. Can be disabled with annotation.
  [cyberark/secrets-provider-for-k8s#449](https://github.com/cyberark/secrets-provider-for-k8s/pull/449)
- Secrets Provider allows for its status to be monitored through the creation of a couple of empty sentinel files: `CONJUR_SECRETS_PROVIDED` and `CONJUR_SECRETS_UPDATED`. The first file is created when SP has completed its first round of providing secrets via secret files / Kubernetes Secrets. It creates/recreates the second file whenever it has updated secret files / Kubernetes Secrets. If desirable, application containers can mount these files via a shared volume.
  [cyberark/secrets-provider-for-k8s#450](https://github.com/cyberark/secrets-provider-for-k8s/pull/450)
- Adds support for secrets rotation with Kubernetes Secrets.
  [cyberark/secrets-provider-for-k8s#448](https://github.com/cyberark/secrets-provider-for-k8s/pull/448)

## [1.4.0] - 2022-02-15

### Added
- Adds support for Secrets Provider secrets rotation feature, Community release.
  [cyberark/secrets-provider-for-k8s#426](https://github.com/cyberark/secrets-provider-for-k8s/pull/426)
  [cyberark/secrets-provider-for-k8s#432](https://github.com/cyberark/secrets-provider-for-k8s/pull/432)
- Adds support for Authn-JWT.
  [cyberark/secrets-provider-for-k8s#431](https://github.com/cyberark/secrets-provider-for-k8s/pull/431)
  [cyberark/secrets-provider-for-k8s#433](https://github.com/cyberark/secrets-provider-for-k8s/pull/433)

## [1.3.0] - 2022-01-03

### Added
- Push-to-File supports default filepaths for templates. [cyberark/secrets-provider-for-k8s#411](https://github.com/cyberark/secrets-provider-for-k8s/pull/411)
- Push-to-File supports custom file permissions for secret files. [cyberark/secrets-provider-for-k8s#408](https://github.com/cyberark/secrets-provider-for-k8s/pull/408)
- Adds support for tracing with OpenTelemetry. [cyberark/secrets-provider-for-k8s#398](https://github.com/cyberark/secrets-provider-for-k8s/pull/398)
- Adds support for Base64 encode/decode functions in custom templates. [cyberark/secrets-provider-for-k8s#409](https://github.com/cyberark/secrets-provider-for-k8s/pull/409)
- Secrets Provider run in Push-to-File mode can use secret file templates
  defined in a volume-mounted ConfigMap.
  [cyberark/secrets-provider-for-k8s#393](https://github.com/cyberark/secrets-provider-for-k8s/pull/393)

### Changed
- Secrets Provider run in Push-to-File mode using a custom secret file template
  requires annotation `conjur.org/secret-file-format.{secret-group}` to be set
  to `template`. This is a breaking change.
  [cyberark/secrets-provider-for-k8s#393](https://github.com/cyberark/secrets-provider-for-k8s/pull/393)

### Fixed
- If the Secrets Provider is run in Push-to-File mode, it no longer errors out
  if it finds any pre-existing secret files. This is helpful when the Secrets
  Provider is being run multiple times.
  [cyberark/secrets-provider-for-k8s#397](https://github.com/cyberark/secrets-provider-for-k8s/pull/397)
- If the Secrets Provider is run in Push-to-File mode, it no longer errors out
  if either (a) multiple secret groups use the same secret path, or (b) there
  are no secrets that need to be retrieved.
  [cyberark/secrets-provider-for-k8s#404](https://github.com/cyberark/secrets-provider-for-k8s/pull/404)

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

[Unreleased]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.7.5...HEAD
[1.7.5]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.7.4...v1.7.5
[1.7.4]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.7.3...v1.7.4
[1.7.3]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.7.2...v1.7.3
[1.7.2]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.7.1...v1.7.2
[1.7.1]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.6.5...v1.7.0
[1.6.5]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.6.4...v1.6.5
[1.6.4]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.6.3...v1.6.4
[1.6.3]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.6.2...v1.6.3
[1.6.2]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.6.1...v1.6.2
[1.6.1]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.5.1...v1.6.0
[1.5.1]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.4.6...v1.5.0
[1.4.6]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.4.5...v1.4.6
[1.4.5]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.4.4...v1.4.5
[1.4.4]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.4.3...v1.4.4
[1.4.3]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.4.2...v1.4.3
[1.4.2]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.4.1...v1.4.2
[1.4.1]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/cyberark/secrets-provider-for-k8s/compare/v1.2.0...v1.3.0
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
