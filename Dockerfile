# Используем официальный образ Go
FROM golang:1.21-alpine AS builder

# Устанавливаем SQLite и зависимости
RUN apk add --no-cache gcc musl-dev sqlite

# Копируем код в контейнер
WORKDIR /app
COPY . .

# Собираем приложение
RUN go mod download
RUN go build -o /pushup-bot

# Финальный образ (меньше размером)
FROM alpine:latest

# Устанавливаем SQLite (для работы с БД)
RUN apk add --no-cache sqlite

# Копируем бинарник из builder
COPY --from=builder /pushup-bot /pushup-bot
COPY --from=builder /app/pushups.db /pushups.db

# Запускаем бота
CMD ["/pushup-bot"]