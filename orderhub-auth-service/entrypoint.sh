#!/bin/sh
set -e

echo "Waiting for database to be ready..."
# Ждем доступности базы данных (простая проверка)
sleep 5

echo "Running database migrations..."
/app/migrate

echo "Starting auth service..."
exec "$@"
