# Build backend.
FROM golang:1.15 as build-backend

ARG VERSION
ARG COMMIT
ARG DATE

# Default build variables. Build for linux with no CGO extensions.
ENV CGO_ENABLED=0
ENV GOOS=linux

# Create run user.
RUN useradd -u 10000 air-alert

WORKDIR /go/src
COPY . .

RUN go build -ldflags="-s -w -X 'main.version=${VERSION}' -X 'main.commit=${COMMIT}' -X 'main.date=${DATE}'" .

# Build frontend components.
FROM node:12-slim as build-frontend

WORKDIR /frontend
COPY . .

WORKDIR /frontend/static

RUN npm install && \
    npm run build

FROM scratch

# Copy required system files.
COPY --from=build-backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build-backend /etc/passwd /etc/passwd

# Copy backend executable.
COPY --from=build-backend /go/src/air-alert /bin/air-alert

# Copy frontend components.
COPY --from=build-frontend /frontend/static/dist /static
COPY --from=build-frontend /frontend/templates /templates

USER air-alert

# Configuration volume. Contains only the application configuration file.
VOLUME [ "/config.toml" ]

ENTRYPOINT [ "/bin/air-alert" ]