#!/bin/bash

set -euo pipefail
cat << EOL
---
apiVersion: v1
kind: Service
metadata:
  name: pet-store-env
  labels:
    app: pet-store-env
spec:
  ports:
    - protocol: TCP
      port: 8080
      targetPort: 8080
  selector:
    app: pet-store-env
  type: LoadBalancer
---
apiVersion: v1
kind: DeploymentConfig
metadata:
  labels:
    app: pet-store-env
  name: pet-store-env
spec:
  replicas: 1
  selector:
    app: pet-store-env
  template:
    metadata:
      labels:
        app: pet-store-env
    spec:
      serviceAccountName: ${TEST_APP_NAMESPACE_NAME}-sa
      containers:
          - image: '${DOCKER_REGISTRY_PATH}/${TEST_APP_NAMESPACE_NAME}/demo-app:latest'
            imagePullPolicy: Always
            name: pet-store-env
            ports:
              - name: http
                containerPort: 8080
            readinessProbe:
              httpGet:
                path: /pets
                port: http
              initialDelaySeconds: 15
              timeoutSeconds: 5
            env:
              - name: DB_PLATFORM
                value: "postgres"
              - name: DB_URL
                value: "postgresql://${POSTGRES_HOSTNAME}:5432/${POSTGRES_DATABASE}"
              - name: DB_USERNAME
                valueFrom:
                  secretKeyRef:
                    name: db-credentials
                    key: username
              - name: DB_PASSWORD
                valueFrom:
                  secretKeyRef:
                    name: db-credentials
                    key: password
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
              value: db-credentials

            - name: SECRETS_DESTINATION
              value: k8s_secrets

            - name: DEBUG
              value: "true"

      imagePullSecrets:
        - name: dockerpullsecret
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: conjur-master-ca-env
  label:
    app: pet-store-env
data:
  ssl-certificate: |
$(echo "${CONJUR_SSL_CERTIFICATE}" | while read line; do printf "%20s%s\n" "" "$line"; done)
EOL
