ARG GO_VERSION=1.22.5

FROM golang:${GO_VERSION} AS dev
WORKDIR /app
COPY . /app
RUN go mod download

FROM golang:${GO_VERSION} AS vuln
WORKDIR /app
COPY --from=dev /go/ /go
COPY --from=dev /app/ /app
RUN ./xc vuln

FROM golang:${GO_VERSION} AS vet
WORKDIR /app
COPY --from=dev /go/ /go
COPY --from=dev /app/ /app
RUN ./xc vet

FROM golang:${GO_VERSION} AS test
WORKDIR /app
COPY --from=dev /go/ /go
COPY --from=dev /app/ /app
RUN ./xc test

FROM golang:${GO_VERSION} AS build-body
ARG os
ARG arch
ENV os=$os
ENV arch=$arch
WORKDIR /app
COPY --from=dev /go/ /go
COPY --from=dev /app/ /app
RUN ./xc buildx

FROM scratch AS build
COPY --from=build-body /app/dist/rpath /rpath
