#!/usr/bin/env python3
"""Test client that simulates Suricata sending HTTP events via Unix socket."""

import socket
import json
import time
import sys

SOCKET_PATH = "/home/Serebr1k/flagmate/sockets/eve.sock"

def send_test_event():
    event = {
        "timestamp": "2026-06-04T12:38:00.000000+0000",
        "event_type": "http",
        "src_ip": "172.28.0.5",
        "src_port": 8080,
        "dest_ip": "172.28.0.2",
        "dest_port": 18080,
        "proto": "TCP",
        "flow_id": 12345,
        "http": {
            "hostname": "localhost",
            "url": "/api/test",
            "http_user_agent": "TestAgent/1.0",
            "http_content_type": "application/json",
            "http_method": "GET",
            "http_content_length": 0,
            "status": 200,
            "http_referer": "",
            "protocol": "HTTP/1.1",
            "length": 123
        }
    }
    
    try:
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(SOCKET_PATH)
        
        # Send the event as JSON line
        data = json.dumps(event) + "\n"
        sock.sendall(data.encode())
        print(f"Sent event: {json.dumps(event, indent=2)}")
        
        sock.close()
        return True
    except Exception as e:
        print(f"Error: {e}")
        return False

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--loop":
        while True:
            send_test_event()
            time.sleep(1)
    else:
        success = send_test_event()
        sys.exit(0 if success else 1)
