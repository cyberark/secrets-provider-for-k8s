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
        sh './bin/test'
        
        junit 'unit-test/junit.xml'
      }
    }

    stage('Run Integrations Tests') {
      steps {
        sh 'cd test && ./test --docker'
      }
    }

    stage('Publish client Docker image') {
      when {
        branch 'master'
      }
      steps {
        sh './bin/publish'
      }
    }
  }

  post {
    always {
      cleanupAndNotify(currentBuild.currentResult)
    }
  }
}
