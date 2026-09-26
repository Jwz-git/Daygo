#!/usr/bin/env python3
"""Read-only macOS RSS/footprint sampler. No activity, paths, or pixels are collected."""
import argparse
import csv
import math
from datetime import datetime, timezone
from pathlib import Path
import re
import subprocess
import time


def sample(pid, expected_start):
    try:
        if not expected_start:
            return '', '', 'missing'
        rss = subprocess.run(['ps', '-p', str(pid), '-o', 'rss=', '-o', 'lstart='],
                             capture_output=True, text=True, timeout=10)
        if rss.returncode != 0 or not rss.stdout.strip():
            return '', '', 'missing'
        raw_rss, start_id = rss.stdout.strip().split(maxsplit=1)
        if start_id != expected_start:
            return '', '', 'pid_reused'
        rss_bytes = int(raw_rss) * 1024
        vm = subprocess.run(['vmmap', '-summary', str(pid)],
                            capture_output=True, text=True, timeout=30)
        found = re.search(r'^Physical footprint:\s+([\d.]+)([KMGT])', vm.stdout, re.M)
        if not found:
            return rss_bytes, '', 'footprint_unavailable'
        footprint = round(float(found[1]) * 1024 ** ('KMGT'.index(found[2]) + 1))
        return rss_bytes, footprint, 'ok'
    except (subprocess.TimeoutExpired, ValueError, OSError):
        return '', '', 'sample_unavailable'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--pid', action='append', required=True, metavar='LABEL=PID',
                        help='labels: main, web, gpu, network; repeat for multiple web processes')
    parser.add_argument('--duration', type=float, default=86400, help='seconds; 0 samples once')
    parser.add_argument('--interval', type=float, default=300, help='seconds between samples')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if not math.isfinite(args.duration) or not math.isfinite(args.interval) or args.duration < 0 or args.interval <= 0:
        parser.error('duration must be nonnegative and interval positive')
    processes = []
    for entry in args.pid:
        label, separator, raw_pid = entry.partition('=')
        if not separator or label not in ('main', 'web', 'gpu', 'network') or not raw_pid.isdigit() or int(raw_pid) < 1:
            parser.error('each --pid must be main|web|gpu|network=positive PID')
        try:
            started = subprocess.run(['ps', '-p', raw_pid, '-o', 'lstart='],
                                     capture_output=True, text=True, timeout=10).stdout.strip()
        except (subprocess.TimeoutExpired, OSError):
            started = ''
        processes.append((label, int(raw_pid), started))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    # Exclusive creation preserves earlier evidence if a command is rerun.
    with args.output.open('x', newline='') as output:
        writer = csv.writer(output)
        writer.writerow(['utc', 'label', 'pid', 'rss_bytes', 'footprint_bytes', 'status'])
        start = time.monotonic()
        deadline = start + args.duration
        next_sample = start
        while True:
            for label, pid, started in processes:
                stamp = datetime.now(timezone.utc).isoformat()
                writer.writerow([stamp, label, pid, *sample(pid, started)])
            output.flush()
            if time.monotonic() >= deadline:
                break
            next_sample += args.interval
            # Sampling is sequential (vmmap has nonzero cost); avoid catch-up storms.
            next_sample = max(next_sample, time.monotonic())
            time.sleep(max(0, min(next_sample, deadline) - time.monotonic()))


if __name__ == '__main__':
    main()
