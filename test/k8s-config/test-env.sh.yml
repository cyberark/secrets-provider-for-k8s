#!/bin/bash

set -euo pipefail
cat << EOL
---
apiVersion: v1
kind: DeploymentConfig
metadata:
  labels:
    app: test-env
  name: test-env
spec:
  replicas: 1
  selector:
    app: test-env
  template:
    metadata:
      labels:
        app: test-env
    spec:
      serviceAccountName: ${TEST_APP_NAMESPACE_NAME}-sa
      containers:
      - image: debian
        name: test-app
        command: ["printenv"]
        args: ["TEST_SECRET"]
        env:
          - name: TEST_SECRET
            valueFrom:
              secretKeyRef:
                name: test-k8s-secret
                key: secret
      initContainers:
      - image: '${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/secrets-provider:latest'
        imagePullPolicy: Always
        name: cyberark-secrets-provider
        env:
          - name: CONTAINER_MODE
            value: init

          - name: MY_POD_NAME
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.name

          - name: MY_POD_NAMESPACE
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.namespace

          - name: MY_POD_IP
            valueFrom:
              fieldRef:
                fieldPath: status.podIP

          - name: CONJUR_VERSION
            value: '5'

          - name: CONJUR_APPLIANCE_URL
            value: >-
              https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local/api

          - name: CONJUR_AUTHN_URL
            value: >-
              https://conjur-follower.${CONJUR_NAMESPACE_NAME}.svc.cluster.local/api/authn-k8s/${AUTHENTICATOR_ID}

          - name: CONJUR_ACCOUNT
            value: ${CONJUR_ACCOUNT}

          - name: CONJUR_AUTHN_LOGIN
            value: >-
              host/conjur/authn-k8s/${AUTHENTICATOR_ID}/apps/${TEST_APP_NAMESPACE_NAME}/*/*

          - name: CONJUR_SSL_CERTIFICATE
            valueFrom:
              configMapKeyRef:
                name: conjur-master-ca-env
                key: ssl-certificate

          - name: K8S_SECRETS
            value: test-k8s-secret

          - name: DEBUG
            value: "true"

          - name: SECRETS_DESTINATION
            value: k8s_secrets

      imagePullSecrets:
        - name: dockerpullsecret
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: conjur-master-ca-env
  label:
    app: test-env
data:
  ssl-certificate: |
$(echo "${CONJUR_SSL_CERTIFICATE}" | while read line; do printf "%20s%s\n" "" "$line"; done)
EOL
