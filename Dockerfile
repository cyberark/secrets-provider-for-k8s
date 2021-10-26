# =================== BASE BUILD LAYER ===================
# this layer is used to prepare a common layer for both debug and release builds
FROM golang:1.16 as secrets-provider-builder-base
MAINTAINER CyberArk Software Ltd.

ENV GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0

RUN go get -u github.com/jstemmer/go-junit-report && \
    go get github.com/smartystreets/goconvey

WORKDIR /opt/secrets-provider-for-k8s

EXPOSE 8080

COPY go.mod go.sum ./

# Add a layer of prefetched modules so the modules are already cached in case we rebuild
RUN go mod download

# =================== RELEASE BUILD LAYER ===================
# this layer is used to build the release binaries
FROM secrets-provider-builder-base as secrets-provider-builder

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
FROM secrets-provider-builder-base as secrets-provider-builder-debug

# Build Delve - debugging tool for Go
RUN go get github.com/go-delve/delve/cmd/dlv

# Expose port 40000 for debugging
EXPOSE 40000

COPY . .

# Build debug flavor without compilation optimizations using "all=-N -l"
RUN go build -a -installsuffix cgo -gcflags="all=-N -l" -o secrets-provider ./cmd/secrets-provider

# =================== BUSYBOX LAYER ===================
# this layer is used to get binaries into the main container
FROM busybox

# =================== BASE MAIN CONTAINER ===================
# this layer is used to prepare a common layer for both debug and release containers
FROM alpine:3.14 as secrets-provider-base
MAINTAINER CyberArk Software Ltd.

# Ensure openssl development libraries are always up to date
RUN apk add --no-cache openssl-dev

# copy a few commands from busybox
COPY --from=busybox /bin/tar /bin/tar
COPY --from=busybox /bin/sleep /bin/sleep
COPY --from=busybox /bin/sh /bin/sh
COPY --from=busybox /bin/ls /bin/ls
COPY --from=busybox /bin/id /bin/id
COPY --from=busybox /bin/whoami /bin/whoami
COPY --from=busybox /bin/mkdir /bin/mkdir
COPY --from=busybox /bin/chmod /bin/chmod
COPY --from=busybox /bin/cat /bin/cat

RUN apk add -u shadow libc6-compat && \
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
    mkdir -p /usr/local/lib/secrets-provider /etc/conjur/ssl /run/conjur && \
    # Use GID of 0 since that is what OpenShift will want to be able to read things
    chown secrets-provider:0 /usr/local/lib/secrets-provider \
                           /etc/conjur/ssl \
                           /run/conjur && \
    # We need open group permissions in these directories since OpenShift won't
    # match our UID when we try to write files to them
    chmod 770 /etc/conjur/ssl \
              /run/conjur

USER secrets-provider

# =================== RELEASE MAIN CONTAINER ===================
FROM secrets-provider-base as secrets-provider

COPY --from=secrets-provider-builder /opt/secrets-provider-for-k8s/secrets-provider /usr/local/bin/

CMD [ "/usr/local/bin/secrets-provider"]

# =================== DEBUG MAIN CONTAINER ===================
FROM secrets-provider-base as secrets-provider-debug

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
FROM registry.access.redhat.com/ubi8/ubi as secrets-provider-for-k8s-redhat
MAINTAINER CyberArk Software Ltd.

ARG VERSION

LABEL name="secrets-provider-for-k8s"
LABEL vendor="CyberArk"
LABEL version="$VERSION"
LABEL release="$VERSION"
LABEL summary="Store secrets in Conjur or DAP and consume them in your Kubernetes / Openshift application containers"
LABEL description="To retrieve the secrets from Conjur or DAP, the CyberArk Secrets Provider for Kubernetes runs as an \
 init container or separate application container and fetches the secrets that the pods require"

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
    mkdir -p /usr/local/lib/secrets-provider /etc/conjur/ssl /run/conjur /licenses && \
    # Use GID of 0 since that is what OpenShift will want to be able to read things
    chown secrets-provider:0 /usr/local/lib/secrets-provider \
                           /etc/conjur/ssl \
                           /run/conjur && \
    # We need open group permissions in these directories since OpenShift won't
    # match our UID when we try to write files to them
    chmod 770 /etc/conjur/ssl \
              /run/conjur

COPY --from=secrets-provider-builder /opt/secrets-provider-for-k8s/secrets-provider /usr/local/bin/

COPY LICENSE.md /licenses

USER secrets-provider

ENTRYPOINT [ "/usr/local/bin/secrets-provider"]

