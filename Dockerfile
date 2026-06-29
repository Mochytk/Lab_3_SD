# syntax=docker/dockerfile:1

FROM golang:alpine AS builder

WORKDIR /app

# Instalar dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código
COPY . .

# Compilar todos los binarios
RUN go build -o bin/broker ./broker
RUN go build -o bin/gateway ./gateway
RUN go build -o bin/datanode ./datanode
RUN go build -o bin/client ./client

# Imagen final ligera
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bin/* ./
# Copiar los archivos CSV necesarios para el broker
COPY broker/*.csv ./

# El comando se especificará en docker-compose
