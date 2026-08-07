FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata
RUN go mod init exfil-server
COPY . .
RUN go get github.com/valyala/fasthttp@v1.52.0
RUN go build -o exfil-server main.go

FROM scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/exfil-server /exfil-server
EXPOSE 8080
ENTRYPOINT ["/exfil-server"]
