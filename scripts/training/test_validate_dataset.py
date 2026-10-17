import base64
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

from validate_dataset import validate


class DatasetFixture(unittest.TestCase):
    def write(self, directory, approved=True, mismatch=False, traversal=False):
        sample_id = "0123456789abcdef0123456789abcdef"
        data = b"anonymous image fixture"
        sample = {"version": 1, "id": sample_id, "verdict": "accepted", "privacyApproved": approved,
                  "group": "anonymous-group", "label": "{}", "parts": [{"image": base64.b64encode(data).decode(),
                  "sha256": hashlib.sha256(data).hexdigest()}]}
        image_path = f"images/{sample_id}-0.png"
        record = {"id": sample_id, "group_id": sample["group"], "image": [image_path],
                  "conversations": [{"from": "human", "value": "<image>\nAnonymous fixture"}, {"from": "gpt", "value": "{}"}]}
        path = Path(directory) / "fixture.zip"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("train.json", json.dumps([record]))
            archive.writestr(f"samples/{sample_id}.json", json.dumps(sample))
            archive.writestr(image_path, b"different" if mismatch else data)
            if traversal:
                archive.writestr("../outside", "anonymous")
        return path

    def test_validated_pair(self):
        with tempfile.TemporaryDirectory() as directory:
            self.assertEqual(validate(self.write(directory)), {"samples": 1, "images": 1, "source_groups": 1})

    def test_rejects_unapproved_mismatched_and_escaping_data(self):
        for options in ({"approved": False}, {"mismatch": True}, {"traversal": True}):
            with self.subTest(options=options), tempfile.TemporaryDirectory() as directory:
                with self.assertRaises(ValueError):
                    validate(self.write(directory, **options))


if __name__ == "__main__":
    unittest.main()
