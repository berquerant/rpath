FROM golang:1.22.5 AS dev
WORKDIR /app
COPY *.go go.mod go.sum xc README.md /app/
COPY cmd/ /app/cmd
RUN go mod download

FROM golang:1.22.5 AS vuln
WORKDIR /app
COPY --from=dev /go/ /go
COPY --from=dev /app/ /app
RUN ./xc vuln

FROM golang:1.22.5 AS vet
WORKDIR /app
COPY --from=dev /go/ /go
COPY --from=dev /app/ /app
RUN ./xc vet

FROM golang:1.22.5 AS test
WORKDIR /app
COPY --from=dev /go/ /go
COPY --from=dev /app/ /app
RUN ./xc test

FROM golang:1.22.5 AS build-body
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
