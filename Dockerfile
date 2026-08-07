FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git

# Initialize module files if they are missing
RUN go mod init exfil-server || true
RUN go get github.com/valyala/fasthttp@v1.52.0

COPY . .
RUN go build -o exfil-server main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/exfil-server .
EXPOSE 8080
ENTRYPOINT ["./exfil-server"]
