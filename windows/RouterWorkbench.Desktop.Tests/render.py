"""Render the actual WPF vector output for visual QA when remote desktop capture is unavailable."""
from pathlib import Path
import sys
import fitz

root = Path(sys.argv[1] if len(sys.argv) > 1 else "build/windows-desktop/verification")
files = sorted(root.glob("*.xps"))
if not files:
    raise SystemExit("No WPF XPS renders found")
for path in files:
    with fitz.open(path) as doc:
        assert len(doc) == 1 and len(doc[0].get_text()) > 150, f"Empty WPF vector output: {path}"
        pixmap = doc[0].get_pixmap(matrix=fitz.Matrix(4/3, 4/3), alpha=False)
        assert len(set(pixmap.samples)) > 20, f"Blank rendered page: {path}"
        pixmap.save(str(path.with_suffix(".png")))
    print(f"PASS WPF vector render {path.stem}")
print(f"Rendered {len(files)} actual WPF layouts; excludes native title bar and WebView2 pixels.")
