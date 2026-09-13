# Build an immutable Linux binary. The runtime image deliberately contains no
# Go toolchain or project source code.
FROM golang:1.26-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gojo-server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed-problems ./cmd/seed_problems \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bootstrap-admin ./cmd/bootstrap_admin

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S gojo \
    && adduser -S -D -H -G gojo gojo

WORKDIR /app

COPY --from=builder /out/gojo-server /app/gojo-server
COPY --from=builder /out/seed-problems /app/seed-problems
COPY --from=builder /out/bootstrap-admin /app/bootstrap-admin
# The repository ships a secret-free production template. Runtime secrets are
# supplied through GOJO_* environment variables by Docker Compose.
COPY config/config.production.example.yaml /app/config/config.production.yaml

# The Docker socket group is supplied at runtime by Compose. The application
# itself does not need root privileges.
USER gojo

EXPOSE 8080

CMD ["/app/gojo-server"]
