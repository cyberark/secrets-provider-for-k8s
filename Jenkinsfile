#!/usr/bin/env groovy

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

  parameters {
    booleanParam(name: 'TEST_OCP_NEXT', defaultValue: false, description: 'Run DAP tests against our running "next version" of Openshift')

    booleanParam(name: 'TEST_OCP_OLDEST', defaultValue: false, description: 'Run DAP tests against our running "oldest version" of Openshift')
  }

  stages {
    stage('Validate') {
      parallel {
        stage('Changelog') {
          steps { sh './bin/parse-changelog.sh' }
        }
      }
    }

    stage('Build and test Secrets Provider') {
      when {
        // Run tests only when EITHER of the following is true:
        // 1. A non-markdown file has changed.
        // 2. It's the main branch.
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
        }
      }
      stages {
        stage('Build client Docker image') {
          steps {
            sh './bin/build'
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
                scanAndReport('secrets-provider-for-k8s:latest', "HIGH", false)
              }
            }
            stage("Scan Docker image for total issues") {
              steps {
                // By default, trivy includes vulnerabilities with no fix. We
                // want to know about that ASAP, but they shouldn't cause a
                // build failure until we can do something about it. This call
                // to scanAndReport should always be left as "NONE"
                scanAndReport("secrets-provider-for-k8s:latest", "NONE", true)
              }
            }
            stage('Scan RedHat image for fixable issues') {
              steps {
                scanAndReport("secrets-provider-for-k8s-redhat:latest", "HIGH", false)
              }
            }
    
            stage('Scan RedHat image for all issues') {
              steps {
                scanAndReport("secrets-provider-for-k8s-redhat:latest", "NONE", true)
              }
            }
          }
        }

        stage('Run Unit Tests') {
          steps {
            sh './bin/test_unit'

            junit 'junit.xml'
            cobertura autoUpdateHealth: false, autoUpdateStability: false, coberturaReportFile: 'coverage.xml', conditionalCoverageTargets: '70, 0, 0', failUnhealthy: false, failUnstable: false, maxNumberOfBuilds: 0, lineCoverageTargets: '70, 0, 0', methodCoverageTargets: '70, 0, 0', onlyStable: false, sourceEncoding: 'ASCII', zoomCoverageChart: false
            ccCoverage("gocov", "--prefix github.com/cyberark/secrets-provider-for-k8s")
          }
        }
        
        
        stage ("DAP Integration Tests on GKE") {
          steps {
            script {
              def tasks = [:]
              tasks["Kubernetes GKE, DAP"] = {
                sh "./bin/start --docker --dap --gke"
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
              tasks["Openshift v3.11, DAP"] = {
                  sh "./bin/start --docker --dap --oc311"
              }
              if ( params.TEST_OCP_OLDEST ) {
                tasks["Openshift (Oldest), DAP"] = {
                  sh "./bin/start --docker --dap --oldest"
                }
              }
              tasks["Openshift (Current), DAP"] = {
                sh "./bin/start --docker --dap --current"
              }
              if ( params.TEST_OCP_NEXT ) {
                tasks["Openshift (Next), DAP"] = {
                  sh "./bin/start --docker --dap --next"
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
          steps {
            script {
              def tasks = [:]
                tasks["Kubernetes GKE, oss"] = {
                  sh "./bin/start --docker --oss --gke"
                }
              parallel tasks
            }
          }
        }

        stage('Release') {
          parallel {
            stage('Push Images') {
              steps {
                script {
                  BRANCH_NAME=env.BRANCH_NAME
                }
                withCredentials(
                  [
                    usernamePassword(
                      credentialsId: 'conjur-jenkins-api',
                      usernameVariable: 'GIT_USER',
                      passwordVariable: 'GIT_PASSWORD'
                    )
                  ]
                ) {
                    sh '''
                        git config --local credential.helper '! echo username=${GIT_USER}; echo password=${GIT_PASSWORD}; echo > /dev/null'
                        git fetch --tags
                        export GIT_DESCRIPTION=$(git describe --tags)
                        export BRANCH_NAME=${BRANCH_NAME}
                        summon ./bin/publish
                    '''
                  }
              }
            }
            stage('Package artifacts') {
              steps {
                sh 'ci/jenkins_build'

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
