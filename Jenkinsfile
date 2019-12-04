#!/usr/bin/env groovy

pipeline {
  agent { label 'executor-v2' }

  options {
    timestamps()
    buildDiscarder(logRotator(numToKeepStr: '30'))
  }

  stages {
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

    stage ("Run Integration Tests") {
      steps {
        script {
          def tasks = [:]
          ["oss", "dap"].each { deployment ->
            stage("Run Integration Tests with ${deployment} deployment") {
              parallel "Kubernetes GKE, ${deployment}": {
                sh "./bin/test_integration --docker --${deployment} --gke"
              },
              "Openshift v3.11, ${deployment}": {
                sh "./bin/test_integration --docker --${deployment} --oc311"
              },
              "Openshift v3.10, ${deployment}": {
                sh "./bin/test_integration --docker --${deployment} --oc310"
              },
              "Openshift v3.9, ${deployment}": {
                sh "./bin/test_integration --docker --${deployment} --oc39"
              }
            }
          }
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
    always {
      cleanupAndNotify(currentBuild.currentResult)
    }
  }
}
