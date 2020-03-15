#!/usr/bin/env groovy

pipeline {
  agent { label 'executor-v2' }

  options {
    timestamps()
    disableConcurrentBuilds()
    buildDiscarder(logRotator(numToKeepStr: '30'))
  }

  stages {
    stage('Validate') {
      parallel {
        stage('Changelog') {
          steps { sh './bin/parse-changelog.sh' }
        }
      }
    }

    stage('Build client Docker image') {
      steps {
        sh './bin/build'
      }
    }

    stage('Run Unit Tests') {
      steps {
        sh './bin/test_unit'

        junit 'unit-test/junit.xml'
      }
    }

   stage ("Run Integration Tests on oss") {
      steps {
        script {
          def tasks = [:]
          ["oss"].each { deployment ->
            tasks["Kubernetes GKE, ${deployment}"] = {
                sh "./bin/test_integration --docker --${deployment} --gke"
            }
            tasks["Openshift v3.11, ${deployment}"] = {
                sh "./bin/test_integration --docker --${deployment} --oc311"
            }
            tasks["Openshift v3.10, ${deployment}"] = {
                sh "./bin/test_integration --docker --${deployment} --oc310"
            }
          }
          parallel tasks
        }
      }
    }

    stage ("Run Integration Tests on dap") {
          steps {
            script {
              def tasks = [:]
              ["dap"].each { deployment ->
                tasks["Kubernetes GKE, ${deployment}"] = {
                    sh "./bin/test_integration --docker --${deployment} --gke"
                }
                tasks["Openshift v3.11, ${deployment}"] = {
                    sh "./bin/test_integration --docker --${deployment} --oc311"
                }
                tasks["Openshift v3.10, ${deployment}"] = {
                    sh "./bin/test_integration --docker --${deployment} --oc310"
                }
              }
              parallel tasks
            }
          }
        }

    stage('Publish client Docker image') {
        steps {
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
                    ./bin/publish
                '''
            }
        }
    }
  }

  post {
    success {
      cleanupAndNotify(currentBuild.currentResult)
    }
    unsuccessful {
      cleanupAndNotify(currentBuild.currentResult, "#development", "@secrets-provider-for-k8s-owners")
    }
  }
}
