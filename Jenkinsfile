#!/usr/bin/env groovy
@Library("product-pipelines-shared-library") _

// Automated release, promotion and dependencies
properties([
  // Include the automated release parameters for the build
  release.addParams(),
  // Dependencies of the project that should trigger builds
  dependencies([
    'conjur-enterprise/conjur-opentelemetry-tracer',
    'conjur-enterprise/conjur-authn-k8s-client',
    'conjur-enterprise/conjur-api-go'
  ])
])

// Performs release promotion.  No other stages will be run
if (params.MODE == "PROMOTE") {
  release.promote(params.VERSION_TO_PROMOTE) { infrapool, sourceVersion, targetVersion, assetDirectory ->
    // Any assets from sourceVersion Github release are available in assetDirectory
    // Any version number updates from sourceVersion to targetVersion occur here
    // Any publishing of targetVersion artifacts occur here
    // Anything added to assetDirectory will be attached to the Github Release

    // Pull existing images from internal registry in order to promote
    infrapool.agentSh """
      export PATH="release-tools/bin:${PATH}"
      docker pull registry.tld/secrets-provider-for-k8s:${sourceVersion}
      docker pull registry.tld/secrets-provider-for-k8s-redhat:${sourceVersion}
      # Promote source version to target version.
      summon ./bin/publish --promote --source ${sourceVersion} --target ${targetVersion}
    """
  }

  // Copy Github Enterprise release to Github
  release.copyEnterpriseRelease(params.VERSION_TO_PROMOTE)
  return
}

