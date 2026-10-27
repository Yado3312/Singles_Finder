# syntax=docker/dockerfile:1

FROM node:22-alpine AS frontend
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY angular.json tsconfig.json tsconfig.app.json ./
COPY src/ ./src/
COPY public/ ./public/
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /build
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/singlesfinder .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /out/singlesfinder /app/singlesfinder
COPY --from=frontend /build/dist/singles-finder/browser/ /app/web/
ENV ADDR=0.0.0.0:8080 WEB_DIR=/app/web MONGODB_DATABASE=singlesfinder
USER 10001:10001
EXPOSE 8080
CMD ["/app/singlesfinder"]
