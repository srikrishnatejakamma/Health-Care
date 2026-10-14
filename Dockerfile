FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /out/workflow-worker ./cmd/workflow-worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/integration-worker ./cmd/integration-worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -o /out/token ./cmd/token

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S healthcare && adduser -S -G healthcare healthcare
WORKDIR /app
COPY --from=build /out/ /app/
USER healthcare
EXPOSE 8080 9090
ENTRYPOINT ["/app/api"]
