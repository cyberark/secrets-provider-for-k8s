#!/bin/bash -e

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 cluster_name"
  exit 1
fi

PROJECT=" conjur-gke-dev"
VERSION="1.12.5-gke.10"

REGION="us-central1"
ZONE="$REGION-a"
MACHINE_TYPE="n1-standard-4"

cluster_name="$1"

nodes=1
if [[ $# -gt 1 ]]; then
  nodes=$2
fi

echo "Creating cluster $cluster_name in $ZONE with $nodes nodes..."

gcloud beta container --project "$PROJECT" \
  clusters create "$cluster_name" --zone "$ZONE" \
                                  --no-enable-basic-auth \
                                  --cluster-version "$VERSION" \
                                  --machine-type "$MACHINE_TYPE" \
                                  --image-type "COS" \
                                  --disk-type "pd-standard" \
                                  --disk-size "50" \
                                  --scopes "https://www.googleapis.com/auth/devstorage.read_only","https://www.googleapis.com/auth/logging.write","https://www.googleapis.com/auth/monitoring","https://www.googleapis.com/auth/servicecontrol","https://www.googleapis.com/auth/service.management.readonly","https://www.googleapis.com/auth/trace.append" \
                                  --num-nodes "$nodes" \
                                  --no-enable-cloud-logging \
                                  --no-enable-cloud-monitoring \
                                  --no-enable-ip-alias \
                                  --network "projects/$PROJECT/global/networks/default" \
                                  --subnetwork "projects/$PROJECT/regions/$REGION/subnetworks/default" \
                                  --addons HorizontalPodAutoscaling,HttpLoadBalancing,KubernetesDashboard \
                                  --enable-autoupgrade \
                                  --enable-autorepair

