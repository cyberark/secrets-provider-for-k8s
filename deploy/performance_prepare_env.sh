#!/bin/bash
set -eu

function oc_login() {
  summon -p summon-conjur --yaml="
OPENSHIFT_URL: !var dev/openshift/$1/hostname
OPENSHIFT_USERNAME: !var dev/openshift/$1/username
OPENSHIFT_PASSWORD: !var dev/openshift/$1/password
" sh -c "oc login \$OPENSHIFT_URL:8443 --insecure-skip-tls-verify=true -u \$OPENSHIFT_USERNAME -p \$OPENSHIFT_PASSWORD"
}

function runScriptWithSummon() {
    summon --environment $1 -f $repo_root_path/deploy/summon/secrets.yml $2
}

function deploy_conjur() {
    k8s_deploy_folder=kubernetes-conjur-deploy-$UNIQUE_TEST_ID
    if [[ ! -d $k8s_deploy_folder ]]; then
      git clone git@github.com:cyberark/kubernetes-conjur-deploy $k8s_deploy_folder
    fi
    pushd $k8s_deploy_folder > /dev/null
      runScriptWithSummon $1 "./start --dap"
    popd > /dev/null
}

# Variables
OPENSHIFT_VERSION=3.11
SUMMON_ENV=oc311
k8s_secrets_count=10
repo_root_path=$(git rev-parse --show-toplevel)

# Login
oc_login $OPENSHIFT_VERSION

# Prepare env vars
script_dir=$(dirname "$0")
cd $script_dir
source ../bootstrap.env

#	Deploy conjur
deploy_conjur $SUMMON_ENV


#	Get conjur cli pod name
CONJUR_CLI=$(oc get pods | grep conjur-cli | awk '{print $1}')

#	Create authenticator policy
envsubst < authn-k8s-policy.yml |
  oc exec $CONJUR_CLI -i -- conjur policy load --replace root -

#	Create and populate secrets in Conjur
for (( i = 1; i <= k8s_secrets_count; i++ ))
do
  index=$i envsubst < secrets-policy-template.yml |
    oc exec $CONJUR_CLI -i -- conjur policy load root -
  for j in {1..5}
  do
    oc exec $CONJUR_CLI -i -- conjur variable values add secrets-$i/app-secret-$j my-very-topsecret-$i-$j
  done
done

# Get conjur master pod name
MASTER=$(oc get pods | grep conjur-cluster | head -1 | awk '{print $1}')

#	Copy Conjur certificate
oc exec $MASTER cat /opt/conjur/etc/ssl/conjur.pem > conjur-ssl-cert.pem

#	Enable and prepare authenticator
oc exec $MASTER -- chpst -u conjur conjur-plugin-service possum "rake authn_k8s:ca_init[conjur/authn-k8s/$AUTHENTICATOR_ID]"

#	Create secrets provider namespace
oc new-project $APP_NAMESPACE_NAME

#	Create k8s entities needed for using authn k8s by secrets provider
envsubst < conjur-authenticator-role.yml | oc apply -f -
envsubst < secrets-access-role.yml | oc apply -f -
oc create configmap conjur-master-ca-env --from-file=ssl-certificate=conjur-ssl-cert.pem
oc label configmap conjur-master-ca-env app=test-env

#	Create k8s secrets
for (( i = 1; i <= k8s_secrets_count; i++ ))
do
  index=$i envsubst < k8s-secrets-template.yml |
    oc apply -f -
done

#	Enable pulling secrets provider image from openshift internal registry
export DOCKER_REGISTRY_PATH=$(runScriptWithSummon $SUMMON_ENV 'printenv DOCKER_REGISTRY_PATH')

oc create secret docker-registry dockerpullsecret --docker-server=${DOCKER_REGISTRY_PATH} --docker-username=_ --docker-password=$(oc whoami -t) --docker-email=_
oc secrets add serviceaccount/default secrets/dockerpullsecret --for=pull

#	Login to opneshift internal registry
docker login -u _ -p $(oc whoami -t) $DOCKER_REGISTRY_PATH

