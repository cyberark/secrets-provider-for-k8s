FROM golang:1.13 as secrets-provider-builder
MAINTAINER CyberArk Software, Ltd.

ENV GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0

COPY . /opt/secrets-provider-for-k8s

WORKDIR /opt/secrets-provider-for-k8s

EXPOSE 8080

RUN apt-get update && apt-get install -y jq

RUN go get -u github.com/jstemmer/go-junit-report && \
    go get github.com/smartystreets/goconvey

RUN go build -a -installsuffix cgo -o secrets-provider ./cmd/secrets-provider

# =================== BUSYBOX LAYER ===================
# this layer is used to get binaries into the main container
FROM busybox

# =================== MAIN CONTAINER ===================
FROM alpine:3.11 as secrets-provider
MAINTAINER CyberArk Software, Ltd.

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

COPY --from=secrets-provider-builder /opt/secrets-provider-for-k8s/secrets-provider /usr/local/bin/

ENTRYPOINT [ "/usr/local/bin/secrets-provider"]

