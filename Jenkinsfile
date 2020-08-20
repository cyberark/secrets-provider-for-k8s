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
    timeout(time: 2, unit: 'HOURS')
  }

  stages {
    stage('Validate') {
      parallel {
        stage('Changelog') {
          steps { sh './bin/parse-changelog.sh' }
        }
      }
    }

    stage('Check permissions') {
      steps {
        sh 'ls -l /var/lib/jenkins/.docker/config.json'
      }
    }

    stage('Build client Docker image') {
      steps {
        sh './bin/build'
      }
    }

    stage('Check permissions 2') {
      steps {
        sh 'ls -l /var/lib/jenkins/.docker/config.json'
      }
    }

    stage('Run Unit Tests') {
      steps {
        sh './bin/test_unit'

        junit 'junit.xml'
        cobertura autoUpdateHealth: true, autoUpdateStability: true, coberturaReportFile: 'coverage.xml', conditionalCoverageTargets: '30, 0, 0', failUnhealthy: true, failUnstable: false, maxNumberOfBuilds: 0, methodCoverageTargets: '30, 0, 0', onlyStable: false, sourceEncoding: 'ASCII', zoomCoverageChart: false
        ccCoverage("gocov", "--prefix github.com/cyberark/secrets-provider-for-k8s")
      }
    }

    stage('Check permissions 3') {
      steps {
        sh 'ls -l /var/lib/jenkins/.docker/config.json'
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
              sh "./bin/start --docker --oss --gke"
            }
            // tasks["Openshift v3.11, oss"] = {
            //  sh "./bin/start --docker --oss --oc311"
            // }
            // skip oc310 tests until the environment will be ready to use
            // tasks["Openshift v3.10, oss"] = {
            //   sh "./bin/start --docker --oss --oc310"
            // }
          parallel tasks
        }
      }
    }

    stage('Check permissions 4') {
      steps {
        sh 'ls -l /var/lib/jenkins/.docker/config.json'
        sh 'exit 1'
      }
    }

    stage ("Run Integration Tests on DAP") {
      steps {
        script {
          def tasks = [:]
            tasks["Kubernetes GKE, DAP"] = {
              sh "./bin/start --docker --dap --gke"
            }
            tasks["Openshift v3.11, DAP"] = {
              sh "./bin/start --docker --dap --oc311"
            }
            // skip oc310 tests until the environment will be ready to use
            // tasks["Openshift v3.10, DAP"] = {
            //  sh "./bin/start --docker --dap --oc310"
            // }
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
                summon ./bin/publish
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
        script {
            if (env.BRANCH_NAME == 'master') {
              cleanupAndNotify(currentBuild.currentResult, "#development", "@secrets-provider-for-k8s-owners")
            } else {
              cleanupAndNotify(currentBuild.currentResult, "#development")
            }
        }
    }
  }
}
