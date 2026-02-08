FROM golang:1.25-bookworm AS api-builder

WORKDIR /build

# Build monitoring-dashboard-api
COPY monitoring-dashboard-api/go.mod monitoring-dashboard-api/go.sum ./
RUN go mod download

COPY monitoring-dashboard-api/ ./

# Install templ and generate templates
RUN go install github.com/a-h/templ/cmd/templ@v0.3.977
RUN templ generate

# Build the applications
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /out/monitoring-dashboard ./cmd/monitoring-dashboard-api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /out/release-analyzer ./cmd/release-analyzer

FROM golang:1.25-bookworm AS gateway-builder

WORKDIR /build

# Build monitoring-gateway
COPY monitoring-gateway/go.mod monitoring-gateway/go.sum ./
RUN go mod download

COPY monitoring-gateway/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /out/monitoring-gateway ./cmd/monitoring-gateway

FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy all binaries
COPY --from=api-builder /out/monitoring-dashboard /app/monitoring-dashboard
COPY --from=api-builder /out/release-analyzer /app/release-analyzer
COPY --from=gateway-builder /out/monitoring-gateway /app/monitoring-gateway

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/monitoring-dashboard"]
