FROM golang:1.17.2 as build

WORKDIR /src/cmd

COPY go.mod ./
COPY go.sum ./
RUN go mod download
ENV GOARCH="amd64"
ENV CGO_ENABLED=0

COPY *.go ./

RUN go build -o s3-rate-limit-experiment

FROM alpine:3.13
RUN apk add --no-cache ca-certificates libcap

COPY --from=build /src/cmd/s3-rate-limit-experiment /usr/bin/s3-rate-limit-experiment

EXPOSE 8080
ENTRYPOINT [ "/usr/bin/s3-rate-limit-experiment" ]
