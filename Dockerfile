FROM golang:1.24.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags=-static" \
    -trimpath \
    -o tcp-proxy ./main.go

FROM gcr.io/distroless/static:nonroot

WORKDIR /app

COPY --from=builder /app/tcp-proxy ./tcp-proxy

EXPOSE 15432 16379

CMD ["./tcp-proxy"]
