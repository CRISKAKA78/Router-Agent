"""Rasterize actual WPF component XPS at four densities; not a physical display/DPI test."""
from pathlib import Path
import sys
import fitz

root = Path(sys.argv[1])
sources = sorted(root.glob("components-*.xps"))
assert len(sources) == 4, "Run Desktop.Tests component checks first"
for source in sources:
    with fitz.open(source) as document:
        for scale in (1, 1.25, 1.5, 2):
            pixels = document[0].get_pixmap(matrix=fitz.Matrix(4 / 3 * scale, 4 / 3 * scale), alpha=False)
            pixels.save(str(source.with_name(f"{source.stem}@{round(scale * 100)}.png")))
            print(f"PASS WPF vector raster density {source.stem}: {scale * 100:g}% / {pixels.width}x{pixels.height}")
print("16 density renders; layout and physical desktop scaling are separate checks.")
