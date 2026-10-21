#!/bin/sh
set -e
mkdir -p /app/data /app/uploads
if [ ! -f /app/data/app.db ]; then
  python /app/scripts/init_db.py
fi
exec gunicorn -b 0.0.0.0:8080 "run:app"
