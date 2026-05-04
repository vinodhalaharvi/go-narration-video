#!/usr/bin/env python3
"""
Embed a Go source file into src/Composition.tsx as the goCode template literal.

Usage: scripts/embed-go.py path/to/file.go
"""
import re
import sys
from pathlib import Path


def main():
    if len(sys.argv) != 2:
        print("Usage: scripts/embed-go.py path/to/file.go", file=sys.stderr)
        sys.exit(1)

    src_path = Path(sys.argv[1])
    comp_path = Path("src/Composition.tsx")

    if not src_path.is_file():
        print(f"✗ Source file not found: {src_path}", file=sys.stderr)
        sys.exit(1)

    if not comp_path.is_file():
        print(f"✗ {comp_path} not found — run from project root", file=sys.stderr)
        sys.exit(1)

    go_code = src_path.read_text()

    escaped = (
        go_code.replace("\\", "\\\\")
        .replace("`", "\\`")
        .replace("${", "\\${")
    )

    content = comp_path.read_text()

    pattern = re.compile(r"const goCode = `[^`]*`;", re.DOTALL)
    if not pattern.search(content):
        print(f"✗ Could not find 'const goCode = `...`;' in {comp_path}", file=sys.stderr)
        sys.exit(1)

    new_block = f"const goCode = `{escaped}`;"
    new_content = pattern.sub(lambda m: new_block, content, count=1)

    comp_path.write_text(new_content)
    line_count = len(go_code.splitlines())
    print(f"✓ Embedded {src_path} ({line_count} lines) into {comp_path}")


if __name__ == "__main__":
    main()
