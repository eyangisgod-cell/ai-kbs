# AI-KBS Langfuse dashboard bootstrap sidecar (T0.7).
#
# Runs once on first `docker compose up`. Imports dashboards/ai-kbs-default.json
# into the Langfuse v3 instance via its internal NextAuth + REST API.
# Idempotent: skips if the dashboard already exists by name.
#
# Image is intentionally minimal: python:3.12-slim + requests + urllib3.
FROM python:3.12-slim

WORKDIR /app

# Copy the dashboard JSON and the bootstrap script.
COPY dashboards/ /dashboards/
COPY bootstrap.py /app/bootstrap.py
COPY requirements.txt /app/requirements.txt

RUN pip install --no-cache-dir -r /app/requirements.txt

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    LANGFUSE_URL=http://langfuse:3000 \
    DASHBOARD_PATH=/dashboards/ai-kbs-default.json \
    HEALTH_TIMEOUT_S=120

CMD ["python", "/app/bootstrap.py"]
