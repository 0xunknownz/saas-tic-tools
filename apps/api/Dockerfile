# syntax=docker/dockerfile:1
# Build context: the repository root. The migration runner is a separate Go
# module under deploy/migrate, so it gets its own stage.
FROM golang:1.26-alpine AS app
# git is needed to fetch the private Foundation module from GitHub, and Go must be told that the
# module path is private so it does not ask a public proxy for it.
RUN apk add --no-cache git
WORKDIR /src/apps/api
ENV CGO_ENABLED=0 GOPRIVATE=github.com/0xunknownz/*
# Modules resolve during the build, not in a separate download layer: a build-time vendor tree
# (offline and release builds, docs/foundation-modularization.md 5) is then used automatically,
# and a development replace path does not have to exist inside the image. Remote private modules
# are fetched with the builder's credentials (5.3).
COPY apps/api/ ./
# The token is mounted for this step only (BuildKit secret, design 5.3): it never lands in a layer,
# the build history, an ARG or an ENV. Without a secret (a build-time vendor tree, offline delivery)
# the step just builds.
RUN --mount=type=secret,id=foundation_token,required=false \
    if [ -s /run/secrets/foundation_token ]; then \
      git config --global url."https://x-access-token:$(cat /run/secrets/foundation_token)@github.com/".insteadOf "https://github.com/"; \
    fi; \
    go build -trimpath -o /out/api ./cmd/api \
 && go build -trimpath -o /out/worker ./cmd/worker

FROM golang:1.26-alpine AS migrate
WORKDIR /src/deploy/migrate
ENV CGO_ENABLED=0
COPY deploy/migrate/go.mod deploy/migrate/go.sum ./
RUN go mod download
COPY deploy/migrate/ ./
RUN go build -trimpath -o /out/migrate .

FROM alpine:3
RUN adduser -D -H -u 10001 app
COPY --from=app /out/ /app/
COPY --from=migrate /out/migrate /app/migrate
USER 10001
EXPOSE 8080
CMD ["/app/api"]
