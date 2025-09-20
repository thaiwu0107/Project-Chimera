FROM golang:1.22-alpine AS builder

RUN set -eux; \
    apk update && apk add git

ENV GOPATH /go
ENV GO_WORKDIR $GOPATH/src/project-chimera/s8-autopsy
ENV GOFLAGS=-mod=vendor
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR $GO_WORKDIR
ADD . $GO_WORKDIR
RUN go build -ldflags "-X 'main.GitCommitNum=git rev-parse --short=12 HEAD' -o s8-autopsy ."

FROM asia-east1-docker.pkg.dev/cyr-gcp/cyr1/centos:7
ENV GOPATH /go
ENV GO_WORKDIR $GOPATH/src/project-chimera/s8-autopsy
WORKDIR /app
COPY --from=builder $GO_WORKDIR/s8-autopsy s8-autopsy
COPY env.yaml .
COPY config.yaml .

EXPOSE 8088
CMD ["./s8-autopsy"]


