FROM golang:1.25-bookworm AS builder

WORKDIR /build

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . ./

# Install templ and generate templates
RUN go install github.com/a-h/templ/cmd/templ@v0.3.977
RUN templ generate

# Build the applications
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /out/monitoring-dashboard ./cmd/monitoring-dashboard-api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /out/release-analyzer ./cmd/release-analyzer

FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy binaries
COPY --from=builder /out/monitoring-dashboard /app/monitoring-dashboard
COPY --from=builder /out/release-analyzer /app/release-analyzer

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/monitoring-dashboard"]
