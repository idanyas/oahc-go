# Build Stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /oahc-go .

# Final Stage
FROM alpine:latest
COPY --from=builder /oahc-go /oahc-go
ENTRYPOINT ["/oahc-go"]