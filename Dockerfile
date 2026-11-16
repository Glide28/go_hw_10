FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./gateway/cmd/gateway && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ledger ./ledger/cmd/ledger

FROM alpine:3.22
RUN adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=build /out/gateway /app/gateway
COPY --from=build /out/ledger /app/ledger
USER appuser
EXPOSE 8080 9090
