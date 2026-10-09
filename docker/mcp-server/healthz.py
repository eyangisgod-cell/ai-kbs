# AI-KBS MCP server — T0.3 healthz stub.
#
# Real MCP transports (stdio / SSE / HTTP) and tool definitions land in
# T2.10/2.11. This stub mirrors the SAG bridge stub so both placeholder
# images share the same contract: respond 200 on /healthz.

import sys
from http.server import BaseHTTPRequestHandler, HTTPServer


class Healthz(BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802 (stdlib HTTP verb)
        if self.path.startswith("/healthz"):
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"status":"ok","service":"ai-kbs-mcp-server","version":"0.1.0"}')
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):  # noqa: A002 - stdlib signature
        return


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8081
    HTTPServer(("0.0.0.0", port), Healthz).serve_forever()