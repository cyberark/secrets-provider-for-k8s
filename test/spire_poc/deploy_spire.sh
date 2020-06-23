#!/bin/bash
set -euxo pipefail

cli=kubectl

function teardown() {

    # Delete the workload container
    $cli delete deployment client --ignore-not-found

    # Run the following command to delete all deployments and configurations for the agent, server, and namespace:
    $cli delete namespace spire --ignore-not-found

    # Run the following commands to delete the ClusterRole and ClusterRoleBinding settings:
    $cli delete clusterrole spire-server-trust-role spire-agent-cluster-role --ignore-not-found
    $cli delete clusterrolebinding spire-server-trust-role-binding spire-agent-cluster-role-binding --ignore-not-found

}

#############################################################################
# this script is based on: https://spiffe.io/spire/try/getting-started-k8s/ #
#############################################################################
rm -rf spire-tutorials
git clone https://github.com/spiffe/spire-tutorials.git
cd ./spire-tutorials/k8s/quickstart

teardown
# Delete any leftover
#kubectl delete -f server-cluster-role.yaml --ignore-not-found
#kubectl delete serviceaccount -n spire spire-server --ignore-not-found
#kubectl delete -f spire-bundle-configmap.yaml --ignore-not-found
#kubectl delete all --all -n spire --ignore-not-found
#kubectl delete -f agent-account.yaml --ignore-not-found
#kubectl delete -f agent-cluster-role.yaml --ignore-not-found
#kubectl delete namespace spire  --ignore-not-found


### Configure Kubernetes Namespace for SPIRE Components ###

# create 'spire' namespace in which SPIRE Server and SPIRE Agent are deployed.
$cli apply -f spire-namespace.yaml

# Verify namespace was created
$cli get namespaces | grep spire

$cli config set-context --current --namespace=spire

### Configure SPIRE Server ###

# For the server to function, it is necessary for it to provide agents with certificates
# that they can use to verify the identity of the server when establishing a connection.
# In a deployment such as this, where the agent and server share the same cluster, SPIRE
# can be configured to automatically generate these certificates on a periodic basis and
# update a configmap with contents of the certificate. To do that, the server needs the
# ability to get and patch a configmap object in the spire namespace.
# To allow the server to read and write to this configmap, a ClusterRole must be created
# that confers the appropriate entitlements to Kubernetes RBAC, and that ClusterRoleBinding
# must be associated with the service account created in the previous step.
# Create the server’s service account, configmap and associated role bindings as follows:

$cli apply -f server-account.yaml
$cli apply -f spire-bundle-configmap.yaml
$cli apply -f server-cluster-role.yaml


### Create Server Configmap
# The server is configured in the Kubernetes configmap specified in server-configmap.yaml,
# which specifies a number of important directories, notably /run/spire/data and
# /run/spire/config. These volumes are bound in when the server container is deployed.
# Deploy the server configmap and statefulset by applying the following files via kubectl:
$cli apply -f server-configmap.yaml
$cli apply -f server-statefulset.yaml
$cli apply -f server-service.yaml

# This creates a statefulset called 'spire-server' in the spire namespace and starts
# up a 'spire-server' pod. Following commands assert it:
$cli get pods --namespace spire -o=jsonpath='{.items[].metadata.name}' | grep spire-server
$cli get statefulset --namespace spire -o=jsonpath='{.items[].metadata.name}' | grep spire-server
$cli get services --namespace spire | grep spire-server

### Configure and deploy the SPIRE Agent ###

# To allow the agent read access to the kubelet API to perform workload attestation,
# a Service Account and ClusterRole must be created that confers the appropriate
# entitlements to Kubernetes RBAC, and that ClusterRoleBinding must be associated with
# the service account created in the previous step.
$cli apply -f agent-account.yaml -f agent-cluster-role.yaml

# Apply the agent-configmap.yaml configuration file to create the agent configmap
# and deploy the Agent as a daemonset that runs one instance of each Agent on each
# Kubernetes worker node.
$cli apply -f agent-configmap.yaml -f agent-daemonset.yaml

# This creates a daemonset called spire-agent in the spire namespace and starts up
# a spire-agent pod along side spire-server, as demonstrated in the output of the
# following commands:
$cli get daemonset --namespace spire -o=jsonpath='{.items[].metadata.name}' | grep spire-agent
$cli get pods --namespace spire | grep spire-agent
$cli get pods --namespace spire | grep spire-server
# As a daemonset, you’ll see as many spire-agent pods as you have nodes.

sleep 20

# Create a new registration entry for the node, specifying the SPIFFE ID to allocate
# to the node:
$cli exec -n spire spire-server-0 -- \
    /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/ns/spire/sa/spire-agent \
    -selector k8s_sat:cluster:demo-cluster \
    -selector k8s_sat:agent_ns:spire \
    -selector k8s_sat:agent_sa:spire-agent \
    -node

# Create a new registration entry for the workload, specifying the SPIFFE ID to
# allocate to the workload:
$cli exec -n spire spire-server-0 -- \
    /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/ns/default/sa/default \
    -parentID spiffe://example.org/ns/spire/sa/spire-agent \
    -selector k8s:ns:default \
    -selector k8s:sa:default


### Configure a Workload Container to Access SPIRE ###

# In this step, you configure a workload container to access SPIRE. Specifically,
# you are configuring the workload container to access the Workload API UNIX domain socket.

# Configure a Workload Container to Access SPIRE
$cli apply -f client-deployment.yaml

client_pod=$(kubectl get pods | awk '{print $1;}' | grep client-)
$cli exec -it $client_pod /bin/sh