pipeline {
  agent { label 'conjur-enterprise-common-agent' }

  options {
    timestamps()
    // We want to avoid running in parallel.
    // When we have 2 build running on the same environment (gke env only) in parallel,
    // we get the error "gcloud crashed : database is locked"
    disableConcurrentBuilds()
    buildDiscarder(logRotator(numToKeepStr: '30'))
    timeout(time: 3, unit: 'HOURS')
  }

  environment {
    // Sets the MODE to the specified or autocalculated value as appropriate
    MODE = release.canonicalizeMode()
  }

  parameters {
    booleanParam(name: 'TEST_OCP_NEXT', defaultValue: false, description: 'Run DAP tests against our running "next version" of Openshift')

    booleanParam(name: 'TEST_OCP_OLDEST', defaultValue: false, description: 'Run DAP tests against our running "oldest version" of Openshift')

    booleanParam(name: 'TEST_E2E', defaultValue: false, description: 'Run E2E tests on a branch')
  }

  stages {
    stage('Scan for internal URLs') {
      steps {
        script {
          detectInternalUrls()
        }
      }
    }

    stage('Get InfraPool ExecutorV2 Agent') {
      steps {
        script {
          // Request ExecutorV2 agents for 1 hour(s)
          INFRAPOOL_EXECUTORV2_AGENT_0 = getInfraPoolAgent.connected(type: "ExecutorV2", quantity: 1, duration: 2)[0]
        }
      }
    }

    // Aborts any builds triggered by another project that wouldn't include any changes
    stage ("Skip build if triggering job didn't create a release") {
      when {
        expression {
          MODE == "SKIP"
        }
      }
      steps {
        script {
          currentBuild.result = 'ABORTED'
          error("Aborting build because this build was triggered from upstream, but no release was built")
        }
      }
    }

    stage('Validate') {
      parallel {
        stage('Changelog') {
          steps { script { parseChangelog(INFRAPOOL_EXECUTORV2_AGENT_0) } }
        }
        stage('Log messages') {
          steps {
            validateLogMessages()
          }
        }
      }
    }

    // Generates a VERSION file based on the current build number and latest version in CHANGELOG.md
    stage('Validate Changelog and set version') {
      steps {
        updateVersion(INFRAPOOL_EXECUTORV2_AGENT_0, "CHANGELOG.md", "${BUILD_NUMBER}")
      }
    }

    stage('Get latest upstream dependencies') {
      steps {
        script {
          updatePrivateGoDependencies("${WORKSPACE}/go.mod")
          // Copy the vendor directory onto infrapool
          INFRAPOOL_EXECUTORV2_AGENT_0.agentPut from: "vendor", to: "${WORKSPACE}"
          INFRAPOOL_EXECUTORV2_AGENT_0.agentPut from: "go.*", to: "${WORKSPACE}"
          INFRAPOOL_EXECUTORV2_AGENT_0.agentPut from: "/root/go", to: "/var/lib/jenkins/"
        }
      }
    }

    stage('Build and test Secrets Provider') {
      when {
        // Run tests only when EITHER of the following is true:
        // 1. A non-markdown file has changed.
        // 2. It's the main branch.
        // 3. It's a version tag, typically created during a release
        anyOf {
          // Note: You cannot use "when"'s changeset condition here because it's
          // not powerful enough to express "_only_ md files have changed".
          // Dropping down to a git script was the easiest alternative.
          expression {
            0 == sh(
              returnStatus: true,
              // A non-markdown file has changed.
              script: '''
                git diff  origin/main --name-only |
                grep -v "^.*\\.md$" > /dev/null
              '''
            )
          }

          // Always run the full pipeline on main branch
          branch 'main'

          // Always run the full pipeline on a version tag created during release
          buildingTag()
        }
      }
      stages {
        stage('Build client Docker image') {
          steps {
            script {
              INFRAPOOL_EXECUTORV2_AGENT_0.agentSh './bin/build'
            }
          }
        }

        stage('Scan Docker Image') {
          parallel {
            stage("Scan Docker Image for fixable issues") {
              steps {
                // Adding the false parameter to scanAndReport causes trivy to
                // ignore vulnerabilities for which no fix is available. We'll
                // only fail the build if we can actually fix the vulnerability
                // right now.
                scanAndReport(INFRAPOOL_EXECUTORV2_AGENT_0, 'secrets-provider-for-k8s:latest', "HIGH", false)
              }
            }
            stage("Scan Docker image for total issues") {
              steps {
                // By default, trivy includes vulnerabilities with no fix. We
                // want to know about that ASAP, but they shouldn't cause a
                // build failure until we can do something about it. This call
                // to scanAndReport should always be left as "NONE"
                scanAndReport(INFRAPOOL_EXECUTORV2_AGENT_0, "secrets-provider-for-k8s:latest", "NONE", true)
              }
            }
            stage('Scan RedHat image for fixable issues') {
              steps {
                scanAndReport(INFRAPOOL_EXECUTORV2_AGENT_0, "secrets-provider-for-k8s-redhat:latest", "HIGH", false)
              }
            }
    
            stage('Scan RedHat image for all issues') {
              steps {
                scanAndReport(INFRAPOOL_EXECUTORV2_AGENT_0, "secrets-provider-for-k8s-redhat:latest", "NONE", true)
              }
            }
          }
        }

        stage('Run Unit Tests') {
          steps {
            script {
              INFRAPOOL_EXECUTORV2_AGENT_0.agentSh './bin/test_unit'
            }
          }
          post {
            always {
              script {
                INFRAPOOL_EXECUTORV2_AGENT_0.agentSh './bin/coverage'
                INFRAPOOL_EXECUTORV2_AGENT_0.agentStash name: 'coverage', includes: '*.xml'
                unstash 'coverage'
                junit 'junit.xml'
                cobertura autoUpdateHealth: false, autoUpdateStability: false, coberturaReportFile: 'coverage.xml', conditionalCoverageTargets: '70, 0, 0', failUnhealthy: false, failUnstable: false, maxNumberOfBuilds: 0, lineCoverageTargets: '70, 0, 0', methodCoverageTargets: '70, 0, 0', onlyStable: false, sourceEncoding: 'ASCII', zoomCoverageChart: false
                codacy action: 'reportCoverage', filePath: "coverage.xml"
              }
            }
          }
        }
        
        stage ("DAP Integration Tests on GKE") {
          when { anyOf {
            branch 'main'
            expression { params.TEST_E2E == true }
          } }
          steps {
            script {
              def tasks = [:]
              tasks["Kubernetes GKE, DAP"] = {
                INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "./bin/start --docker --dap --gke"
              }
              parallel tasks
            }
          }
        }

        stage ("DAP Integration Tests on OpenShift") {
          when {
            // Run integration tests against OpenShift only on the main branch
            //
            // There's been a lot of flakiness around OpenShift, which has the negative effect of impeding developer velocity.
            // Generally speaking the integration tests for this repository interact with the generic Kubernetes API, for
            // scheduling and giving identity to workloads. There is no platform-specifc functionality within the secrets provider.
            // We can reasonably assume that if a branch is green in GKE then it will likely be green for OpenShift.
            // With that in mind, for now we have chosen to run Openshift integration tests only on the main branch while we figure
            // out a better way to address the flakiness.
            branch 'main'
          }
          steps {
            script {
              def tasks = [:]
              if ( params.TEST_OCP_OLDEST ) {
                tasks["Openshift (Oldest), DAP"] = {
                  INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "./bin/start --docker --dap --oldest"
                }
              }
              tasks["Openshift (Current), DAP"] = {
                INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "./bin/start --docker --dap --current"
              }
              if ( params.TEST_OCP_NEXT ) {
                tasks["Openshift (Next), DAP"] = {
                  INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "./bin/start --docker --dap --next"
                }
              }
              parallel tasks
            }
          }
        }

        // We want to avoid running in parallel.
        // When we have 2 build running on the same environment (gke env only) in parallel,
        // we get the error "gcloud crashed : database is locked"
        stage ("OSS Integration Tests on GKE") {
          when { anyOf {
            branch 'main'
            expression { params.TEST_E2E == true }
          } }
          steps {
            script {
              def tasks = [:]
                tasks["Kubernetes GKE, oss"] = {
                  INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "./bin/start --docker --oss --gke"
                }
              parallel tasks
            }
          }
        }

        // Allows for the promotion of images.
        stage('Push images to internal registry') {
          steps {
            script {
              INFRAPOOL_EXECUTORV2_AGENT_0.agentSh './bin/publish --internal'
            }
          }
        }

        stage('Release') {
          when {
            expression {
              MODE == "RELEASE"
            }
          }
          parallel {
            stage('Push Images') {
              steps {
                script {
                  release(INFRAPOOL_EXECUTORV2_AGENT_0) { billOfMaterialsDirectory, assetDirectory, toolsDirectory ->
                    // Publish release artifacts to all the appropriate locations
                    // Copy any artifacts to assetDirectory to attach them to the Github release

                    // Create Go application SBOM using the go.mod version for the golang container image
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh """export PATH="${toolsDirectory}/bin:${PATH}" && go-bom --tools "${toolsDirectory}" --go-mod ./go.mod --image "golang" --main "cmd/secrets-provider/" --output "${billOfMaterialsDirectory}/go-app-bom.json" """
                    // Create Go module SBOM
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh """export PATH="${toolsDirectory}/bin:${PATH}" && go-bom --tools "${toolsDirectory}" --go-mod ./go.mod --image "golang" --output "${billOfMaterialsDirectory}/go-mod-bom.json" """
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh 'summon ./bin/publish --edge'
                  }
                }
              }
            }
            stage('Package artifacts') {
              steps {
                script {
                  INFRAPOOL_EXECUTORV2_AGENT_0.agentSh 'ci/jenkins_build'
                }

                archiveArtifacts artifacts: "helm-artifacts/", fingerprint: false, allowEmptyArchive: true
              }
            }
          }
        }
      }
    }
  }

  post {
    always {
      archiveArtifacts artifacts: "deploy/output/*.txt", fingerprint: false, allowEmptyArchive: true
      releaseInfraPoolAgent(".infrapool/release_agents")
    }
  }
}
