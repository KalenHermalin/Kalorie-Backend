# Stage 1: Build (install deps into a venv so the runtime image stays slim)
FROM python:3.13-slim AS builder

WORKDIR /app

RUN python -m venv /venv
ENV PATH="/venv/bin:$PATH"

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Stage 2: Runtime
FROM python:3.13-slim

WORKDIR /app

COPY --from=builder /venv /venv
ENV PATH="/venv/bin:$PATH"

COPY app ./app
COPY migrations ./migrations

EXPOSE 8080

# Heroku will assign a dynamic $PORT; your code must use os.getenv("PORT")
CMD ["python", "-m", "app.main"]
