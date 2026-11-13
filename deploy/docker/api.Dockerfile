# syntax=docker/dockerfile:1
# Test-environment image: API, worker and the deployment migration runner.
# Build it from the Product root:  docker compose -f deploy/compose/test.yaml build
FROM golang:1.26-alpine AS build
# git is needed to fetch the private Foundation module from GitHub, and Go must be told that the
# module path is private so it does not ask a public proxy for it.
RUN apk add --no-cache git
ENV CGO_ENABLED=0 GOPRIVATE=github.com/0xunknownz/*
WORKDIR /src
COPY deploy/migrate/go.mod deploy/migrate/go.sum ./deploy/migrate/
RUN cd deploy/migrate && go mod download
# apps/api resolves its modules during the build so a build-time vendor tree is used automatically
# and a development replace path does not have to exist inside the image (5.3, 5).
COPY apps/api ./apps/api
COPY deploy/migrate ./deploy/migrate
# The Foundation module is private: mount the read-only token for this step only. It never lands in a
# layer, the build history, an ARG or an ENV; a vendor tree or an anonymous cache needs no secret.
RUN --mount=type=secret,id=foundation_token,required=false \
    if [ -s /run/secrets/foundation_token ]; then \
      git config --global url."https://x-access-token:$(cat /run/secrets/foundation_token)@github.com/".insteadOf "https://github.com/"; \
    fi; \
    cd apps/api && go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN cd deploy/migrate && go build -trimpath -ldflags="-s -w" -o /out/migrate .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
 && addgroup -S -g 10001 app \
 && adduser -S -u 10001 -G app -h /app app \
 && mkdir -p /app /data/objects /app/migrations \
 && chown -R app:app /app /data
COPY --from=build /out/api /out/worker /out/migrate /app/
# Written by "go tool saasctl build"; the migration runner applies exactly this plan before the API starts.
COPY release/migrations/plan.json /app/migrations/plan.json
COPY deploy/scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/api /app/worker /app/migrate /app/entrypoint.sh && chown -R app:app /app
USER app
WORKDIR /app
ENV APP_ENV=development \
    HTTP_ADDR=:8080 \
    PLAN_PATH=/app/migrations/plan.json \
    MIGRATE_ON_START=false
EXPOSE 8080
# No image-level HEALTHCHECK: the API and the worker share this image but only the API serves HTTP.
# compose/test.yaml gives the API (and the edge) their own health check.
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/api"]
