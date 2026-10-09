FROM golang:1.26.9-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache tzdata && addgroup -S app && adduser -S app -G app
USER app
COPY --from=builder /api /api
EXPOSE 8080
CMD ["/api"]
