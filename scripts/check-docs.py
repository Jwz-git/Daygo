#!/usr/bin/env python3
"""Checks that the documentation set stays internally consistent.

Two failure modes are cheap to introduce and expensive to notice by reading:

  1. A link points at a file or heading that no longer exists. Section numbers
     move, headings get reworded, and nothing complains until a reader follows
     the link.
  2. A document under docs/ is reachable from nothing. An unlinked decision
     record is, in practice, a decision nobody applies.

Both are mechanical, so they are checked mechanically. Everything else about
documentation accuracy — whether a sentence still matches the code — stays a
human job (AGENTS.md: fix the docs in the same commit as the code).

Run directly, or through scripts/gate.sh. Standard library only.
"""

from __future__ import annotations

import re
import sys
import unicodedata
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SKIP_DIRS = {".git", "node_modules", "build", "dist", "legacy", ".playwright-mcp"}

# Markdown inline links: [text](target). Reference-style links are not used in
# this repository, so this one pattern is enough.
LINK = re.compile(r"\[[^\]]*\]\(([^)\s]+)\)")


def markdown_files() -> list[Path]:
    files = []
    for path in ROOT.rglob("*.md"):
        if any(part in SKIP_DIRS for part in path.relative_to(ROOT).parts):
            continue
        files.append(path)
    return sorted(files)


def slug(title: str) -> str:
    """Reproduce GitHub's heading anchor: lowercase, drop punctuation, spaces to hyphens."""
    text = title.strip().replace("`", "")
    kept = [
        "-" if character in " \t" else character
        for character in text.lower()
        if character in "-_ \t" or not unicodedata.category(character).startswith("P")
    ]
    return "".join(kept)


def anchors(path: Path) -> set[str]:
    found: set[str] = set()
    seen: dict[str, int] = {}
    fenced = False
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if fenced or not line.startswith("#"):
            continue
        base = slug(line.lstrip("#"))
        count = seen.get(base, 0)
        seen[base] = count + 1
        found.add(base if count == 0 else f"{base}-{count}")
    return found


def main() -> int:
    files = markdown_files()
    anchor_cache: dict[Path, set[str]] = {}
    problems: list[str] = []
    linked: set[Path] = set()

    for source in files:
        for target in LINK.findall(source.read_text(encoding="utf-8")):
            if target.startswith(("http://", "https://", "mailto:", "#")) and not target.startswith("#"):
                continue
            path_part, _, anchor = target.partition("#")
            if path_part == "":
                destination = source
            else:
                destination = (source.parent / path_part).resolve()
                if not destination.exists():
                    problems.append(f"{source.relative_to(ROOT)}: 链接指向不存在的路径 -> {target}")
                    continue
                linked.add(destination)
            if anchor and destination.suffix == ".md":
                if destination not in anchor_cache:
                    anchor_cache[destination] = anchors(destination)
                if anchor not in anchor_cache[destination]:
                    problems.append(f"{source.relative_to(ROOT)}: 链接指向不存在的小节 -> {target}")

    # Entry points are reachable by definition; everything else under docs/ has
    # to be linked from somewhere, or it is documentation nobody will find.
    entry_points = {ROOT / "README.md", ROOT / "README.en.md", ROOT / "AGENTS.md", ROOT / "docs" / "README.md"}
    for path in files:
        if path in entry_points or path in linked:
            continue
        if ROOT / "docs" not in path.parents and path.parent != ROOT / "docs":
            continue
        problems.append(f"{path.relative_to(ROOT)}: 没有任何文档链接到它")

    for problem in problems:
        print(f"check-docs: {problem}", file=sys.stderr)
    print(f"check-docs: 检查 {len(files)} 个 markdown 文件，{len(problems)} 处问题")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
