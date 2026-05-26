#!/usr/bin/env python3
"""Run benchmark comparison for pull requests and update a PR comment."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import statistics
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path


COMMENT_MARKER = "<!-- go-toml-benchmark-comparison -->"
BENCH_RE = re.compile(
    r"^(Benchmark\S+)-\d+\s+\d+\s+"
    r"(?P<ns>[\d.]+)\s+ns/op\s+"
    r"(?P<bytes>[\d.]+)\s+B/op\s+"
    r"(?P<allocs>[\d.]+)\s+allocs/op"
)


def main() -> int:
    args = parse_args()
    repo_root = Path(args.repo_root).resolve()

    base_sha, head_sha, pr_number = resolve_context(args)
    if not base_sha or not head_sha:
        print("benchmark comparison is only available for pull_request events", file=sys.stderr)
        return 0

    with tempfile.TemporaryDirectory(prefix="go-toml-bench-") as tmp:
        tmpdir = Path(tmp)
        base_dir = tmpdir / "base"
        head_dir = tmpdir / "head"

        git(repo_root, "worktree", "add", "--detach", str(base_dir), base_sha)
        git(repo_root, "worktree", "add", "--detach", str(head_dir), head_sha)
        try:
            ensure_benchmark_harness(base_dir, head_dir)
            base_output = run_benchmarks(base_dir, args.count)
            head_output = run_benchmarks(head_dir, args.count)
        finally:
            git(repo_root, "worktree", "remove", "--force", str(base_dir), check=False)
            git(repo_root, "worktree", "remove", "--force", str(head_dir), check=False)

    base = parse_benchmarks(base_output)
    head = parse_benchmarks(head_output)
    body = render_comment(base, head, base_sha, head_sha, args.count)

    print(body)
    if args.no_comment:
        return 0
    if not pr_number:
        print("PR number is unavailable; skipping comment update", file=sys.stderr)
        return 0
    try:
        update_pr_comment(pr_number, body)
    except RuntimeError as err:
        print(f"failed to update PR comment: {err}", file=sys.stderr)
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=".")
    parser.add_argument("--base")
    parser.add_argument("--head")
    parser.add_argument("--pr-number")
    parser.add_argument("--count", type=int, default=int(os.getenv("BENCH_COUNT", "5")))
    parser.add_argument("--no-comment", action="store_true")
    return parser.parse_args()


def resolve_context(args: argparse.Namespace) -> tuple[str | None, str | None, int | None]:
    if args.base and args.head:
        pr_number = int(args.pr_number) if args.pr_number else None
        return args.base, args.head, pr_number

    event_path = os.getenv("GITHUB_EVENT_PATH")
    if not event_path:
        return None, None, None

    with open(event_path, encoding="utf-8") as f:
        event = json.load(f)
    pull = event.get("pull_request")
    if not pull:
        return None, None, None
    return pull["base"]["sha"], pull["head"]["sha"], int(pull["number"])


def ensure_benchmark_harness(base_dir: Path, head_dir: Path) -> None:
    base_benchmarks = base_dir / "benchmarks"
    if base_benchmarks.exists():
        return
    shutil.copytree(head_dir / "benchmarks", base_benchmarks)


def run_benchmarks(worktree: Path, count: int) -> str:
    benchmarks = worktree / "benchmarks"
    if not benchmarks.exists():
        raise RuntimeError(f"{benchmarks} does not exist")
    proc = subprocess.run(
        ["go", "test", "-run", "^$", "-bench", ".", "-benchmem", "-count", str(count)],
        cwd=benchmarks,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    if proc.returncode != 0:
        raise RuntimeError(proc.stdout)
    return proc.stdout


def parse_benchmarks(output: str) -> dict[str, dict[str, float]]:
    values: dict[str, dict[str, list[float]]] = {}
    for line in output.splitlines():
        match = BENCH_RE.match(line)
        if not match:
            continue
        name = line.split()[0].rsplit("-", 1)[0]
        values.setdefault(name, {"ns": [], "bytes": [], "allocs": []})
        values[name]["ns"].append(float(match.group("ns")))
        values[name]["bytes"].append(float(match.group("bytes")))
        values[name]["allocs"].append(float(match.group("allocs")))

    return {
        name: {metric: statistics.median(samples) for metric, samples in metrics.items()}
        for name, metrics in values.items()
    }


def render_comment(
    base: dict[str, dict[str, float]],
    head: dict[str, dict[str, float]],
    base_sha: str,
    head_sha: str,
    count: int,
) -> str:
    names = sorted(set(base) & set(head))
    if not names:
        raise RuntimeError("no matching benchmark results found")
    lines = [
        COMMENT_MARKER,
        "## Benchmark comparison",
        "",
        f"Base `{base_sha[:12]}` vs head `{head_sha[:12]}`. Median of `{count}` runs.",
        "",
        "| Benchmark | ns/op | Δ | B/op | Δ | allocs/op | Δ |",
        "| --- | ---: | ---: | ---: | ---: | ---: | ---: |",
    ]
    for name in names:
        base_metrics = base[name]
        head_metrics = head[name]
        lines.append(
            "| {name} | {ns} | {ns_delta} | {bytes_} | {bytes_delta} | {allocs} | {allocs_delta} |".format(
                name=name,
                ns=format_pair(base_metrics["ns"], head_metrics["ns"]),
                ns_delta=format_delta(base_metrics["ns"], head_metrics["ns"]),
                bytes_=format_pair(base_metrics["bytes"], head_metrics["bytes"]),
                bytes_delta=format_delta(base_metrics["bytes"], head_metrics["bytes"]),
                allocs=format_pair(base_metrics["allocs"], head_metrics["allocs"]),
                allocs_delta=format_delta(base_metrics["allocs"], head_metrics["allocs"]),
            )
        )
    lines.extend(
        [
            "",
            "Negative deltas mean the PR is faster or allocates less.",
        ]
    )
    return "\n".join(lines)


def format_pair(base: float, head: float) -> str:
    return f"{base:.0f} → {head:.0f}"


def format_delta(base: float, head: float) -> str:
    if base == 0:
        return "n/a"
    return f"{((head - base) / base) * 100:+.2f}%"


def update_pr_comment(pr_number: int, body: str) -> None:
    repo = os.environ["GITHUB_REPOSITORY"]
    token = os.environ["GITHUB_TOKEN"]
    base_url = add_query(
        f"https://api.github.com/repos/{repo}/issues/{pr_number}/comments",
        {"per_page": "100"},
    )
    headers = {
        "Accept": "application/vnd.github+json",
        "Authorization": f"Bearer {token}",
        "X-GitHub-Api-Version": "2022-11-28",
    }

    comments = request_json_pages(base_url, headers=headers)
    for comment in comments:
        if COMMENT_MARKER in comment.get("body", ""):
            request_json(comment["url"], method="PATCH", headers=headers, payload={"body": body})
            return
    request_json(base_url, method="POST", headers=headers, payload={"body": body})


def request_json_pages(url: str, *, headers: dict[str, str]) -> list[object]:
    items: list[object] = []
    next_url: str | None = url
    while next_url:
        page, next_url = request_json_page(next_url, headers=headers)
        if not isinstance(page, list):
            raise RuntimeError("GitHub API returned a non-list comments response")
        items.extend(page)
    return items


def request_json_page(url: str, *, headers: dict[str, str]) -> tuple[object, str | None]:
    req = urllib.request.Request(url, headers=headers, method="GET")
    try:
        with urllib.request.urlopen(req) as resp:
            return json.load(resp), next_link(resp.headers.get("Link", ""))
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"GitHub API request failed: {err.code} {detail}") from err


def request_json(
    url: str,
    *,
    method: str = "GET",
    headers: dict[str, str],
    payload: dict[str, str] | None = None,
) -> object:
    data = None
    if payload is not None:
        data = json.dumps(payload).encode()
        headers = {**headers, "Content-Type": "application/json"}
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            return json.load(resp)
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"GitHub API request failed: {err.code} {detail}") from err


def next_link(link_header: str) -> str | None:
    for part in link_header.split(","):
        url_part, _, rel_part = part.partition(";")
        if 'rel="next"' not in rel_part:
            continue
        url_part = url_part.strip()
        if url_part.startswith("<") and url_part.endswith(">"):
            return url_part[1:-1]
    return None


def add_query(url: str, params: dict[str, str]) -> str:
    parsed = urllib.parse.urlparse(url)
    query = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
    query.extend(params.items())
    return urllib.parse.urlunparse(parsed._replace(query=urllib.parse.urlencode(query)))


def git(repo_root: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=repo_root,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=check,
    )


if __name__ == "__main__":
    raise SystemExit(main())
