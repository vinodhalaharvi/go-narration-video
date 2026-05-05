# PureAST Walkthroughs

Self-contained scripts and source files for narrating
[PureAST](https://github.com/Pure-Company/pureast) — a functorial AST
extraction toolkit for Go.

## Layout

```
walkthrough-pureast/
├── long/            One ~10-12 minute video covering the whole library
│   ├── *.go         Source files (copied from pureast)
│   └── script.txt   Narration with [[file:NAME line:N]] markers
└── shorts/          Ten focused 30-50 second shorts
    ├── 01/          Each short has its own subset of files + script
    ├── 02/
    └── ...
```

## Render the long-form

```bash
make use-pureast-long
make build
make open
```

Long-form is 14 narration beats across 5 files
(`node.go`, `visitor.go`, `extract.go`, `directory.go`, `dump.go`).

## Render a short

List available shorts:

```bash
make list-pureast-shorts
```

Output:
```
01  Stop sending whole files to your LLM
02  An AST in 30 seconds
03  Monoids in 60 seconds
04  Visitor pattern, but better
05  Pure functions = free parallelism
06  Token budgets that don't break syntax
07  One symbol, with everything it needs
08  PR context for an LLM, without the bloat
09  Give Claude direct access to your codebase
10  Why functorial design scales
```

Render one:

```bash
make use-pureast-short N=01
make short                    # renders vertical 9:16 with captions
make open
```

Or render with a specific voice:

```bash
make use-pureast-short N=03
SHORT=1 PROVIDER=elevenlabs VOICE=charlie make audio
make render
make open
```

## Batch render all 10 shorts

```bash
for n in 01 02 03 04 05 06 07 08 09 10; do
  make use-pureast-short N=$n
  make short
  make archive NAME=pureast-short-$n
done
```

Each archived file ends up in `archive/pureast-short-NN.mp4`.

## Editing

The scripts are designed to be tweaked. Open `walkthrough-pureast/long/script.txt`
or any short's `script.txt` and adjust narration. Line numbers in `[[line:N]]`
markers are validated by the build — if a marker doesn't resolve in the audio
transcript, you'll see a warning during `make audio`.
