#!/usr/bin/env python3
import json, os, threading, time, sqlite3, random, string, urllib.request, urllib.parse
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

FLAG_PREFIX = os.environ.get("FLAG_PREFIX", "flag{demo_")
PORT = int(os.environ.get("CHALL_PORT", "18082"))
db = None
current_flag = ""
checker_user = "checker_bot_" + ''.join(random.choices(string.ascii_letters, k=6))

def get_db():
    global db
    if db is None:
        db = sqlite3.connect("/tmp/chall.db", check_same_thread=False)
        db.execute("CREATE TABLE IF NOT EXISTS users (username TEXT PRIMARY KEY, password TEXT, role TEXT)")
        db.execute("CREATE TABLE IF NOT EXISTS secrets (key TEXT PRIMARY KEY, value TEXT)")
        db.commit()
    return db

def make_flag():
    return FLAG_PREFIX + str(PORT) + "_" + ''.join(random.choices(string.ascii_lowercase+string.digits, k=12)) + "}"

class ChallHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path.rstrip('/')
        qs = parse_qs(parsed.query, keep_blank_values=True)
        conn = get_db()

        if path == '/api/register':
            username = qs.get('username', [''])[0]
            password = qs.get('password', [''])[0]
            if not username:
                return self._json(400, {"error": "missing username"})
            try:
                conn.execute("INSERT INTO users(username, password, role) VALUES ('{}', '{}', 'user')".format(username, password))
                conn.commit()
            except Exception:
                return self._json(400, {"error": "exists"})
            return self._json(200, {"status": "registered", "user": username})

        elif path == '/api/login':
            username = qs.get('username', [''])[0]
            password = qs.get('password', [''])[0]
            cur = conn.execute("SELECT role FROM users WHERE username = '{}' AND password = '{}'".format(username, password))
            row = cur.fetchone()
            if row:
                return self._json(200, {"status": "ok", "role": row[0]})
            return self._json(403, {"error": "bad credentials"})

        elif path == '/api/profile':
            username = qs.get('user', [''])[0]
            if not username:
                return self._json(400, {"error": "no user"})
            safe = username.replace('../', '').replace('..\\', '')
            return self._json(200, {"user": username, "profile_path": "/profiles/{}.txt".format(safe), "data": "Profile: " + safe})

        elif path == '/api/flag':
            global current_flag
            username = qs.get('user', [''])[0]
            password = qs.get('password', [''])[0]
            cur = conn.execute("SELECT role FROM users WHERE username = '{}' AND password = '{}'".format(username, password))
            row = cur.fetchone()
            flag = current_flag if current_flag else "no flag yet"
            if row:
                return self._json(200, {"flag": flag, "role": row[0]})
            return self._json(403, {"error": "auth failed"})

        elif path == '/api/health':
            return self._json(200, {"status": "ok"})
        else:
            return self._json(404, {"error": "not found"})

    def _json(self, status, data):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())
    def log_message(self, fmt, *args): pass

def http_get(path):
    try:
        r = urllib.request.urlopen("http://127.0.0.1:{}{}".format(PORT, path), timeout=5)
        return r.read().decode()
    except Exception as e:
        return None

def checker_loop():
    global current_flag
    # Register checker user once
    http_get("/api/register?username={}&password=checker_pass".format(checker_user))
    while True:
        try:
            flag = make_flag()
            current_flag = flag
            # Store flag in DB
            conn = get_db()
            conn.execute("INSERT OR REPLACE INTO secrets(key, value) VALUES ('flag', ?)", (flag,))
            conn.commit()
            print("[checker:{}] flag stored: {}".format(PORT, flag), flush=True)

            time.sleep(60)

            # Verify flag via HTTP
            resp = http_get("/api/flag?user={}&password=checker_pass".format(checker_user))
            if resp and flag in resp:
                print("[checker:{}] flag OK via HTTP".format(PORT), flush=True)
            else:
                print("[checker:{}] flag GONE! got: {}".format(PORT, resp), flush=True)

            conn.execute("DELETE FROM secrets WHERE key = 'flag'")
            conn.commit()
            current_flag = ""
            print("[checker:{}] flag removed".format(PORT), flush=True)
        except Exception as e:
            print("[checker:{}] error: {}".format(PORT, e), flush=True)

if __name__ == '__main__':
    server = HTTPServer(('0.0.0.0', PORT), ChallHandler)
    print("Vuln challenge starting on :{}".format(PORT), flush=True)
    threading.Thread(target=checker_loop, daemon=True).start()
    server.serve_forever()
