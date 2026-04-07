# Multi-stage Dockerfile for all-in-one project
# Stage 1: Build the frontend
FROM node:22-alpine AS frontend-builder

WORKDIR /app/web

# Copy package files
COPY web/package*.json ./

# Install dependencies
RUN npm ci

# Copy frontend source
COPY web/ ./

# Build the frontend
RUN npm run build

# Stage 2: Build the backend
FROM golang:1.25-alpine AS backend-builder

# Install build dependencies (gcc, musl-dev for CGO/sqlite3)
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with CGO enabled and static linking
# Static linking (-extldflags '-static') ensures the binary has no dynamic
# dependencies on musl (Alpine) or glibc, making it compatible with the
# distroless/base-debian12 runtime image.
ENV CGO_ENABLED=1
ENV GOOS=linux
RUN go build -ldflags="-w -s -extldflags '-static'" -o /app/bin/all-in-one cmd/all-in-one/main.go

# Stage 3: Final runtime image
FROM gcr.io/distroless/base-debian12:latest

WORKDIR /app

# Copy the built backend binary
COPY --from=backend-builder /app/bin/all-in-one /app/all-in-one

# Copy configuration file
COPY config/config.yml /app/config/config.yml

# Copy database migrations
COPY db/migrations /app/db/migrations

# Copy the built frontend static files (adapter-static output)
COPY --from=frontend-builder /app/web/build /app/web/build

# Expose the application port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/app/all-in-one"]
CMD ["server"]
