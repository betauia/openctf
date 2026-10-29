FROM golang:1.27.3 AS base
WORKDIR /app
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM base AS development
CMD ["go", "run", "./cmd/server"]

FROM base AS build
RUN go build -trimpath -ldflags="-s -w" -o /out/backend ./cmd/server

# Runs as root so it can use the mounted Docker socket to spawn challenge instances.
FROM gcr.io/distroless/static-debian12 AS production
COPY --from=build /out/backend /backend
EXPOSE 8000
ENTRYPOINT ["/backend"]
