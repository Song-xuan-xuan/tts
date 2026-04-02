FROM golang:1.24-alpine AS build
WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH

COPY gateway/go.mod gateway/go.sum ./
RUN go mod download

COPY gateway/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/tts-gateway ./cmd/tts-gateway

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/tts-gateway /app/tts-gateway
ENTRYPOINT ["/app/tts-gateway"]
