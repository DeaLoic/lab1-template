# Сборка приложения
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/app .

# Итоговый образ
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
COPY --from=build /bin/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
