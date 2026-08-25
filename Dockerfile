FROM golang:1.26.7-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/4caster ./cmd

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/4caster /app/4caster
USER nonroot:nonroot
ENTRYPOINT ["/app/4caster"]
