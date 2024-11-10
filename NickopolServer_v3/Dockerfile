# Используем базовый образ Go
FROM golang:1.21.3

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Копируем go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./

# Устанавливаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Сборка бинарного файла
RUN GOOS=linux GOARCH=amd64 go build -o main main.go

# Открываем порт 80
EXPOSE 80

# Команда для запуска приложения
CMD ["./main"]