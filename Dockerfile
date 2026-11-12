# One image per Go service, selected with --build-arg SERVICE=control-api|telemetry|swarmctl.
# Build context is the PARENT directory (swarm-ops/) so the sibling contracts module resolves
# through the `replace` directive in go.mod:
#   docker build -f swarmops-backend/Dockerfile --build-arg SERVICE=control-api ..
FROM golang:1.24-alpine AS build
COPY swarmops-contracts/gen/go /src/swarmops-contracts/gen/go
COPY swarmops-backend /src/swarmops-backend
WORKDIR /src/swarmops-backend
# go.sum is generated on first `go mod tidy`; commit it and this becomes a no-op.
RUN test -f go.sum || go mod tidy
RUN go mod download
ARG SERVICE
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot
ENTRYPOINT ["/app"]
