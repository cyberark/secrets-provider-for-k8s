# =================== BASE BUILD LAYER ===================
# this layer is used to prepare a common layer for both debug and release builds
FROM golang:1.24 AS secrets-provider-builder-base
LABEL maintainer="CyberArk Software Ltd."

ENV GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0

# On CyberArk dev laptops, golang module dependencies are downloaded with a
# corporate proxy in the middle. For these connections to succeed we need to
# configure the proxy CA certificate in build containers.
#
# To allow this script to also work on non-CyberArk laptops where the CA
# certificate is not available, we copy the (potentially empty) directory
# and update container certificates based on that, rather than rely on the
# CA file itself.
COPY build_ca_certificate /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN go install github.com/jstemmer/go-junit-report/v2@latest

WORKDIR /opt/secrets-provider-for-k8s

EXPOSE 8080

COPY go.mod go.sum ./

# =================== RELEASE BUILD LAYER ===================
# this layer is used to build the release binaries
FROM secrets-provider-builder-base AS secrets-provider-builder

COPY . .

# this value is set in ./bin/build
ARG TAG

RUN go build \
    -a \
    -installsuffix cgo \
    -ldflags="-X github.com/cyberark/secrets-provider-for-k8s/pkg/secrets.Tag=$TAG" \
    -o secrets-provider \
    ./cmd/secrets-provider

# =================== DEBUG BUILD LAYER ===================
# this layer is used to build the debug binaries
FROM secrets-provider-builder-base AS secrets-provider-builder-debug

# Build Delve - debugging tool for Go
RUN go get github.com/go-delve/delve/cmd/dlv

# Expose port 40000 for debugging
EXPOSE 40000

COPY . .

# Build debug flavor without compilation optimizations using "all=-N -l"
RUN go build -a -installsuffix cgo -gcflags="all=-N -l" -o secrets-provider ./cmd/secrets-provider

# =================== BASE MAIN CONTAINER ===================
# this layer is used to prepare a common layer for both debug and release containers
FROM alpine:latest AS secrets-provider-base
LABEL maintainer="CyberArk Software Ltd."

# Ensure openssl development libraries are always up to date
RUN apk add --no-cache openssl-dev

RUN apk add -u --no-cache shadow libc6-compat && \
    # Add limited user
    groupadd -r secrets-provider \
             -g 777 && \
    useradd -c "secrets-provider runner account" \
            -g secrets-provider \
            -u 777 \
            -m \
            -r \
            secrets-provider && \
    # Ensure plugin dir is owned by secrets-provider user
    mkdir -p /usr/local/lib/secrets-provider /etc/conjur/ssl /run/conjur /conjur/status && \
    # Use GID of 0 since that is what OpenShift will want to be able to read things
    chown secrets-provider:0 /usr/local/lib/secrets-provider \
                           /etc/conjur/ssl \
                           /run/conjur \
                           /conjur/status && \
    # We need open group permissions in these directories since OpenShift won't
    # match our UID when we try to write files to them
    chmod 770 /etc/conjur/ssl \
              /run/conjur && \
    chmod 777 /conjur/status

COPY --chown=secrets-provider:0 bin/run-time-scripts /usr/local/bin/

USER secrets-provider

# =================== RELEASE MAIN CONTAINER ===================
FROM secrets-provider-base AS secrets-provider

COPY --from=secrets-provider-builder /opt/secrets-provider-for-k8s/secrets-provider /usr/local/bin/

CMD [ "/usr/local/bin/secrets-provider"]

# =================== DEBUG MAIN CONTAINER ===================
FROM secrets-provider-base AS secrets-provider-debug

COPY --from=secrets-provider-builder-debug /go/bin/dlv /usr/local/bin/

COPY --from=secrets-provider-builder-debug /opt/secrets-provider-for-k8s/secrets-provider /usr/local/bin/

# Execute secrets provider wrapped with dlv debugger listening on port 40000 for remote debugger connection.
# Will wait indefinitely until a debugger is connected.
CMD ["/usr/local/bin/dlv",  \
     "--listen=:40000",     \
     "--headless=true",     \
     "--api-version=2",     \
     "--accept-multiclient",\
     "exec",                \
     "/usr/local/bin/secrets-provider"]

# =================== MAIN CONTAINER (REDHAT) ===================
FROM registry.access.redhat.com/ubi9/ubi AS secrets-provider-for-k8s-redhat
LABEL maintainer="CyberArk Software Ltd."

ARG VERSION

LABEL name="secrets-provider-for-k8s"
LABEL vendor="CyberArk"
LABEL version="$VERSION"
LABEL release="$VERSION"
LABEL summary="Store secrets in Conjur or DAP and consume them in your Kubernetes / Openshift application containers"
LABEL description="To retrieve the secrets from Conjur or DAP, the CyberArk Secrets Provider for Kubernetes runs as an \
 init container or separate application container and fetches the secrets that the pods require"

RUN yum -y distro-sync

# Add limited user
RUN groupadd -r secrets-provider \
             -g 777 && \
    useradd -c "secrets-provider runner account" \
            -g secrets-provider \
            -u 777 \
            -m \
            -r \
            secrets-provider && \
    # Ensure plugin dir is owned by secrets-provider user
    mkdir -p /usr/local/lib/secrets-provider /etc/conjur/ssl /run/conjur /conjur/status /licenses && \
    # Use GID of 0 since that is what OpenShift will want to be able to read things
    chown secrets-provider:0 /usr/local/lib/secrets-provider \
                           /etc/conjur/ssl \
                           /run/conjur \
                           /conjur/status && \
    # We need open group permissions in these directories since OpenShift won't
    # match our UID when we try to write files to them
    chmod 770 /etc/conjur/ssl \
              /run/conjur && \
    chmod 777 /conjur/status

COPY --from=secrets-provider-builder /opt/secrets-provider-for-k8s/secrets-provider /usr/local/bin/
COPY --chown=secrets-provider:0 bin/run-time-scripts /usr/local/bin/

COPY LICENSE.md /licenses

USER secrets-provider

ENTRYPOINT [ "/usr/local/bin/secrets-provider"]

