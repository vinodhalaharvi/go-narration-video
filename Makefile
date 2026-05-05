.DEFAULT_GOAL := help
.PHONY: help install init audio render build short voices rebuild-audio use-pureast-long use-pureast-short list-pureast-shorts watch studio clean reset preview new add clear-walkthrough check open archive distclean

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
	@echo "  keep narration under 60 seconds"
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
	@test -f $(META) || echo '{"durationSec":5,"format":"long","title":""}' > $(META)
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
audio: check  ## Generate narration MP3 + schedule + codeFiles from script
	@echo "$(CYAN)→ Running Go pipeline (TTS + Whisper + schedule)...$(RESET)"
	@go run ./cmd/build

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

short:  ## Build a vertical short (1080x1920 with baked captions)
	@echo "$(CYAN)→ Generating shorts-mode audio + schedule...$(RESET)"
	@SHORT=1 $(MAKE) audio
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

rebuild-audio: check  ## Regenerate audio with a different voice (keeps existing render setup)
	@echo "$(CYAN)→ Regenerating audio (VOICE=$$VOICE PROVIDER=$$PROVIDER)...$(RESET)"
	@go run ./cmd/build
	@echo "$(GREEN)✓ Audio regenerated. Run 'make render' to update video.$(RESET)"

# ========================================
# PureAST walkthroughs (swap into walkthrough/)
# ========================================
use-pureast-long:  ## Load the long-form PureAST walkthrough into walkthrough/
	@test -d walkthrough-pureast/long || { echo "$(RED)✗ walkthrough-pureast/long not found$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Loading PureAST long-form walkthrough...$(RESET)"
	@go run ./cmd/embed --clear
	@cp walkthrough-pureast/long/*.go walkthrough/
	@cp walkthrough-pureast/long/script.txt walkthrough/script.txt
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
		title=$$(grep -m1 -oP '(?<=\[\[title:)[^\]]+' $$d/script.txt 2>/dev/null || echo "(no title)"); \
		printf "  $(GREEN)%s$(RESET)  %s\n" "$$n" "$$title"; \
	done

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
