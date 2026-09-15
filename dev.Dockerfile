# Dev image - restarts the whole process on code changes, the same way
# `air` rebuilt and restarted the Go binary (main.py does its own env-var
# driven wiring on every startup, so a full-process restart - not an
# in-process module reload - is the right match here).
FROM python:3.13-slim

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# We do NOT copy code here; docker-compose will mount it
EXPOSE 8080

CMD ["watchfiles", "python -m app.main", "app"]
