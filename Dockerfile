FROM goboring/golang:1.12.17b4
MAINTAINER Conjur Inc

RUN apt-get update && apt-get install -y jq

RUN go get -u github.com/jstemmer/go-junit-report && \
    go get github.com/smartystreets/goconvey

ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /opt/secrets-provider-for-k8s
EXPOSE 8080
