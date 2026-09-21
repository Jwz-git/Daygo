#!/usr/bin/env python3
"""Create the shared Sparkle/WinSparkle appcast from already signed assets."""

from __future__ import annotations

import argparse
import datetime as dt
import html
from pathlib import Path


def enclosure(url: str, path: Path, signature: str, os_name: str) -> str:
    attrs = {
        "url": url,
        "length": str(path.stat().st_size),
        "type": "application/octet-stream",
        "sparkle:edSignature": signature,
        "sparkle:os": os_name,
    }
    rendered = " ".join(f'{key}="{html.escape(value, quote=True)}"' for key, value in attrs.items())
    return f"      <enclosure {rendered} />"


def item(version: str, published: str, enclosure_xml: str) -> list[str]:
    escaped = html.escape(version)
    return [
        f"    <item><title>Daygo {escaped}</title>",
        f"      <pubDate>{published}</pubDate>",
        f"      <sparkle:version>{escaped}</sparkle:version>",
        f"      <sparkle:shortVersionString>{escaped}</sparkle:shortVersionString>",
        f"      <link>https://github.com/Jwz-git/Daygo/releases/tag/v{escaped}</link>",
        enclosure_xml,
        "    </item>",
    ]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--mac", type=Path, required=True)
    parser.add_argument("--mac-signature", required=True)
    parser.add_argument("--windows", type=Path, required=True)
    parser.add_argument("--windows-signature", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    base = f"https://github.com/Jwz-git/Daygo/releases/download/v{args.version}"
    published = dt.datetime.now(dt.timezone.utc).strftime("%a, %d %b %Y %H:%M:%S %z")
    rows = [
        '<?xml version="1.0" encoding="utf-8"?>',
        '<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">',
        "  <channel>",
        "    <title>Daygo updates</title>",
    ]
    rows += item(args.version, published, enclosure(f"{base}/{args.mac.name}", args.mac, args.mac_signature, "macos"))
    rows += item(args.version, published, enclosure(f"{base}/{args.windows.name}", args.windows, args.windows_signature, "windows"))
    rows += ["  </channel>", "</rss>", ""]
    args.output.write_text("\n".join(rows), encoding="utf-8")


if __name__ == "__main__":
    main()
