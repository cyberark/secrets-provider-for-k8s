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

    stage('Run Integrations Tests') {
      steps {
        sh './bin/test_integration --docker'
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
