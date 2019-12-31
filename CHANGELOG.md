# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2019-12-26

### Changed
  - Using a new authn-client version that sends the full host-id in the CSR 
    request so we have this capability in this project. This enables users to
    authenticate with hosts that are defined anywhere in the policy tree.
  
## [0.2.0] - 2019-09-19

### Added
  - Logs
    - Logging in different log levels (info, debug, warn)
    - Capability to Enable debug logs via the env
    - More messages to increase UX and supportability
  - An end-to-end integration test
    
### Changed
  - Escape secrets with backslashes before patching in k8s
    