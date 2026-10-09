# AI-KBS SAG bridge — T0.3 healthz stub.
#
# The T0.3 docker-compose stack needs the `sag-bridge` container to come up
# and respond on :8090/healthz so `depends_on: { sag-bridge: { condition:
# service_healthy } }` resolves. Real SAG logic (LanceDB indexer/retriever)
# lands in T2.6; this file is replaced in that task.

from http.server import BaseHTTPRequestHandler, HTTPServer


class Healthz(BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802 (stdlib HTTP verb)
        if self.path.startswith("/healthz"):
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"status":"ok","service":"ai-kbs-sag-bridge","version":"0.1.0"}')
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):  # noqa: A002 - stdlib signature
        # Route access logs through stderr; docker-compose captures them.
        return


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", 8090), Healthz).serve_forever()