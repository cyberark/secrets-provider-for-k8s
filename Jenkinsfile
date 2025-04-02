#!/usr/bin/env groovy
@Library("product-pipelines-shared-library") _

def productName = 'Secrets Provider for Kubernetes'
def productTypeName = 'Conjur Enterprise'

// Automated release, promotion and dependencies
properties([
  // Include the automated release parameters for the build
  release.addParams(),
  // Dependencies of the project that should trigger builds
  dependencies([
    'conjur-enterprise/conjur-opentelemetry-tracer',
    'conjur-enterprise/conjur-authn-k8s-client',
    'conjur-enterprise/conjur-api-go',
    'conjur-enterprise/conjur'
  ])
])

// Performs release promotion.  No other stages will be run
if (params.MODE == "PROMOTE") {
  release.promote(params.VERSION_TO_PROMOTE) { infrapool, sourceVersion, targetVersion, assetDirectory ->
    // Any assets from sourceVersion Github release are available in assetDirectory
    // Any version number updates from sourceVersion to targetVersion occur here
    // Any publishing of targetVersion artifacts occur here
    // Anything added to assetDirectory will be attached to the Github Release

    env.INFRAPOOL_PRODUCT_NAME = "${productName}"
    env.INFRAPOOL_DD_PRODUCT_TYPE_NAME = "${productTypeName}"

    def scans = [:]

    scans["Scan main Docker image"] = {
      runSecurityScans(infrapool,
        image: "registry.tld/secrets-provider-for-k8s:${sourceVersion}",
        buildMode: params.MODE,
        branch: env.BRANCH_NAME)
    }

    scans["Scan redhat Docker image"] = {
      runSecurityScans(infrapool,
      image: "registry.tld/secrets-provider-for-k8s-redhat:${sourceVersion}",
      buildMode: params.MODE,
      branch: env.BRANCH_NAME)
    }

    parallel(scans)

    // Pull existing images from internal registry in order to promote
    infrapool.agentSh """
      export PATH="release-tools/bin:${PATH}"
      docker pull registry.tld/secrets-provider-for-k8s:${sourceVersion}
      docker pull registry.tld/secrets-provider-for-k8s-redhat:${sourceVersion}
      # Promote source version to target version.
      summon ./bin/publish --promote --source ${sourceVersion} --target ${targetVersion}
    """

    dockerImages = "docker-image*.tar"
    // Place the Docker image(s) onto the Jenkins agent and sign them
    infrapool.agentGet from: "${assetDirectory}/${dockerImages}", to: "./"
    signArtifacts patterns: ["${dockerImages}"]
    // Copy the docker images and signed artifacts (.sig) back to
    // infrapool and into the assetDirectory for release promotion
    dockerImageLocation = pwd() + "/docker-image*.tar*"
    infrapool.agentPut from: "${dockerImageLocation}", to: "${assetDirectory}"
    // Resolve ownership issue before promotion
    sh 'git config --global --add safe.directory ${PWD}'
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

    // Values to direct scan results to the right place in DefectDojo
    INFRAPOOL_PRODUCT_NAME = "${productName}"
    INFRAPOOL_PRODUCT_TYPE_NAME = "${productTypeName}"
  }

  triggers {
    cron(getDailyCronString())
    parameterizedCron(getWeeklyCronString("H(1-5)","%MODE=RELEASE"))
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
        stage('Package helm chart') {
          steps {
            script {
              INFRAPOOL_EXECUTORV2_AGENT_0.agentSh './ci/jenkins_build'
            }
          }
        }

        // Allows for the promotion of images. Need to push before we do security scans
        // since the Snyk scans pull from artifactory on a seprate executor node
        stage('Push images to internal registry') {
          steps {
            script {
              INFRAPOOL_EXECUTORV2_AGENT_0.agentSh './bin/publish --internal'
            }
          }
        }

        stage('Scan Docker Image') {
          parallel {
            stage("Scan main image") {
              steps {
                script {
                  VERSION = INFRAPOOL_EXECUTORV2_AGENT_0.agentSh(returnStdout: true, script: 'cat VERSION')
                }
                runSecurityScans(INFRAPOOL_EXECUTORV2_AGENT_0,
                  image: "registry.tld/secrets-provider-for-k8s:${VERSION}",
                  buildMode: params.MODE,
                  branch: env.BRANCH_NAME)
              }
            }
           stage('Scan RedHat image') {
              steps {
                script {
                  VERSION = INFRAPOOL_EXECUTORV2_AGENT_0.agentSh(returnStdout: true, script: 'cat VERSION')
                }
                runSecurityScans(INFRAPOOL_EXECUTORV2_AGENT_0,
                  image: "registry.tld/secrets-provider-for-k8s-redhat:${VERSION}",
                  buildMode: params.MODE,
                  branch: env.BRANCH_NAME)
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

        stage ("DAP Integration Tests on GKE/Openshift") {
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
              tasks["Openshift (Current), DAP"] = {
                INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "./bin/start --docker --dap --current"
              }
              parallel tasks
            }
          }
        }

        stage ("DAP Integration Tests on OpenShift oldest/next") {
          steps {
            script {
              def tasks = [:]
              if ( params.TEST_OCP_OLDEST ) {
                tasks["Openshift (Oldest), DAP"] = {
                  INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "./bin/start --docker --dap --oldest"
                }
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

        stage('Release') {
          when {
            expression {
              MODE == "RELEASE"
            }
          }
          stages {
          stage('Release') {
              steps {
                script {
                  release(INFRAPOOL_EXECUTORV2_AGENT_0) { billOfMaterialsDirectory, assetDirectory, toolsDirectory ->
                    // Publish release artifacts to all the appropriate locations
                    // Copy any artifacts to assetDirectory to attach them to the Github release

                    // Copy helm chart to assetDirectory
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "cp -a helm-artifacts/*.tgz ${assetDirectory}"
                    // Create Go application SBOM using the go.mod version for the golang container image
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh """export PATH="${toolsDirectory}/bin:${PATH}" && go-bom --tools "${toolsDirectory}" --go-mod ./go.mod --image "golang" --main "cmd/secrets-provider/" --output "${billOfMaterialsDirectory}/go-app-bom.json" """
                    // Create Go module SBOM
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh """export PATH="${toolsDirectory}/bin:${PATH}" && go-bom --tools "${toolsDirectory}" --go-mod ./go.mod --image "golang" --output "${billOfMaterialsDirectory}/go-mod-bom.json" """
                    // Publish will save docker images to the executing directory
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh 'summon ./bin/publish --edge'
                    // Copy the docker images into the assetDirectory for signing in the promote step
                    INFRAPOOL_EXECUTORV2_AGENT_0.agentSh "cp -a docker-image*.tar ${assetDirectory}"

                  }
                }
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

      // Resolve ownership issue before running infra post hook
      sh 'git config --global --add safe.directory ${PWD}'
      infraPostHook()
    }
  }
}
