.DEFAULT_GOAL := help
.PHONY: help install init audio render build short voices rebuild-audio use-pureast-long use-pureast-short list-pureast-shorts use-fseam list-fseams use-typewriter list-typewriter youtube-meta publish watch studio clean reset preview new add clear-walkthrough check open archive distclean

# ========================================
# Configuration
# ========================================
COMPOSITION := GoWalkthrough
OUTPUT      := out.mp4
SCRIPT      := walkthrough/script.txt
AUDIO       := public/narration.mp3
SCHEDULE    := src/schedule.json
META        := src/meta.json
CODEFILES   := src/codeFiles.json
CAPTIONS    := src/captions.json

CYAN   := \033[0;36m
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
RESET  := \033[0m

# ========================================
# Help
# ========================================
help:  ## Show this help
	@echo ""
	@echo "$(CYAN)Go Narration Video — Makefile commands$(RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-18s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Single-file walkthrough:$(RESET)"
	@echo "  make add SRC=path/to/your.go"
	@echo "  edit walkthrough/script.txt"
	@echo "  make build && make open"
	@echo ""
	@echo "$(YELLOW)Multi-file walkthrough:$(RESET)"
	@echo "  make add SRC=path/to/file1.go"
	@echo "  make add SRC=path/to/file2.go"
	@echo "  edit walkthrough/script.txt with [[file:NAME line:N]] markers"
	@echo "  make build && make open"
	@echo ""
	@echo "$(YELLOW)YouTube short (vertical 9:16 with captions):$(RESET)"
	@echo "  edit walkthrough/script.txt — add [[title:Your Hook]] at top"
	@echo "  keep narration under 60s for best engagement (180s is the hard cap)"
	@echo "  make short && make open"
	@echo ""
	@echo "$(YELLOW)Voice selection:$(RESET)"
	@echo "  make voices                            # list all options"
	@echo "  make build VOICE=onyx                  # use OpenAI's onyx"
	@echo "  make build PROVIDER=elevenlabs VOICE=adam"
	@echo ""

# ========================================
# Setup
# ========================================
install: init  ## Install all dependencies (npm + go modules)
	@echo "$(CYAN)→ Installing npm dependencies...$(RESET)"
	@npm install
	@echo "$(CYAN)→ Tidying Go modules...$(RESET)"
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies installed$(RESET)"

init:  ## Create placeholder generated files so the project compiles before first build
	@mkdir -p walkthrough src public
	@test -f $(SCHEDULE) || echo '[{"file":"main.go","line":1,"startSec":0}]' > $(SCHEDULE)
	@test -f $(META) || echo '{"durationSec":5,"format":"long","title":"","introIcon":"","introText":"","typewriter":"","typewriterReveals":[]}' > $(META)
	@test -f $(CODEFILES) || echo '{}' > $(CODEFILES)
	@test -f $(CAPTIONS) || echo '[]' > $(CAPTIONS)

check:  ## Verify environment is set up correctly
	@echo "$(CYAN)Checking environment...$(RESET)"
	@command -v go >/dev/null 2>&1 || { echo "$(RED)✗ go not found$(RESET)"; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "$(RED)✗ node not found$(RESET)"; exit 1; }
	@command -v npx >/dev/null 2>&1 || { echo "$(RED)✗ npx not found$(RESET)"; exit 1; }
	@test -n "$$OPENAI_API_KEY" || { echo "$(RED)✗ OPENAI_API_KEY not set$(RESET)"; exit 1; }
	@test -d node_modules || { echo "$(YELLOW)⚠ node_modules missing — run 'make install'$(RESET)"; exit 1; }
	@test -f $(SCRIPT) || { echo "$(RED)✗ $(SCRIPT) not found — add files via 'make add' first$(RESET)"; exit 1; }
	@echo "$(GREEN)✓ Environment OK$(RESET)"

# ========================================
# Pipeline
# ========================================
audio: check  ## Generate narration MP3 + schedule + codeFiles from script. Optional: MODE=typewriter|off GRANULARITY=line|word SPEED=0.85
	@echo "$(CYAN)→ Running Go pipeline (TTS + Whisper + schedule)...$(RESET)"
	@MODE="$(MODE)" GRANULARITY="$(GRANULARITY)" SPEED="$(SPEED)" go run ./cmd/build

render: init  ## Render MP4 (auto-generates audio if missing)
	@if [ ! -f $(AUDIO) ]; then \
		echo "$(YELLOW)⚠ Missing audio — generating first...$(RESET)"; \
		$(MAKE) audio; \
	fi
	@echo "$(CYAN)→ Rendering $(COMPOSITION) → $(OUTPUT)...$(RESET)"
	@rm -rf node_modules/.cache $(OUTPUT)
	@npx remotion render $(COMPOSITION) $(OUTPUT)
	@echo "$(GREEN)✓ Rendered $(OUTPUT)$(RESET)"

build: audio render  ## Full pipeline: audio + schedule + render (long-form 1920x1080)

short:  ## Build a vertical short (1080x1920 with baked captions). Optional: MODE=typewriter|off GRANULARITY=line|word SPEED=0.85
	@echo "$(CYAN)→ Generating shorts-mode audio + schedule...$(RESET)"
	@SHORT=1 MODE="$(MODE)" GRANULARITY="$(GRANULARITY)" SPEED="$(SPEED)" $(MAKE) audio
	@echo "$(CYAN)→ Rendering shorts-mode video...$(RESET)"
	@rm -rf node_modules/.cache $(OUTPUT)
	@npx remotion render $(COMPOSITION) $(OUTPUT)
	@echo "$(GREEN)✓ Rendered $(OUTPUT) (vertical short)$(RESET)"

# ========================================
# Voice presets
# Override with VOICE=name and/or PROVIDER=openai|elevenlabs
# ========================================
voices:  ## List available voice options
	@echo ""
	@echo "$(CYAN)OpenAI voices$(RESET) (PROVIDER=openai, default):"
	@echo "  $(GREEN)nova$(RESET)    — female, warm, conversational (current default)"
	@echo "  $(GREEN)sage$(RESET)    — female, measured, clear"
	@echo "  $(GREEN)onyx$(RESET)    — male, deep, authoritative — popular for tech"
	@echo "  $(GREEN)ash$(RESET)     — male, conversational"
	@echo "  $(GREEN)verse$(RESET)   — male, dynamic"
	@echo "  $(GREEN)coral$(RESET)   — female, friendly"
	@echo "  $(GREEN)alloy$(RESET)   — neutral"
	@echo "  $(GREEN)echo$(RESET)    — male, classic"
	@echo "  $(GREEN)fable$(RESET)   — narrative"
	@echo "  $(GREEN)shimmer$(RESET) — female, light"
	@echo ""
	@echo "$(CYAN)ElevenLabs voices$(RESET) (PROVIDER=elevenlabs, requires ELEVENLABS_API_KEY):"
	@echo "  $(GREEN)adam$(RESET)    — male, warm — most popular for tech"
	@echo "  $(GREEN)brian$(RESET)   — male, mature, deep"
	@echo "  $(GREEN)rachel$(RESET)  — female, calm"
	@echo "  $(GREEN)bella$(RESET)   — female, energetic"
	@echo "  $(GREEN)antoni$(RESET)  — male, narrative"
	@echo "  $(GREEN)charlie$(RESET) — male, casual"
	@echo "  $(GREEN)daniel$(RESET)  — male, authoritative"
	@echo "  $(GREEN)emily$(RESET)   — female, soft"
	@echo "  $(GREEN)george$(RESET)  — male, mature"
	@echo ""
	@echo "$(YELLOW)Examples:$(RESET)"
	@echo "  make build VOICE=onyx"
	@echo "  make build PROVIDER=elevenlabs VOICE=adam"
	@echo "  make build PROVIDER=elevenlabs VOICE=brian"
	@echo "  make rebuild-audio VOICE=sage   # regenerate just audio without rebuilding video"
	@echo ""

rebuild-audio: check  ## Regenerate audio with a different voice/speed (keeps existing render setup)
	@echo "$(CYAN)→ Regenerating audio (VOICE=$$VOICE PROVIDER=$$PROVIDER SPEED=$$SPEED)...$(RESET)"
	@MODE="$(MODE)" GRANULARITY="$(GRANULARITY)" SPEED="$(SPEED)" go run ./cmd/build
	@echo "$(GREEN)✓ Audio regenerated. Run 'make render' to update video.$(RESET)"

# ========================================
# PureAST walkthroughs (swap into walkthrough/)
# ========================================
PUREAST_DUMPS ?= $(HOME)/pureast-stdlib-dumps

use-pureast-long:  ## Load the long-form PureAST walkthrough (needs stdlib dumps)
	@test -d walkthrough-pureast/long || { echo "$(RED)✗ walkthrough-pureast/long not found$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Loading PureAST long-form walkthrough...$(RESET)"
	@go run ./cmd/embed --clear
	@cp walkthrough-pureast/long/*.go walkthrough/
	@cp walkthrough-pureast/long/script.txt walkthrough/script.txt
	@echo "$(CYAN)→ Looking for stdlib dumps in $(PUREAST_DUMPS)...$(RESET)"
	@if [ -d "$(PUREAST_DUMPS)" ]; then \
		missing=0; \
		for f in nethttp.go context.go io.go sync.go; do \
			if [ -f "$(PUREAST_DUMPS)/$$f" ]; then \
				cp "$(PUREAST_DUMPS)/$$f" walkthrough/; \
				echo "  $(GREEN)✓$(RESET) $$f"; \
			else \
				echo "  $(YELLOW)⚠$(RESET) $$f missing in $(PUREAST_DUMPS)"; \
				missing=$$((missing+1)); \
			fi; \
		done; \
		if [ $$missing -gt 0 ]; then \
			echo "$(YELLOW)⚠ $$missing stdlib dump(s) missing — those sections will not render$(RESET)"; \
		fi; \
	else \
		echo "$(RED)✗ $(PUREAST_DUMPS) does not exist.$(RESET)"; \
		echo "$(YELLOW)Generate stdlib dumps first:$(RESET)"; \
		echo "  mkdir -p $(PUREAST_DUMPS) && cd $(PUREAST_DUMPS)"; \
		echo "  pureast dump \$$(go env GOROOT)/src/net/http > nethttp.go"; \
		echo "  pureast dump \$$(go env GOROOT)/src/context > context.go"; \
		echo "  pureast dump \$$(go env GOROOT)/src/io > io.go"; \
		echo "  pureast dump \$$(go env GOROOT)/src/sync > sync.go"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Loaded. Run 'make build' to render.$(RESET)"

use-pureast-short:  ## Load a PureAST short: make use-pureast-short N=01
	@test -n "$(N)" || { echo "$(RED)Usage: make use-pureast-short N=01$(RESET)"; exit 1; }
	@test -d walkthrough-pureast/shorts/$(N) || { echo "$(RED)✗ walkthrough-pureast/shorts/$(N) not found$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Loading PureAST short #$(N)...$(RESET)"
	@go run ./cmd/embed --clear
	@cp walkthrough-pureast/shorts/$(N)/*.go walkthrough/
	@cp walkthrough-pureast/shorts/$(N)/script.txt walkthrough/script.txt
	@echo "$(GREEN)✓ Loaded short #$(N). Run 'make short' to render vertical.$(RESET)"

list-pureast-shorts:  ## List all available PureAST shorts with their titles
	@echo "$(CYAN)Available PureAST shorts:$(RESET)"
	@for d in walkthrough-pureast/shorts/*/; do \
		n=$$(basename $$d); \
		title=$$(sed -n 's/.*\[\[title:\([^]]*\)\]\].*/\1/p' $$d/script.txt | head -1); \
		[ -z "$$title" ] && title="(no title)"; \
		printf "  $(GREEN)%s$(RESET)  %s\n" "$$n" "$$title"; \
	done

# ========================================
# Functional seam shorts
# ========================================
use-fseam:  ## Load a functional-seam short: make use-fseam N=01-intro
	@test -n "$(N)" || { echo "$(RED)Usage: make use-fseam N=01-intro|02-no-mocks$(RESET)"; exit 1; }
	@test -d walkthrough-functional-seams/$(N) || { echo "$(RED)✗ walkthrough-functional-seams/$(N) not found$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Loading functional-seam short: $(N)...$(RESET)"
	@go run ./cmd/embed --clear
	@cp walkthrough-functional-seams/$(N)/*.go walkthrough/
	@cp walkthrough-functional-seams/$(N)/script.txt walkthrough/script.txt
	@if [ -f walkthrough-functional-seams/$(N)/youtube.md ]; then \
		cp walkthrough-functional-seams/$(N)/youtube.md walkthrough/youtube.md; \
	fi
	@echo "$(GREEN)✓ Loaded $(N).$(RESET)"

list-fseams:  ## List functional-seam shorts with their titles
	@echo "$(CYAN)Available functional-seam shorts:$(RESET)"
	@for d in walkthrough-functional-seams/*/; do \
		n=$$(basename $$d); \
		title=$$(sed -n 's/.*\[\[title:\([^]]*\)\]\].*/\1/p' $$d/script.txt | head -1); \
		[ -z "$$title" ] && title="(no title)"; \
		printf "  $(GREEN)%-15s$(RESET)  %s\n" "$$n" "$$title"; \
	done

# ========================================
# Typewriter walkthroughs (build code from scratch with reveal animations)
# ========================================
use-typewriter:  ## Load a typewriter walkthrough: make use-typewriter N=01-fold
	@test -n "$(N)" || { echo "$(RED)Usage: make use-typewriter N=01-fold$(RESET)"; exit 1; }
	@test -d walkthrough-typewriter/$(N) || { echo "$(RED)✗ walkthrough-typewriter/$(N) not found$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Loading typewriter walkthrough: $(N)...$(RESET)"
	@go run ./cmd/embed --clear
	@cp walkthrough-typewriter/$(N)/*.go walkthrough/
	@cp walkthrough-typewriter/$(N)/script.txt walkthrough/script.txt
	@if [ -f walkthrough-typewriter/$(N)/youtube.md ]; then \
		cp walkthrough-typewriter/$(N)/youtube.md walkthrough/youtube.md; \
	fi
	@echo "$(GREEN)✓ Loaded $(N).$(RESET)"

list-typewriter:  ## List typewriter walkthroughs
	@echo "$(CYAN)Available typewriter walkthroughs:$(RESET)"
	@for d in walkthrough-typewriter/*/; do \
		n=$$(basename $$d); \
		title=$$(sed -n 's/.*\[\[title:\([^]]*\)\]\].*/\1/p' $$d/script.txt | head -1); \
		[ -z "$$title" ] && title="(no title)"; \
		printf "  $(GREEN)%-15s$(RESET)  %s\n" "$$n" "$$title"; \
	done

# ========================================
# YouTube upload (uses ~/.config/go-narration-video/credentials.json)
# ========================================
youtube-meta:  ## Print upload metadata for the current walkthrough
	@if [ ! -f walkthrough/youtube.md ]; then \
		echo "$(RED)✗ walkthrough/youtube.md not found$(RESET)"; \
		exit 1; \
	fi
	@echo ""
	@echo "$(CYAN)═══ YOUTUBE UPLOAD METADATA ═══$(RESET)"
	@echo ""
	@echo "$(YELLOW)──── TITLE ────$(RESET)"
	@sed -n 's/^title: *//p' walkthrough/youtube.md | head -1
	@echo ""
	@echo "$(YELLOW)──── DESCRIPTION ────$(RESET)"
	@awk '/^---$$/{f++; next} f==2' walkthrough/youtube.md
	@echo ""
	@echo "$(YELLOW)──── TAGS ────$(RESET)"
	@sed -n '/^tags:/,/^[a-z]/p' walkthrough/youtube.md | grep '^  - ' | sed 's/^  - //' | tr '\n' ',' | sed 's/,$$//' | sed 's/,/, /g'
	@echo ""

publish: youtube-meta  ## Upload current short to YouTube (asks for confirmation)
	@if [ ! -f out.mp4 ]; then \
		echo "$(RED)✗ out.mp4 not found — run 'make short' or 'make render' first$(RESET)"; \
		exit 1; \
	fi
	@if [ ! -f $$HOME/.config/go-narration-video/credentials.json ]; then \
		echo "$(RED)✗ credentials.json not found at ~/.config/go-narration-video/credentials.json$(RESET)"; \
		echo "$(YELLOW)  See docs/youtube-setup.md to set up.$(RESET)"; \
		exit 1; \
	fi
	@if command -v ffprobe >/dev/null 2>&1; then \
		duration=$$(ffprobe -v error -show_entries format=duration -of csv=p=0 out.mp4 2>/dev/null | cut -d. -f1); \
		size=$$(ls -lh out.mp4 | awk '{print $$5}'); \
		echo "$(YELLOW)──── FILE INFO ────$(RESET)"; \
		echo "  out.mp4  $$size  $${duration}s"; \
		if [ -n "$$duration" ] && [ $$duration -gt 180 ]; then \
			echo "  $(RED)⚠ Over 180s — YouTube will treat as regular video, not a Short$(RESET)"; \
		elif [ -n "$$duration" ] && [ $$duration -gt 60 ]; then \
			echo "  $(YELLOW)ℹ Over 60s — still a Short (max 180s), but engagement sweet spot is 20-45s$(RESET)"; \
		fi; \
		echo ""; \
	fi
	@printf "$(CYAN)Upload this video to YouTube? [N/y] $(RESET)"
	@read confirm; \
	case "$$confirm" in \
		y|Y|yes|YES) \
			echo "$(CYAN)→ Uploading...$(RESET)"; \
			go run ./cmd/youtube out.mp4 \
			;; \
		*) \
			echo "$(YELLOW)Aborted.$(RESET)" \
			;; \
	esac

# ========================================
# Iteration helpers
# ========================================
studio: check  ## Open Remotion Studio (live preview in browser)
	@npx remotion studio

preview: check  ## Render a single frame as PNG for quick visual check
	@echo "$(CYAN)→ Rendering preview frame...$(RESET)"
	@npx remotion still $(COMPOSITION) preview.png --frame=60
	@echo "$(GREEN)✓ Saved preview.png$(RESET)"
	@command -v open >/dev/null 2>&1 && open preview.png || true

watch:  ## Re-render whenever script changes (requires fswatch)
	@command -v fswatch >/dev/null 2>&1 || { echo "$(RED)✗ Install fswatch: brew install fswatch$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Watching walkthrough/ for changes... (Ctrl-C to stop)$(RESET)"
	@fswatch -o walkthrough/ | xargs -n1 -I{} make build

open:  ## Play the rendered video
	@test -f $(OUTPUT) || { echo "$(RED)✗ No $(OUTPUT) yet — run 'make build'$(RESET)"; exit 1; }
	@open $(OUTPUT)

# ========================================
# Multi-file workflow
# ========================================
add:  ## Add a Go file to the walkthrough: make add SRC=path/to/file.go
	@test -n "$(SRC)" || { echo "$(RED)Usage: make add SRC=path/to/file.go$(RESET)"; exit 1; }
	@test -f "$(SRC)" || { echo "$(RED)✗ $(SRC) not found$(RESET)"; exit 1; }
	@go run ./cmd/embed "$(SRC)"

new:  ## Reset walkthrough and add one file: make new SRC=path/to/file.go
	@test -n "$(SRC)" || { echo "$(RED)Usage: make new SRC=path/to/file.go$(RESET)"; exit 1; }
	@test -f "$(SRC)" || { echo "$(RED)✗ $(SRC) not found$(RESET)"; exit 1; }
	@go run ./cmd/embed --clear
	@go run ./cmd/embed "$(SRC)"

clear-walkthrough:  ## Remove all .go files from walkthrough/
	@go run ./cmd/embed --clear

archive:  ## Save current build with a name: make archive NAME=functor-intro
	@test -n "$(NAME)" || { echo "$(RED)Usage: make archive NAME=video-name$(RESET)"; exit 1; }
	@test -f $(OUTPUT) || { echo "$(RED)✗ No $(OUTPUT) to archive$(RESET)"; exit 1; }
	@mkdir -p archive
	@cp $(OUTPUT) archive/$(NAME).mp4
	@cp $(SCRIPT) archive/$(NAME).script.txt
	@cp $(AUDIO) archive/$(NAME).mp3 2>/dev/null || true
	@echo "$(GREEN)✓ Archived to archive/$(NAME).*$(RESET)"

# ========================================
# Cleanup
# ========================================
clean:  ## Remove generated files (keeps node_modules)
	@rm -f $(OUTPUT) preview.png
	@rm -rf out/ node_modules/.cache
	@echo "$(GREEN)✓ Cleaned outputs$(RESET)"

reset: clean  ## Clean + remove generated audio/schedule/meta/codeFiles/captions
	@rm -f $(AUDIO) $(SCHEDULE) $(META) $(CODEFILES) $(CAPTIONS)
	@echo "$(GREEN)✓ Reset to script-only state$(RESET)"

distclean: reset  ## Nuclear: also wipes node_modules
	@rm -rf node_modules/
	@echo "$(GREEN)✓ Removed node_modules$(RESET)"
