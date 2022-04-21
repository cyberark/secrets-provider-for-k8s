#!/usr/bin/env groovy

// Automated release, promotion and dependencies
properties([
  // Include the automated release parameters for the build
  release.addParams(),
  // Dependencies of the project that should trigger builds
  dependencies([
    'cyberark/conjur-opentelemetry-tracer',
    'cyberark/conjur-authn-k8s-client',
    'cyberark/conjur-api-go'
  ])
])

// Performs release promotion.  No other stages will be run
if (params.MODE == "PROMOTE") {
  release.promote(params.VERSION_TO_PROMOTE) { sourceVersion, targetVersion, assetDirectory ->
    // Any assets from sourceVersion Github release are available in assetDirectory
    // Any version number updates from sourceVersion to targetVersion occur here
    // Any publishing of targetVersion artifacts occur here
    // Anything added to assetDirectory will be attached to the Github Release

    // Pull existing images from internal registry in order to promote
    sh "docker pull registry.tld/secrets-provider-for-k8s:${sourceVersion}"
    sh "docker pull registry.tld/secrets-provider-for-k8s-redhat:${sourceVersion}"
    // Promote source version to target version.
    sh "summon ./bin/publish --promote --source ${sourceVersion} --target ${targetVersion}"
  }
  return
}

pipeline {
  agent { label 'executor-v2' }

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
  }

  stages {
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

    // Generates a VERSION file based on the current build number and latest version in CHANGELOG.md
    stage('Validate Changelog and set version') {
      steps {
        updateVersion("CHANGELOG.md", "${BUILD_NUMBER}")
      }
    }

    stage('Get latest upstream dependencies') {
      steps {
        updateGoDependencies('${WORKSPACE}/go.mod')
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
          tag "v*"
        }
      }
      stages {
        stage('Build client Docker image') {
          steps {
            sh './bin/build'
          }
        }

        stage('Release') {
          when {
            expression {
              MODE == "RELEASE"
            }
          }
          steps {
            release { billOfMaterialsDirectory, assetDirectory, toolsDirectory ->
              // Publish release artifacts to all the appropriate locations
              // Copy any artifacts to assetDirectory to attach them to the Github release

              //    // Create Go application SBOM using the go.mod version for the golang container image
              sh """go-bom --tools "${toolsDirectory}" --go-mod ./go.mod --image "golang" --main "cmd/secrets-provider/" --output "${billOfMaterialsDirectory}/go-app-bom.json" """
              //    // Create Go module SBOM
              sh """go-bom --tools "${toolsDirectory}" --go-mod ./go.mod --image "golang" --output "${billOfMaterialsDirectory}/go-mod-bom.json" """
              sh """cat "${billOfMaterialsDirectory}/go-app-bom.json" """
              sh """cat "${billOfMaterialsDirectory}/go-mod-bom.json" """
              sh """cat go.mod """
              sh """ exit 1 """
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
    }
    success {
      cleanupAndNotify(currentBuild.currentResult)
    }
    unsuccessful {
      script {
        if (env.BRANCH_NAME == 'main') {
          cleanupAndNotify(currentBuild.currentResult, "#development", "@secrets-provider-for-k8s-owners")
        } else {
          cleanupAndNotify(currentBuild.currentResult, "#development")
        }
      }
    }
  }
}
