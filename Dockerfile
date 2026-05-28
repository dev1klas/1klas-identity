FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /app/bin/server ./bin/server
COPY --from=builder /app/bin/migrate ./bin/migrate
USER nonroot
EXPOSE 8080
CMD ["./bin/server"]
