FROM google/cloud-sdk

RUN mkdir -p /src
WORKDIR /src

# Install Docker client
RUN apt-get update -y && \
    apt-get install -y apt-transport-https ca-certificates curl gnupg2 software-properties-common wget && \
    curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "$ID")/gpg | apt-key add - && \
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/$(. /etc/os-release; echo "$ID") $(lsb_release -cs) stable" && \
    apt-get update && \
    apt-get install -y docker-ce && \
    rm -rf /var/lib/apt/lists/*

# Install kubectl CLI
ARG KUBECTL_CLI_URL
RUN wget -O /usr/local/bin/kubectl ${KUBECTL_CLI_URL:-https://storage.googleapis.com/kubernetes-release/release/v1.7.6/bin/linux/amd64/kubectl} && \
    chmod +x /usr/local/bin/kubectl

# Install OpenShift oc CLI
ARG OPENSHIFT_CLI_URL
RUN mkdir -p ocbin && \
    wget -O oc.tar.gz ${OPENSHIFT_CLI_URL:-https://github.com/openshift/origin/releases/download/v3.7.2/openshift-origin-client-tools-v3.7.2-282e43f-linux-64bit.tar.gz} && \
    tar xvf oc.tar.gz --strip-components=1 -C ocbin && \
    mv ocbin/oc /usr/local/bin/oc && \
    rm -rf ocbin oc.tar.gz

# Install Helm
RUN curl https://baltocdn.com/helm/signing.asc | apt-key add - && \
    apt-get install apt-transport-https --yes && \
    echo "deb https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list && \
    apt-get update && \
    apt-get install helm=3.2.*

# Adds ability to perform mathematical operations with floats for testing
RUN apt-get install -y bc