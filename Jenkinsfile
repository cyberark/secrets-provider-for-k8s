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

    stage('Run Integration Tests with OSS') {
      parallel {
        stage('Kubernetes GKE') {
          steps {
            sh './bin/test_integration --docker --gke'
          }
        }
        stage('Openshift v3.11') {
          steps {
            sh './bin/test_integration --docker --oc311'
          }
        }
        stage('Openshift v3.10') {
          steps {
            sh './bin/test_integration --docker --oc310'
          }
        }
        stage('Openshift v3.9') {
          steps {
            sh './bin/test_integration --docker --oc39'
          }
        }
      }
    }

    stage('Run Integration Tests with DAP') {
      parallel {
        stage('Kubernetes GKE') {
          steps {
            sh './bin/test_integration --docker --dap --gke'
          }
        }
        stage('Openshift v3.11') {
          steps {
            sh './bin/test_integration --docker --dap --oc311'
          }
        }
        stage('Openshift v3.10') {
          steps {
            sh './bin/test_integration --docker --dap --oc310'
          }
        }
        stage('Openshift v3.9') {
          steps {
            sh './bin/test_integration --docker --dap --oc39'
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
