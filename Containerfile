# Фаза 1: Сборка приложения
FROM docker.io/library/golang:1.24 AS builder

# Установка рабочей директории
WORKDIR /app

# Копирование go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN echo "===> Adding trusted CA..." && \
    curl -sL http://ca.s7.ru/GroupS7_RootCA.crt | openssl x509 -inform der -outform pem > /usr/local/share/ca-certificates/group-root.crt && \
    curl -sL http://ca.s7.ru/SUBCAUSR02.crt | openssl x509 -inform der -outform pem > /usr/local/share/ca-certificates/group-subca-usr02.crt && \
    update-ca-certificates
# Загрузка и установка зависимостей
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка исполняемого файла
RUN CGO_ENABLED=0 GOOS=linux go build -o s3-syntetic cmd/main.go

# Фаза 2: Создание окончательного образа
FROM scratch

# Копирование собранного исполняемого файла из первой фазы
COPY --from=builder /app/s3-syntetic .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Экспорт метрик Prometheus на порту 8080
EXPOSE 8080

# Команда для запуска приложения
CMD ["/s3-syntetic"]