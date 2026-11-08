#!/usr/bin/env python3
"""Measures Querra's startup time, memory and grid scrolling on macOS.

Drives PerfService's autorun modes (QUERRA_PERF_AUTORUN, see
internal/app/perfservice.go). Build and package the app first:

    wails3 task package
    python3 -I dev/perf/measure_macos.py [bin/querra.app]

Set ONLY to a comma-separated list (startup, idleMemory, grid) to run just
those measurements.
Prints a JSON summary to stdout. The app window opens several times while this
runs; leave it in front so macOS doesn't throttle its animation frames.
"""

from __future__ import annotations

import json
import os
import re
import statistics
import subprocess
import sys
import tempfile
import time
from pathlib import Path

APP = Path(sys.argv[1] if len(sys.argv) > 1 else "bin/querra.app")
BIN = APP / "Contents" / "MacOS" / "querra"
RUNS = int(os.environ.get("RUNS", "5"))
IDLE_SECONDS = 10
SETTLE_SECONDS = 20
PREFIX = "PERF "

# Helper processes WKWebView starts on behalf of an app. They are children of
# launchd, so they are found by diffing the process list around each launch.
WEBKIT = re.compile(r"com\.apple\.WebKit\.(WebContent|Networking|GPU)")
FOOTPRINT = re.compile(r"Footprint:\s+([\d.]+)\s+(B|KB|MB|GB)")
UNITS = {"B": 1 / 2**20, "KB": 1 / 1024, "MB": 1, "GB": 1024}


def webkit_pids() -> set[int]:
    out = subprocess.run(["ps", "-axo", "pid=,command="], capture_output=True, text=True).stdout
    return {int(line.split(None, 1)[0]) for line in out.splitlines() if WEBKIT.search(line)}


def process_name(pid: int) -> str:
    out = subprocess.run(["ps", "-o", "comm=", "-p", str(pid)], capture_output=True, text=True)
    return Path(out.stdout.strip()).name or str(pid)


def footprint_mb(pid: int) -> float:
    """Physical footprint, the number Activity Monitor shows as Memory."""
    out = subprocess.run(["footprint", "-p", str(pid)], capture_output=True, text=True).stdout
    match = FOOTPRINT.search(out)
    return float(match.group(1)) * UNITS[match.group(2)] if match else 0.0


def memory(pids: list[int]) -> dict:
    per_process = {f"{process_name(pid)}[{pid}]": round(footprint_mb(pid), 1) for pid in pids}
    return {"totalMB": round(sum(per_process.values()), 1), "processes": per_process}


def launch(mode: str, report: Path, **env: str) -> subprocess.Popen:
    full_env = {**os.environ, "QUERRA_PERF_AUTORUN": mode, "QUERRA_PERF_REPORT": str(report), **env}
    return subprocess.Popen(
        [str(BIN)], env=full_env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
    )


def events(report: Path) -> list[dict]:
    if not report.exists():
        return []
    lines = report.read_text().splitlines()
    return [json.loads(line[len(PREFIX):]) for line in lines if line.startswith(PREFIX)]


def wait_for(report: Path, name: str, timeout: float) -> dict:
    deadline = time.time() + timeout
    while time.time() < deadline:
        for event in events(report):
            if event["event"] == name:
                return event
        time.sleep(0.05)
    raise TimeoutError(f"no {name!r} event within {timeout:.0f}s")


def measure_startup(work: Path) -> dict:
    """Wall-clock time from exec to the first painted screen."""
    samples = []
    for i in range(RUNS):
        report = work / f"startup-{i}.jsonl"
        launched_ms = time.time() * 1000
        proc = launch("startup", report)
        ready = wait_for(report, "ready", 60)
        proc.wait(timeout=30)
        samples.append(
            {
                "wallMs": round(ready["unixMs"] - launched_ms),
                "sinceGoStartMs": ready["sinceProcessStartMs"],
            }
        )
        time.sleep(1)
    wall = [s["wallMs"] for s in samples]
    return {"runs": samples, "firstMs": wall[0], "medianMs": statistics.median(wall), "maxMs": max(wall)}


def measure_idle(work: Path) -> dict:
    report = work / "idle.jsonl"
    before = webkit_pids()
    proc = launch("idle", report)
    try:
        wait_for(report, "ready", 60)
        time.sleep(IDLE_SECONDS)
        return {"afterSeconds": IDLE_SECONDS, **memory([proc.pid, *sorted(webkit_pids() - before)])}
    finally:
        proc.terminate()
        proc.wait(timeout=30)


def measure_grid(work: Path) -> dict:
    """Scroll benchmark over the 100k × 20 dataset, with memory before the
    benchmark, right after it, and after letting garbage collection settle."""
    report = work / "grid.jsonl"
    before = webkit_pids()
    proc = launch("grid", report, QUERRA_PERF_LINGER_MS=str((SETTLE_SECONDS + 10) * 1000))
    try:
        wait_for(report, "grid-loaded", 120)
        pids = [proc.pid, *sorted(webkit_pids() - before)]
        loaded = memory(pids)
        grid = wait_for(report, "grid", 180)
        after_bench = memory(pids)
        time.sleep(SETTLE_SECONDS)
        settled = memory(pids)
        proc.wait(timeout=60)
    finally:
        if proc.poll() is None:
            proc.terminate()
    finish = next((e for e in events(report) if e["event"] == "finish"), {})
    return {
        "phases": grid["phases"],
        "fetchRows": grid["fetch"],
        "memoryDatasetLoaded": loaded,
        "memoryAfterBenchmark": after_bench,
        f"memoryAfter{SETTLE_SECONDS}sSettle": settled,
        "goMemoryAtFinish": finish.get("mem"),
    }


def sizes() -> dict:
    def mb(path: Path) -> float:
        out = subprocess.run(["du", "-sk", str(path)], capture_output=True, text=True).stdout
        return round(int(out.split()[0]) / 1024, 1)

    result = {"appBundleMB": mb(APP), "binaryMB": mb(BIN)}
    for dmg in sorted(APP.parent.glob("*.dmg")):
        result[f"dmgMB:{dmg.name}"] = mb(dmg)
    return result


def main() -> None:
    if not BIN.exists():
        sys.exit(f"{BIN} not found; run `wails3 task package` first")
    sections = {
        "startup": measure_startup,
        "idleMemory": measure_idle,
        "grid": measure_grid,
    }
    # ONLY=startup,grid runs a subset.
    only = {name for name in os.environ.get("ONLY", "").split(",") if name}
    with tempfile.TemporaryDirectory() as tmp:
        work = Path(tmp)
        summary = {"app": str(APP), "sizes": sizes()}
        for name, measure in sections.items():
            if not only or name in only:
                summary[name] = measure(work)
    json.dump(summary, sys.stdout, indent=2)
    print()


if __name__ == "__main__":
    main()
