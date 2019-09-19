# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2019-09-19

### Fixed

### Added

    - Logs
        - Logging in different log levels (info, debug, warn)
        - Capability to Enable debug logs via the env
        - More messages to increase UX and supportability
    - An end-to-end integration test
    
### Changed

    - Escape secrets with backslashes before patching in k8s
    