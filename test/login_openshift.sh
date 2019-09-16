#!/bin/bash

if [ -z ${OPENSHIFT_USERNAME} ] || [ -z ${OPENSHIFT_PASSWORD} ]; then
      echo "Make sure this script is running with summon"
      exit
fi

oc login -s openshift-311.itci.conjur.net:8443 -u $OPENSHIFT_USERNAME -p $OPENSHIFT_PASSWORD
