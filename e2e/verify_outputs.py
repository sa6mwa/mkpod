#!/usr/bin/env python3
import json
import sys
import xml.etree.ElementTree as ET
from pathlib import Path


ITUNES = "{http://www.itunes.com/dtds/podcast-1.0.dtd}"


def fail(message):
    raise SystemExit(f"FAIL: {message}")


def main():
    if len(sys.argv) != 3:
        fail("usage: verify_outputs.py output-bucket.json uploaded-podcast.rss")

    objects = json.loads(Path(sys.argv[1]).read_text())
    keys = {obj["Key"] for obj in objects.get("Contents", [])}
    if "podcast.rss" not in keys:
        fail("podcast.rss missing from output bucket")
    if not any(key.endswith(".m4a") for key in keys):
        fail("no .m4a objects found in output bucket")
    if "artwork/episode-1.jpg" not in keys:
        fail("artwork/episode-1.jpg missing from output bucket")
    if "artwork/podcast-cover.jpg" not in keys:
        fail("artwork/podcast-cover.jpg missing from output bucket")

    root = ET.fromstring(Path(sys.argv[2]).read_text())
    channel = root.find("channel")
    if channel is None:
        fail("uploaded RSS channel element missing")
    items = channel.findall("item")
    if len(items) < 2:
        fail(f"expected at least 2 RSS items, found {len(items)}")

    titles = [items[0].findtext("title", default=""), items[1].findtext("title", default="")]
    if titles != ["Acceptance Episode One", "Acceptance Episode Two"]:
        fail(f"unexpected uploaded RSS item order {titles!r}")

    for idx, item in enumerate(items[:2], start=1):
        description = item.findtext("description", default="").strip()
        if not description:
            fail(f"uploaded RSS item {idx} missing description")

        enclosure = item.find("enclosure")
        if enclosure is None:
            fail(f"uploaded RSS item {idx} missing enclosure")
        if enclosure.attrib.get("type", "") != "audio/mp4":
            fail(f"uploaded RSS item {idx} enclosure type {enclosure.attrib.get('type', '')!r}, expected 'audio/mp4'")
        if not enclosure.attrib.get("url", "").endswith(f"episode-{idx}.m4a"):
            fail(f"uploaded RSS item {idx} enclosure URL {enclosure.attrib.get('url', '')!r} did not reference episode-{idx}.m4a")

        image = item.find(f"{ITUNES}image")
        if image is None:
            fail(f"uploaded RSS item {idx} missing itunes:image")
        expected_image = "artwork/episode-1.jpg" if idx == 1 else "artwork/podcast-cover.jpg"
        if not image.attrib.get("href", "").endswith(expected_image):
            fail(f"uploaded RSS item {idx} image href {image.attrib.get('href', '')!r} did not reference {expected_image!r}")


if __name__ == "__main__":
    main()
