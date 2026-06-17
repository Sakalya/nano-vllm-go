#!/usr/bin/env python3
"""Thin HTTP wrapper around nano-vllm. One instance per GPU."""

import argparse
import json
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

sys.path.insert(0, str(__import__("pathlib").Path(__file__).parent.parent / "nano-vllm"))

from nanovllm import LLM
from nanovllm.sampling_params import SamplingParams


_llm: LLM = None
_model_name: str = ""
_lock = threading.Lock()   # one inference at a time per GPU


class Handler(BaseHTTPRequestHandler):

    def do_GET(self):
        if self.path == "/health":
            self._send_json(200, {"status": "ok", "model": _model_name})
        else:
            self._send_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path == "/generate":
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length))
            self._handle_generate(body)
        else:
            self._send_json(404, {"error": "not found"})

    def _handle_generate(self, req: dict):
        prompt = req.get("prompt", "")
        max_tokens = req.get("max_tokens", 256)
        temperature = max(float(req.get("temperature", 1.0)), 1e-9)

        sp = SamplingParams(max_tokens=max_tokens, temperature=temperature)

        with _lock:
            results = _llm.generate([prompt], sp, use_tqdm=False)

        result = results[0]
        prompt_tokens = len(_llm.tokenizer.encode(prompt))
        completion_tokens = len(result["token_ids"])

        self._send_json(200, {
            "text": result["text"],
            "token_ids": result["token_ids"],
            "usage": {
                "prompt_tokens": prompt_tokens,
                "completion_tokens": completion_tokens,
                "total_tokens": prompt_tokens + completion_tokens,
            },
        })

    def _send_json(self, code: int, data: dict):
        body = json.dumps(data).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        print(f"[backend] {self.address_string()} - {fmt % args}", flush=True)


def main():
    parser = argparse.ArgumentParser(description="nano-vllm HTTP backend")
    parser.add_argument("--model", required=True, help="path to model directory")
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", type=int, default=9000)
    parser.add_argument("--tensor-parallel-size", type=int, default=1)
    parser.add_argument("--gpu-memory-utilization", type=float, default=0.9)
    parser.add_argument("--max-model-len", type=int, default=4096)
    args = parser.parse_args()

    global _llm, _model_name
    _model_name = args.model
    print(f"Loading model {args.model} ...", flush=True)
    _llm = LLM(
        args.model,
        tensor_parallel_size=args.tensor_parallel_size,
        gpu_memory_utilization=args.gpu_memory_utilization,
        max_model_len=args.max_model_len,
    )
    print(f"Model loaded. Serving on {args.host}:{args.port}", flush=True)

    server = ThreadingHTTPServer((args.host, args.port), Handler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("Backend shutting down", flush=True)


if __name__ == "__main__":
    main()
