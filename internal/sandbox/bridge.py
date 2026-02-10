"""
Monty Bridge — JSON-RPC 2.0 over stdio between Go host and Monty sandbox.

The bridge is the JSON-RPC *server*. Go is the *client*.

Methods (Go → Bridge):
  run(script, external_functions) → starts script execution, returns final output
  shutdown() → clean exit (notification, no response)

During script execution, the bridge becomes a *client* calling back to Go:
  Bridge → Go: {"jsonrpc": "2.0", "method": "<primitive>", "params": {...}, "id": <n>}
  Go → Bridge: {"jsonrpc": "2.0", "result": <value>, "id": <n>}

This allows pipelining: multiple scripts can run concurrently, each making
external function calls with unique IDs. Go responds by ID, not by order.
"""

import json
import sys
import threading
import traceback

from pydantic_monty import Monty, MontyComplete, MontySnapshot


class JsonRpcBridge:
    def __init__(self):
        self._next_id = 0
        self._id_lock = threading.Lock()
        self._write_lock = threading.Lock()
        self._pending = {}  # id → threading.Event + result
        self._pending_lock = threading.Lock()

    def next_id(self) -> int:
        with self._id_lock:
            self._next_id += 1
            return self._next_id

    def send(self, msg: dict) -> None:
        with self._write_lock:
            sys.stdout.write(json.dumps(msg) + "\n")
            sys.stdout.flush()

    def send_result(self, id, result) -> None:
        self.send({"jsonrpc": "2.0", "result": result, "id": id})

    def send_error(self, id, code: int, message: str, data=None) -> None:
        err = {"code": code, "message": message}
        if data is not None:
            err["data"] = data
        self.send({"jsonrpc": "2.0", "error": err, "id": id})

    def call_host(self, method: str, params: dict) -> any:
        """Send a JSON-RPC request to the Go host and wait for response."""
        call_id = self.next_id()
        event = threading.Event()
        with self._pending_lock:
            self._pending[call_id] = {"event": event, "result": None, "error": None}

        self.send({
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": call_id,
        })

        event.wait()

        with self._pending_lock:
            entry = self._pending.pop(call_id)

        if entry["error"] is not None:
            raise Exception(entry["error"]["message"])
        return entry["result"]

    def handle_response(self, msg: dict) -> None:
        """Handle a JSON-RPC response from Go (matched by ID)."""
        call_id = msg["id"]
        with self._pending_lock:
            if call_id not in self._pending:
                return
            entry = self._pending[call_id]
        entry["result"] = msg.get("result")
        entry["error"] = msg.get("error")
        entry["event"].set()

    def handle_run(self, params: dict, request_id) -> None:
        """Execute a script, making external function callbacks to Go."""
        script = params["script"]
        external_functions = params.get("external_functions", [])

        m = Monty(
            code=script,
            external_functions=external_functions,
        )

        progress = m.start()

        while isinstance(progress, MontySnapshot):
            rpc_params = {"args": list(progress.args)}
            if progress.kwargs:
                rpc_params["kwargs"] = dict(progress.kwargs)

            result = self.call_host(progress.function_name, rpc_params)
            progress = progress.resume(return_value=result)

        self.send_result(request_id, convert_output(progress.output))

    def run(self) -> None:
        """Main loop: read JSON-RPC messages, dispatch requests and responses."""
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue

            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                self.send_error(None, -32700, "Parse error")
                continue

            # Is this a response to one of our outgoing calls?
            if "result" in msg or "error" in msg:
                self.handle_response(msg)
                continue

            # It's a request from Go
            method = msg.get("method")
            params = msg.get("params", {})
            request_id = msg.get("id")  # None for notifications

            if method == "shutdown":
                return

            if method == "run":
                # Run in a thread to allow concurrent scripts
                t = threading.Thread(
                    target=self._safe_run,
                    args=(params, request_id),
                    daemon=True,
                )
                t.start()
            else:
                self.send_error(request_id, -32601, f"Method not found: {method}")

    def _safe_run(self, params, request_id):
        try:
            self.handle_run(params, request_id)
        except Exception as e:
            self.send_error(request_id, -32000, str(e), {
                "type": type(e).__name__,
                "traceback": traceback.format_exc(),
            })


def convert_output(value):
    """Convert Monty output to JSON-serializable form."""
    if value is None:
        return None
    if isinstance(value, (str, int, float, bool)):
        return value
    if isinstance(value, (list, tuple)):
        return [convert_output(v) for v in value]
    if isinstance(value, dict):
        return {str(k): convert_output(v) for k, v in value.items()}
    return str(value)


if __name__ == "__main__":
    JsonRpcBridge().run()
