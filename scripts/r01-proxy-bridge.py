# pattern: Imperative Shell
"""Temporary loopback CONNECT bridge over wsl.exe pipes, without TLS inspection.

Run with Windows Python. It reuses the existing Windows 127.0.0.1:7890 proxy,
does not listen on a Windows network interface, and changes no system settings.
WSL interop is not required. Only the approved provider hosts can be tunneled.
"""
import argparse
import base64
import json
import os
import socket
import socketserver
import subprocess
import sys
import threading
from pathlib import Path

ALLOWED = {b"chatgpt.com:443", b"api.openai.com:443", b"auth.openai.com:443"}
WRITE_LOCK = threading.Lock()
CONNECTIONS = {}
MAP_LOCK = threading.Lock()

def valid_header(data):
    fields = data.split(b"\r\n", 1)[0].split()
    return len(data) <= 16384 and len(fields) == 3 and fields[0] == b"CONNECT" and fields[1] in ALLOWED

def emit(stream, kind, identity, data=b""):
    raw = json.dumps({"kind": kind, "id": identity, "data": base64.b64encode(data).decode()}).encode()+b"\n"
    with WRITE_LOCK:
        stream.write(raw); stream.flush()

def close_connection(identity):
    with MAP_LOCK:
        sock = CONNECTIONS.pop(identity, None)
    if sock:
        try: sock.shutdown(socket.SHUT_RDWR)
        except OSError: pass
        sock.close()

def linux_server():
    class Handler(socketserver.BaseRequestHandler):
        def handle(self):
            identity = str(id(self))
            try:
                self.request.settimeout(15)
                data = bytearray()
                while not data.endswith(b"\r\n\r\n"):
                    chunk = self.request.recv(1)
                    if not chunk or len(data) >= 16384: return
                    data.extend(chunk)
                if not valid_header(bytes(data)): return
                with MAP_LOCK:
                    if len(CONNECTIONS) >= 16: return
                    CONNECTIONS[identity] = self.request
                emit(sys.stdout.buffer, "open", identity, bytes(data))
                self.request.settimeout(90)
                while chunk := self.request.recv(32768):
                    emit(sys.stdout.buffer, "data", identity, chunk)
            except OSError:
                pass
            finally:
                close_connection(identity)
                emit(sys.stdout.buffer, "close", identity)
    class Server(socketserver.ThreadingTCPServer):
        daemon_threads = True
        allow_reuse_address = False
    with Server(("127.0.0.1", 0), Handler) as server:
        emit(sys.stdout.buffer, "ready", str(server.server_address[1]))
        threading.Thread(target=server.serve_forever, daemon=True).start()
        for line in sys.stdin.buffer:
            message = json.loads(line)
            if message["kind"] == "close":
                close_connection(message["id"]); continue
            with MAP_LOCK: sock = CONNECTIONS.get(message["id"])
            if sock:
                try: sock.sendall(base64.b64decode(message["data"]))
                except OSError: close_connection(message["id"])
        server.shutdown()

def windows_parent():
    child = subprocess.Popen(["wsl", "-d", "Ubuntu-22.04", "--", "python3",
        "/mnt/d/Programs/Polis/scripts/r01-proxy-bridge.py", "--linux"],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    def download(identity, sock):
        try:
            while data := sock.recv(32768): emit(child.stdin, "data", identity, data)
        except OSError:
            pass
        finally:
            close_connection(identity)
            try: emit(child.stdin, "close", identity)
            except (OSError, ValueError): pass
    try:
        for line in child.stdout:
            message = json.loads(line); identity = message["id"]
            if message["kind"] == "ready":
                (Path(__file__).resolve().parent.parent/".runtime/r01-proxy-port.txt").write_text(identity)
                print("temporary provider bridge: 127.0.0.1:"+identity, flush=True)
            elif message["kind"] == "open":
                data = base64.b64decode(message["data"])
                if not valid_header(data): emit(child.stdin,"close",identity); continue
                try:
                    sock = socket.create_connection(("127.0.0.1",7890),timeout=15)
                    sock.settimeout(90); sock.sendall(data)
                    with MAP_LOCK: CONNECTIONS[identity]=sock
                    threading.Thread(target=download,args=(identity,sock),daemon=True).start()
                except OSError: emit(child.stdin,"close",identity)
            elif message["kind"] == "data":
                with MAP_LOCK: sock=CONNECTIONS.get(identity)
                if sock:
                    try: sock.sendall(base64.b64decode(message["data"]))
                    except OSError: close_connection(identity)
            elif message["kind"] == "close": close_connection(identity)
    finally:
        child.stdin.close()
        try: child.wait(timeout=3)
        except subprocess.TimeoutExpired: child.kill(); child.wait()

if __name__ == "__main__":
    parser=argparse.ArgumentParser();parser.add_argument("--linux",action="store_true")
    if parser.parse_args().linux: linux_server()
    else: windows_parent()
