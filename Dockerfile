# This is a multi-stage Dockerfile and requires >= Docker 17.05
# https://docs.docker.com/engine/userguide/eng-image/multistage-build/
FROM golang:1.23.2 as builder

ENV GOPROXY http://proxy.golang.org

RUN mkdir -p /src/catalog-api
WORKDIR /src/catalog-api

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download && go mod verify

ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /bin/app cmd/catalog-api/main.go

FROM alpine
RUN apk add --no-cache bash
RUN apk add --no-cache ca-certificates

WORKDIR /bin/

COPY --from=builder /bin/app .

# Uncomment to run the binary in "production" mode:
# ENV GO_ENV=production

# Bind the app to 0.0.0.0 so it can be seen from outside the container
ENV BIND_ADDRESS=0.0.0.0:8000
ENV REDIS_HOST ""
ENV REDIS_PORT "6379"
ENV MONGODB_URI ""
ENV MONGODB_DATABASE "catalog-api"
ENV GIN_MODE "release"

EXPOSE 8000

# Uncomment to run the migrations before running the binary:
# CMD /bin/app migrate; /bin/app
CMD exec /bin/app
