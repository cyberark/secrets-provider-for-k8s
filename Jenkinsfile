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


        stage ("Run Integration Tests on DAP") {
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
