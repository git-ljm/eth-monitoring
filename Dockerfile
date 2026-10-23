FROM golang:1.26-alpine AS builder
ENV CGO_ENABLED=0 GOFLAGS="-buildvcs=false"
WORKDIR /workdir
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN go build -trimpath -ldflags="-s -w" -o eth-monitoring

FROM alpine:3.22
WORKDIR /app
COPY --from=builder /workdir/eth-monitoring ./
USER 65534:65534
ENTRYPOINT ["/app/eth-monitoring"]
