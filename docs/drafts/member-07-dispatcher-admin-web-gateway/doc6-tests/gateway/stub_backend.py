"""Tiny stand-in for core-api, tracking-api, ai-assistant and web.
Echoes which service answered so routing can be checked. Test use only."""
import json, sys, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

NAME, PORT = sys.argv[1], int(sys.argv[2])

class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def log_message(self, *a):
        pass
    def _send(self, code, body, ctype="application/json", extra=None):
        data = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        for k, v in (extra or {}).items():
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(data)
    def _echo(self):
        n = int(self.headers.get("Content-Length") or 0)
        if n:
            self.rfile.read(n)
        if self.path.endswith("/test/401"):
            return self._send(401, '{"error":{"code":"AUTH_TOKEN_EXPIRED","message":"expired"}}', extra={"WWW-Authenticate": 'Bearer error="invalid_token"'})
        if self.path.endswith("/test/403"):
            return self._send(403, '{"error":{"code":"ACCESS_DENIED","message":"Forbidden"}}')
        if self.path.endswith("/stream"):
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.send_header("X-Accel-Buffering", "no")
            self.end_headers()
            for i in range(3):
                self.wfile.write(f"event: milestone\ndata: {{\"n\":{i}}}\n\n".encode())
                self.wfile.flush()
                time.sleep(1)
            self.close_connection = True
            return
        self._send(200, json.dumps({"service": NAME, "path": self.path,
                                    "authorization": self.headers.get("Authorization"),
                                    "x_api_key": self.headers.get("X-API-Key"),
                                    "bytes": n}))
    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = _echo

ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
