FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS server
WORKDIR /src
COPY server/ ./
COPY --from=web /web/dist/ ./static/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /navigator .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /navigator /navigator
ENV DB_PATH=/data/navigator.db
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/navigator"]
