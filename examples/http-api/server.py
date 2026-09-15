"""Read-only JSON API fixture. Run only as a local example, not in production."""
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit

CUSTOMERS = [
    {"id": 9007199254740993, "name": "Acme", "region": "east"},
    {"id": 2, "name": "Northwind", "region": "east"},
    {"id": 3, "name": "Contoso", "region": "west"},
]


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        url = urlsplit(self.path)
        if url.path != "/customers":
            self.send_error(404)
            return
        query = parse_qs(url.query)
        self.customers(query.get("region", ["east"])[0], query)

    def do_POST(self):
        url = urlsplit(self.path)
        if url.path != "/customers/search":
            self.send_error(404)
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 < length <= 8192:
                raise ValueError("invalid body size")
            body = json.loads(self.rfile.read(length))
            self.customers(body["filter"]["region"], parse_qs(url.query))
        except (ValueError, KeyError, TypeError):
            self.send_error(400)

    def customers(self, region, query):
        rows = [row for row in CUSTOMERS if row["region"] == region]
        token = query.get("cursor", [""])[0]
        if token not in ("", "page-2"):
            self.send_error(400)
            return
        offset = 1 if token == "page-2" else 0
        response = {"data": rows[offset:offset + 1], "next_cursor": "page-2" if len(rows) > 1 and not token else None}
        data = json.dumps(response).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
