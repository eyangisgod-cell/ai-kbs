# AI-KBS MCP server — T0.3 placeholder image
#
# The real FastMCP server (stdio / SSE / HTTP transports, OAuth2, frozen
# tool schemas) ships in T2.10/2.11. T0.3 needs only a container that the
# `mcp-server` service can depend on, with an HTTP healthz on :8081 so the
# docker-compose stack has a deterministic startup gate. The real image
# replaces this one in T2.10 without changing the compose contract.

FROM python:3.11-slim

WORKDIR /app

COPY healthz.py /app/healthz.py

EXPOSE 8081

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

CMD ["python", "-u", "healthz.py", "8081"]