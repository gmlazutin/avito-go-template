FROM golang:1.27 AS builder
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /trip-service ./cmd/trip-service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /trip-service /trip-service
USER nonroot:nonroot
ENTRYPOINT ["/trip-service"]
