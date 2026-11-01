"""Stand-in for core-api, tracking-api, ai-assistant and web (Doc 7 security harness).
Echoes the service name and all request headers. Extra test paths:
  /test/slow  waits 4 s before answering (timeout check)
  /test/hits  how many requests reached /test/slow (no-retry check)
  .../stream  SSE stream that stays open 3 s. Test use only."""
import json, sys, time, threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

NAME, PORT = sys.argv[1], int(sys.argv[2])
HITS = {"slow": 0}; LOCK = threading.Lock()

class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def log_message(self, *a):
        pass
    def _send(self, code, body, ctype="application/json"):
        data = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("X-Powered-By", "stub")
        self.end_headers()
        self.wfile.write(data)
    def _echo(self):
        n = int(self.headers.get("Content-Length") or 0)
        if n:
            self.rfile.read(n)
        if self.path.endswith("/test/slow"):
            with LOCK: HITS["slow"] += 1
            time.sleep(4)
            return self._send(200, '{"late":true}')
        if self.path.endswith("/test/hits"):
            return self._send(200, json.dumps(HITS))
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
        self._send(200, json.dumps({"service": NAME, "path": self.path, "bytes": n,
                                    "headers": {k.lower(): v for k, v in self.headers.items()}}))
    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = _echo

ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
