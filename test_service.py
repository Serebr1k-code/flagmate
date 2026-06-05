#!/usr/bin/env python3
"""Simple test HTTP service for FlagMate testing."""

import base64
import hashlib
import json
from http.server import HTTPServer, BaseHTTPRequestHandler


class TestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/ws':
            self.handle_websocket()
        elif self.path == '/api/data':
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

    def handle_websocket(self):
        key = self.headers.get('Sec-WebSocket-Key', '')
        if not key:
            self.send_response(400)
            self.end_headers()
            return
        accept = base64.b64encode(hashlib.sha1((key + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode()).digest()).decode()
        self.send_response(101)
        self.send_header('Upgrade', 'websocket')
        self.send_header('Connection', 'Upgrade')
        self.send_header('Sec-WebSocket-Accept', accept)
        self.end_headers()
        self.wfile.write(websocket_frame(json.dumps({"hello": "websocket", "hint": "send exploit JSON"})))
        try:
            payload = read_websocket_text(self.rfile)
            if 'sploit' in payload or 'flag' in payload:
                response = {"ok": True, "flag": "test{websocket_flag_789}", "echo": payload}
            else:
                response = {"ok": True, "echo": payload}
            self.wfile.write(websocket_frame(json.dumps(response)))
        except Exception:
            pass

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


def websocket_frame(text):
    data = text.encode()
    if len(data) < 126:
        return bytes([0x81, len(data)]) + data
    return bytes([0x81, 126]) + len(data).to_bytes(2, 'big') + data


def read_websocket_text(stream):
    head = stream.read(2)
    if len(head) < 2:
        return ''
    length = head[1] & 0x7F
    if length == 126:
        length = int.from_bytes(stream.read(2), 'big')
    elif length == 127:
        length = int.from_bytes(stream.read(8), 'big')
    mask = stream.read(4)
    data = bytearray(stream.read(length))
    for i in range(len(data)):
        data[i] ^= mask[i % 4]
    return data.decode(errors='replace')


if __name__ == '__main__':
    run_server()
