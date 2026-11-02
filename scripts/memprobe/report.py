#!/usr/bin/env python3
import csv
import re
import sys
from collections import OrderedDict, defaultdict

UNITS = {"K": 1 / 1024, "M": 1, "G": 1024}


def footprint_mb(text):
    m = re.fullmatch(r"([\d.]+)([KMG])", text or "")
    return float(m.group(1)) * UNITS[m.group(2)] if m else None


def group(name):
    if name == "PlainShelf":
        return "PlainShelf (Go + WKWebView host)"
    return name.removeprefix("com.apple.WebKit.")


def load_samples(path):
    rows = []
    with open(path, newline="") as f:
        for row in csv.DictReader(f):
            rows.append(row)
    return rows


def load_marks(path):
    marks = []
    with open(path) as f:
        for line in f:
            ts, phase, heap, idle, sys_bytes = line.rstrip("\n").split("\t")
            marks.append((int(ts), phase, int(heap), int(idle), int(sys_bytes)))
    return marks


def phase_end_rss(rows):
    by_phase = OrderedDict()
    for row in rows:
        ts = int(row["ts"])
        key = (row["phase"], group(row["process"]))
        sums = by_phase.setdefault(row["phase"], defaultdict(dict))
        sums[ts][key[1]] = sums[ts].get(key[1], 0) + int(row["rss_kb"])
    result = OrderedDict()
    for phase, per_ts in by_phase.items():
        last_ts = max(per_ts)
        result[phase] = {name: kb / 1024 for name, kb in per_ts[last_ts].items()}
    return result


def peaks(rows):
    rss = defaultdict(float)
    fp = defaultdict(float)
    per_ts = defaultdict(lambda: defaultdict(float))
    for row in rows:
        name = group(row["process"])
        per_ts[row["ts"]][name] += int(row["rss_kb"]) / 1024
        value = footprint_mb(row["footprint"])
        if value is not None:
            fp[name] = max(fp[name], value)
    for sample in per_ts.values():
        for name, mb in sample.items():
            rss[name] = max(rss[name], mb)
    return rss, fp


def main():
    samples_path, marks_path = sys.argv[1], sys.argv[2]
    rows = load_samples(samples_path)
    marks = load_marks(marks_path)
    ends = phase_end_rss(rows)
    names = sorted({name for phase in ends.values() for name in phase})
    rss_peak, fp_peak = peaks(rows)

    out = ["## Desktop memory probe", ""]
    out.append("RSS at the end of each phase (MB, summed per process kind):")
    out.append("")
    out.append("| Phase | " + " | ".join(names) + " | Total |")
    out.append("|---|" + "---:|" * (len(names) + 1))
    for phase, values in ends.items():
        cells = [f"{values.get(n, 0):.0f}" for n in names]
        out.append(f"| {phase} | " + " | ".join(cells) + f" | {sum(values.values()):.0f} |")

    out += ["", "Peaks (MB):", "", "| Process kind | Peak RSS | Peak physical footprint |", "|---|---:|---:|"]
    for n in names:
        fp = f"{fp_peak[n]:.0f}" if n in fp_peak else "n/a"
        out.append(f"| {n} | {rss_peak[n]:.0f} | {fp} |")

    out += ["", "Go runtime at each mark (MB):", "", "| Phase | HeapInuse | Heap idle, not released | Sys |", "|---|---:|---:|---:|"]
    for _, phase, heap, idle, sys_bytes in marks:
        out.append(f"| {phase} | {heap / 2**20:.0f} | {idle / 2**20:.0f} | {sys_bytes / 2**20:.0f} |")

    settles = [p for p in ends if p.endswith(" settle")]
    if len(settles) >= 2:
        first, last = ends[settles[0]], ends[settles[-1]]
        out += ["", f"Growth from `{settles[0]}` to `{settles[-1]}` across {len(settles) - 1} more cycles:", ""]
        for n in names:
            delta = last.get(n, 0) - first.get(n, 0)
            out.append(f"- {n}: {delta:+.0f} MB")

    print("\n".join(out))


if __name__ == "__main__":
    main()
