.DEFAULT_GOAL := help
.PHONY: help install audio render build watch studio clean reset preview new check open archive distclean

# ========================================
# Configuration
# ========================================
COMPOSITION := GoWalkthrough
OUTPUT      := out.mp4
SCRIPT      := script.txt
AUDIO       := public/narration.mp3
SCHEDULE    := src/schedule.json

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
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-12s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Common workflow:$(RESET)"
	@echo "  make new SRC=path/to/your.go   # embed a new Go file"
	@echo "  edit script.txt                # write narration with [[line:N]] markers"
	@echo "  make build                     # generate audio + render video"
	@echo "  make open                      # play the result"
	@echo ""

# ========================================
# Setup
# ========================================
install:  ## Install all dependencies (npm + go modules)
	@echo "$(CYAN)→ Installing npm dependencies...$(RESET)"
	@npm install
	@echo "$(CYAN)→ Tidying Go modules...$(RESET)"
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies installed$(RESET)"

check:  ## Verify environment is set up correctly
	@echo "$(CYAN)Checking environment...$(RESET)"
	@command -v go >/dev/null 2>&1 || { echo "$(RED)✗ go not found$(RESET)"; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "$(RED)✗ node not found$(RESET)"; exit 1; }
	@command -v npx >/dev/null 2>&1 || { echo "$(RED)✗ npx not found$(RESET)"; exit 1; }
	@test -n "$$OPENAI_API_KEY" || { echo "$(RED)✗ OPENAI_API_KEY not set$(RESET)"; exit 1; }
	@test -d node_modules || { echo "$(YELLOW)⚠ node_modules missing — run 'make install'$(RESET)"; exit 1; }
	@test -f $(SCRIPT) || { echo "$(RED)✗ $(SCRIPT) not found$(RESET)"; exit 1; }
	@echo "$(GREEN)✓ Environment OK$(RESET)"

# ========================================
# Pipeline
# ========================================
audio: check  ## Generate narration MP3 + schedule from script.txt
	@echo "$(CYAN)→ Running Go pipeline (TTS + Whisper + schedule)...$(RESET)"
	@go run main.go

render:  ## Render MP4 from current schedule + audio (no regen)
	@test -f $(AUDIO) || { echo "$(RED)✗ Missing $(AUDIO) — run 'make audio' first$(RESET)"; exit 1; }
	@test -f $(SCHEDULE) || { echo "$(RED)✗ Missing $(SCHEDULE) — run 'make audio' first$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Rendering $(COMPOSITION) → $(OUTPUT)...$(RESET)"
	@rm -rf node_modules/.cache $(OUTPUT)
	@npx remotion render $(COMPOSITION) $(OUTPUT)
	@echo "$(GREEN)✓ Rendered $(OUTPUT)$(RESET)"

build: audio render  ## Full pipeline: audio + schedule + render

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

watch:  ## Re-render whenever script.txt changes (requires fswatch)
	@command -v fswatch >/dev/null 2>&1 || { echo "$(RED)✗ Install fswatch: brew install fswatch$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Watching $(SCRIPT) for changes... (Ctrl-C to stop)$(RESET)"
	@fswatch -o $(SCRIPT) | xargs -n1 -I{} make build

open:  ## Play the rendered video
	@test -f $(OUTPUT) || { echo "$(RED)✗ No $(OUTPUT) yet — run 'make build'$(RESET)"; exit 1; }
	@open $(OUTPUT)

# ========================================
# Multi-file workflow
# ========================================
new:  ## Set up walkthrough from a Go file: make new SRC=path/to/file.go
	@test -n "$(SRC)" || { echo "$(RED)Usage: make new SRC=path/to/file.go$(RESET)"; exit 1; }
	@test -f "$(SRC)" || { echo "$(RED)✗ $(SRC) not found$(RESET)"; exit 1; }
	@echo "$(CYAN)→ Embedding $(SRC) into Composition.tsx...$(RESET)"
	@./scripts/embed-go.py "$(SRC)"
	@echo "$(GREEN)✓ Embedded. Now edit script.txt with [[line:N]] markers, then 'make build'$(RESET)"

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

reset: clean  ## Clean + remove generated audio and schedule
	@rm -f $(AUDIO) $(SCHEDULE)
	@echo "$(GREEN)✓ Reset to script-only state$(RESET)"

distclean: reset  ## Nuclear: also wipes node_modules
	@rm -rf node_modules/
	@echo "$(GREEN)✓ Removed node_modules$(RESET)"
