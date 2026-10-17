#!/usr/bin/env python3
"""Validate a Daygo ZIP without extracting it or displaying sample contents.

This checks the Qwen-style reader boundary, not tokenization or model quality.
Usage: python3 scripts/training/validate_dataset.py /path/to/export.zip
"""
import argparse
import base64
import hashlib
import json
from pathlib import PurePosixPath
import zipfile


def validate(path):
    with zipfile.ZipFile(path) as archive:
        infos = archive.infolist()
        names = [entry.filename for entry in infos]
        if len(names) != len(set(names)) or sum(entry.file_size for entry in infos) > 1 << 30:
            raise ValueError("invalid archive size or duplicate members")
        for name in names:
            entry = PurePosixPath(name)
            if entry.is_absolute() or ".." in entry.parts or "\\" in name:
                raise ValueError("invalid archive path")
        records = json.loads(archive.read("train.json"))
        if not isinstance(records, list) or not 1 <= len(records) <= 100:
            raise ValueError("invalid record count")
        ids, groups = set(), set()
        image_count = 0
        for record in records:
            sample_id = record["id"]
            if not isinstance(sample_id, str) or len(sample_id) != 32 or any(c not in "0123456789abcdef" for c in sample_id):
                raise ValueError("invalid sample id")
            if sample_id in ids:
                raise ValueError("duplicate sample")
            ids.add(sample_id)
            sample = json.loads(archive.read(f"samples/{sample_id}.json"))
            if sample["id"] != sample_id or sample["version"] != 1 or sample["verdict"] != "accepted" or sample["privacyApproved"] is not True:
                raise ValueError("sample not approved")
            if record["group_id"] != sample["group"] or not record["group_id"]:
                raise ValueError("missing source group")
            groups.add(record["group_id"])
            conversation = record["conversations"]
            if len(conversation) != 2 or [turn["from"] for turn in conversation] != ["human", "gpt"]:
                raise ValueError("invalid conversation roles")
            if conversation[1]["value"] != sample["label"]:
                raise ValueError("training label mismatch")
            json.loads(sample["label"])
            images = record.get("image", [])
            parts = [part for part in sample["parts"] if part.get("image")]
            if len(images) != len(parts) or conversation[0]["value"].count("<image>") != len(parts):
                raise ValueError("image marker mismatch")
            for name, part in zip(images, parts):
                if name not in names or not name.startswith("images/"):
                    raise ValueError("missing image")
                image = archive.read(name)
                if image != base64.b64decode(part["image"], validate=True) or hashlib.sha256(image).hexdigest() != part["sha256"]:
                    raise ValueError("image digest mismatch")
                image_count += 1
        return {"samples": len(records), "images": image_count, "source_groups": len(groups)}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archive")
    args = parser.parse_args()
    try:
        print(json.dumps(validate(args.archive)))
    except (ValueError, KeyError, TypeError, OSError, zipfile.BadZipFile):
        # No paths, prompt text or candidate values in diagnostic output.
        parser.exit(1, "Dataset validation failed: invalid envelope, approvals or image pairing.\n")
