# AI-KBS SAG bridge — T0.3 placeholder image
#
# The real FastAPI app (LanceDB + SAG indexer/retriever/explainer) ships in
# T2.6; T0.3 only needs a container that responds on :8090/healthz so the
# docker-compose `depends_on: service_healthy` gate resolves cleanly during
# the M0 bootstrap.
#
# This image exposes a tiny HTTP server that returns 200 on /healthz and
# nothing else. The container is rebuilt and replaced by the real
# `sag-bridge/` image in T2.6 — the compose contract (port 8090, /healthz
# path, env var AIKBS_SAG_URL) does not change.

FROM python:3.11-slim

WORKDIR /app

# stdlib-only HTTP server keeps the image lean; no pip install required.
COPY healthz.py /app/healthz.py

EXPOSE 8090

# Disable Python's bytecode generation in the placeholder image.
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Plain stdlib http.server. The server listens on 0.0.0.0:8090 and
# responds 200 to any path under /healthz.
CMD ["python", "-u", "healthz.py"]