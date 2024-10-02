FROM golang:1.22.2-alpine as BUILDER
ENV GOFLAGS="-buildvcs=false"
RUN apk add --no-cache ca-certificates make gcc musl-dev linux-headers git
WORKDIR /workdir
COPY . .
RUN go build -o eth-monitoring

FROM alpine:3.17
WORKDIR /app
COPY --from=BUILDER /workdir/eth-monitoring ./
ENTRYPOINT [ "/app/eth-monitoring" ]
