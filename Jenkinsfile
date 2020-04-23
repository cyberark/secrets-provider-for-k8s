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

    // We want to avoid running in parallel.
    // When we have 2 build running on the same environment (gke env only) in parallel,
    // we get the error "gcloud crashed : database is locked"
    stage ("Run Integration Tests on oss") {
      steps {
        script {
          def tasks = [:]
            tasks["Kubernetes GKE, oss"] = {
              sh "./bin/test_integration --docker --oss --gke"
            }
            tasks["Openshift v3.11, oss"] = {
              sh "./bin/test_integration --docker --oss --oc311"
            }
            tasks["Openshift v3.10, oss"] = {
              sh "./bin/test_integration --docker --oss --oc310"
            }
          parallel tasks
        }
      }
    }

    stage ("Run Integration Tests on DAP") {
      steps {
        script {
          def tasks = [:]
            tasks["Kubernetes GKE, DAP"] = {
              sh "./bin/test_integration --docker --dap --gke"
            }
            tasks["Openshift v3.11, DAP"] = {
              sh "./bin/test_integration --docker --dap --oc311"
            }
            tasks["Openshift v3.10, DAP"] = {
              sh "./bin/test_integration --docker --dap --oc310"
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
