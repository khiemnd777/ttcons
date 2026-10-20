"""Read the generated PDF independently; do not change the source or workbooks."""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path

import pdfplumber
from pypdf import PdfReader


ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[2]
SOURCE = ROOT / "huong-dan-demo-cong-trinh.md"
PDF = ROOT / "huong-dan-demo-cong-trinh.pdf"


def normalized(value):
    return " ".join(value.split())


def verify():
    checks = []

    def require(condition, name):
        checks.append({"check": name, "passed": bool(condition)})

    markdown = SOURCE.read_text(encoding="utf-8")
    metadata = json.loads((ROOT / "build-metadata.json").read_text(encoding="utf-8"))
    manifest = json.loads((ROOT / "image-sources.json").read_text(encoding="utf-8"))
    expected_headings = [(len(m[1]), m[2]) for m in re.finditer(r"^(#{1,3}) (.+)$", markdown, re.M) if m[2] != "Mục lục"]
    require(expected_headings == [(h["rank"], h["title"]) for h in metadata["headings"]], "All source headings represented in contents")
    refs = re.findall(r"!\[[^\]]*\]\(([^)]+)\)", markdown)
    require(len(refs) == 95, "69 retained image references plus 26 acceptance images")
    require(all((ROOT / path).is_file() for path in refs), "All image references resolve")
    require(len(manifest) == 26 and len({item["image"] for item in manifest}) == 26, "26 unique acceptance image origins")
    require({p for p in refs if p.startswith("images/bmgs/")} == {item["image"] for item in manifest}, "Every new image has provenance and is used")
    for item in manifest:
        copied = ROOT / item["image"]
        original = REPO / item["source"]
        require(hashlib.sha256(copied.read_bytes()).hexdigest() == item["sha256"] == hashlib.sha256(original.read_bytes()).hexdigest(), "Unchanged acceptance screenshot: " + copied.name)

    old_source = REPO / "output/pdf/demo-guide-2026-09-25/huong-dan-demo-cong-trinh.md"
    old = old_source.read_text(encoding="utf-8")
    for match in re.finditer(r"!\[[^\]]*\]\(([^)]+)\)", old):
        image_path = match[1]
        require(match[0] in markdown and (ROOT / image_path).read_bytes() == (old_source.parent / image_path).read_bytes(), "Retained original screenshot: " + image_path)

    reader = PdfReader(PDF)
    require(len(reader.pages) == metadata["pages"], "PDF page count equals build record")
    texts = [normalized(page.extract_text()) for page in reader.pages]
    contents_text = " ".join(texts[: metadata["headings"][1]["page"] - 1])
    for heading in metadata["headings"]:
        title = normalized(heading["title"])
        require(title in texts[heading["page"] - 1], f"Heading printed on page {heading['page']}: {title}")
        require(title in contents_text, "Heading is present in printed contents: " + title)
        require(f"| {heading['title']} | {heading['page']} |" in markdown, "Markdown contents page matches PDF: " + title)
    for index, text in enumerate(texts, 1):
        require(f"Trang {index}" in text and "16.10.2026" in text, f"Page {index}: current edition and footer")
        require("\ufffd" not in text and "\u25a0" not in text, f"Page {index}: no replacement glyphs")
        require(len(text) > 140, f"Page {index}: not a blank page")

    # Check the actual destination page of every clickable contents link.
    expected_pages = {h["page"] - 1 for h in metadata["headings"]}
    links = []
    broken_links = []
    page_indices = {page.indirect_reference.idnum: i for i, page in enumerate(reader.pages)}
    for page in reader.pages[: metadata["headings"][1]["page"] - 1]:
        for reference in page.get("/Annots", []):
            annotation = reference.get_object()
            if annotation.get("/Subtype") == "/Link":
                destination = annotation.get("/Dest")
                if destination and destination[0].idnum in page_indices:
                    links.append(page_indices[destination[0].idnum])
                else:
                    broken_links.append(str(destination))
    require(len(links) == 2 * len(metadata["headings"]) and not broken_links, "Every contents title and page number has a valid internal link")
    require(set(links) == expected_pages, "Contents link destinations match heading pages")

    image_count = 0
    geometry_issues = []
    font_names = set()
    with pdfplumber.open(PDF) as pdf:
        for number, page in enumerate(pdf.pages, 1):
            require(abs(page.width - 841.89) < 0.1 and abs(page.height - 595.276) < 0.1, f"Page {number}: A4 landscape")
            for picture in page.images:
                image_count += 1
                if picture["x0"] < 38 or picture["x1"] > page.width - 38 or picture["top"] < 45 or picture["bottom"] > page.height - 42:
                    geometry_issues.append({"page": number, "kind": "image", "box": [picture[k] for k in ["x0", "top", "x1", "bottom"]]})
            for char in page.chars:
                font_names.add(char["fontname"])
                if char["x0"] < 38 or char["x1"] > page.width - 38 or char["top"] < 10 or char["bottom"] > page.height - 9:
                    geometry_issues.append({"page": number, "kind": "text", "text": char["text"]})
    require(image_count == len(refs), "All 95 referenced screenshots are embedded in the PDF")
    require(not geometry_issues, "No text or image extends outside page bounds")
    require(all("Tahoma" in font for font in font_names), "All rendered text uses the Vietnamese-capable guide font")

    result = {"source_sha256": hashlib.sha256(SOURCE.read_bytes()).hexdigest(),
              "pdf_sha256": hashlib.sha256(PDF.read_bytes()).hexdigest(),
              "pdf_pages": len(reader.pages), "contents_entries": len(expected_headings),
              "image_references": len(refs), "embedded_images": image_count,
              "acceptance_images": len(manifest), "geometry_issues": geometry_issues,
              "checks": len(checks), "passed": sum(c["passed"] for c in checks),
              "failures": [c["check"] for c in checks if not c["passed"]]}
    (ROOT / "verification.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False, indent=2))
    if result["failures"]:
        raise SystemExit(1)


if __name__ == "__main__":
    verify()
