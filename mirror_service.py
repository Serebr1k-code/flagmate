#!/usr/bin/env python3
import socketserver


FLAG = b"flag{mirror_service_flag_18081}"


class MirrorHandler(socketserver.BaseRequestHandler):
    def handle(self):
        self.request.settimeout(1.0)
        chunks = []
        try:
            while True:
                data = self.request.recv(4096)
                if not data:
                    break
                chunks.append(data)
                if b"\n" in data:
                    break
        except Exception:
            pass
        payload = b"".join(chunks)
        response = b"mirror ok\nflag: " + FLAG + b"\nbytes: " + str(len(payload)).encode() + b"\n"
        self.request.sendall(response)


if __name__ == "__main__":
    with socketserver.ThreadingTCPServer(("0.0.0.0", 18081), MirrorHandler) as server:
        server.allow_reuse_address = True
        print("Mirror test service listening on 0.0.0.0:18081", flush=True)
        server.serve_forever()
