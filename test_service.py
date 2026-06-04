#!/usr/bin/env python3
"""Simple test HTTP service for FlagMate testing."""

import http.server
import json
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler


class TestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/api/data':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            response = json.dumps({"message": "Hello from test service", "flag": "test{secret_flag_123}"})
            self.wfile.write(response.encode())
        elif self.path == '/api/secret':
            self.send_response(200)
            self.send_header('Content-Type', 'text/plain')
            self.end_headers()
            self.wfile.write(b"super_secret_data_456")
        elif self.path == '/api/error':
            self.send_response(500)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({"error": "Internal Server Error"}).encode())
        else:
            self.send_response(404)
            self.send_header('Content-Type', 'text/plain')
            self.end_headers()
            self.wfile.write(b"Not Found")

    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length) if content_length > 0 else b''
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = json.dumps({"received": body.decode(), "status": "ok"})
        self.wfile.write(response.encode())

    def log_message(self, format, *args):
        pass  # Suppress logging


def run_server(port=18080):
    server = HTTPServer(('0.0.0.0', port), TestHandler)
    print(f"Test server running on http://0.0.0.0:{port}")
    server.serve_forever()


if __name__ == '__main__':
    run_server()
