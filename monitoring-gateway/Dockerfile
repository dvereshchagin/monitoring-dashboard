FROM golang:1.25.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/monitoring-gateway ./cmd/monitoring-gateway

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /bin/monitoring-gateway /monitoring-gateway

EXPOSE 8082

USER nonroot:nonroot

ENTRYPOINT ["/monitoring-gateway"]
