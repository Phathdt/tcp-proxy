FROM golang:1.24.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o tcp-proxy ./main.go

FROM alpine:3.22

RUN apk --no-cache add ca-certificates

WORKDIR /app
RUN chown nobody:nobody /app
USER nobody:nobody

COPY --from=builder --chown=nobody:nobody /app/tcp-proxy .

EXPOSE 15432 16379

CMD ["./tcp-proxy"]
