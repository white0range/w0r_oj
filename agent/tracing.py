"""Print Agent execution events to stdout for Docker to collect."""

from contextlib import contextmanager
from contextvars import ContextVar
import hashlib
import os
import re
import time

from langchain_core.callbacks import BaseCallbackHandler


# Disable automatic cloud exports, including stale deployment settings.
def _disable_cloud_tracing():
    for key in ("LANGSMITH_TRACING", "LANGCHAIN_TRACING_V2", "LANGCHAIN_TRACING"):
        os.environ[key] = "false"


_disable_cloud_tracing()
_current = ContextVar("agent_task", default=None)


def _log(stage, context=None, **fields):
    context = context or _current.get()
    if context is None:
        return
    turn_id, attempt_id = context
    values = {"turn_id": turn_id, "stage": stage, **fields}
    if attempt_id:
        values["attempt_id"] = attempt_id
    line = "[agent] " + " ".join(f"{key}={re.sub(r'[^a-zA-Z0-9_.:-]', '_', str(value))[:100]}" for key, value in values.items())
    try:
        print(line, flush=True)
    except (OSError, ValueError):
        pass


trace_event = _log


@contextmanager
def trace_task(turn_id, stage, processing_token=""):
    _disable_cloud_tracing()
    attempt_id = hashlib.sha256(processing_token.encode()).hexdigest()[:24] if processing_token else ""
    token = _current.set((turn_id, attempt_id))
    try:
        with trace_step(stage):
            yield
    finally:
        _current.reset(token)


@contextmanager
def trace_step(name):
    started = time.monotonic()
    _log(f"{name}_start")
    try:
        yield
    except BaseException as exc:
        _log(f"{name}_failed", error_type=type(exc).__name__, duration_ms=int((time.monotonic() - started) * 1000))
        raise
    else:
        _log(f"{name}_done", duration_ms=int((time.monotonic() - started) * 1000))


class ConsoleTraceCallback(BaseCallbackHandler):
    def __init__(self, context):
        self.context = context
        self.runs = {}

    def _start(self, kind, serialized, run_id):
        name = (serialized or {}).get("name", kind)
        self.runs[run_id] = (name, time.monotonic())
        _log(f"{kind}_start", self.context, name=name, run_id=run_id)

    def _end(self, kind, run_id, error=None, **fields):
        name, started = self.runs.pop(run_id, (kind, time.monotonic()))
        if error is not None:
            fields["error_type"] = type(error).__name__
        _log(f"{kind}_{'failed' if error is not None else 'done'}", self.context, name=name,
             run_id=run_id, duration_ms=int((time.monotonic() - started) * 1000), **fields)

    def on_chat_model_start(self, serialized, messages, *, run_id, **kwargs):
        self._start("model", serialized, run_id)

    on_llm_start = on_chat_model_start

    def on_llm_end(self, response, *, run_id, **kwargs):
        usage = (response.llm_output or {}).get("token_usage") or {}
        if not usage:
            for group in response.generations:
                for generation in group:
                    for key, value in (getattr(getattr(generation, "message", None), "usage_metadata", None) or {}).items():
                        if isinstance(value, int):
                            usage[key] = usage.get(key, 0) + value
        fields = {}
        for target, source in (("input_tokens", "prompt_tokens"), ("output_tokens", "completion_tokens"), ("total_tokens", "total_tokens")):
            value = usage.get(source, usage.get(target))
            if isinstance(value, int):
                fields[target] = value
        self._end("model", run_id, **fields)

    def on_llm_error(self, error, *, run_id, **kwargs):
        self._end("model", run_id, error=error)

    def on_tool_start(self, serialized, input_str, *, run_id, **kwargs):
        self._start("tool", serialized, run_id)

    def on_tool_end(self, output, *, run_id, **kwargs):
        self._end("tool", run_id)

    def on_tool_error(self, error, *, run_id, **kwargs):
        self._end("tool", run_id, error=error)


def console_callbacks():
    context = _current.get()
    return [ConsoleTraceCallback(context)] if context is not None else []
