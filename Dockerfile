FROM golang:1.13 as secrets-provider-builder
MAINTAINER Conjur Inc

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
FROM scratch as secrets-provider
MAINTAINER CyberArk Software, Inc.

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

# allow anyone to write to this dir, container may not run as root
RUN mkdir -p /etc/conjur/ssl && chmod 777 /etc/conjur/ssl

COPY --from=secrets-provider-builder /opt/secrets-provider-for-k8s/secrets-provider /bin

CMD ["secrets-provider"]
