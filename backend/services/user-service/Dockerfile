FROM golang:1.23.2-alpine AS builder

ARG SERVICE_NAME=user-service
ARG APP_DIR=/app

WORKDIR ${APP_DIR}

COPY common/ ${APP_DIR}/common/
COPY services/${SERVICE_NAME}/ ${APP_DIR}/

RUN apk add --no-cache git && \
    go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${SERVICE_NAME} \
    -ldflags "-s -w -X main.version=${VERSION:-dev}" ./cmd/

FROM alpine:3.19 AS runtime

ARG SERVICE_NAME=user-service

RUN apk add --no-cache ca-certificates tzdata curl && \
    rm -rf /var/cache/apk/* && \
    adduser -D -H -s /sbin/nologin -u 1000 appuser

WORKDIR /home/appuser

COPY --from=builder /app/${SERVICE_NAME} ./
COPY --from=builder /app/config.yaml ./

RUN chmod +x ${SERVICE_NAME}

USER appuser

EXPOSE 5001 8081

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1

ENV SERVICE_NAME=${SERVICE_NAME}
ENV GIN_MODE=release

ENTRYPOINT ["./${SERVICE_NAME}"]
