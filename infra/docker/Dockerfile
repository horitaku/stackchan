# syntax=docker/dockerfile:1.7

FROM node:lts-alpine AS webui-builder
WORKDIR /workspace/server/webui
COPY server/webui/package.json server/webui/package-lock.json ./
RUN npm ci
COPY server/webui/ ./
RUN npm run build

FROM golang:1.24-alpine AS server-builder
WORKDIR /workspace/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/cmd ./cmd
COPY server/internal ./internal
# WebUI のビルド成果物を実行時に参照できるようにコピーします。
COPY --from=webui-builder /workspace/server/webui/dist ./webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/stackchan-server ./cmd/stackchan-server

FROM gcr.io/distroless/static-debian12 AS runtime
WORKDIR /app
COPY --from=server-builder /out/stackchan-server /app/stackchan-server
COPY --from=server-builder /workspace/server/webui/dist /app/webui/dist
ENV APP_ENV=production
ENV SERVER_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/app/stackchan-server"]
