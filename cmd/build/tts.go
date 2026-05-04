// tts.go — text-to-speech abstraction supporting multiple providers.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

// TTS produces an MP3 stream from text.
type TTS interface {
	Synthesize(ctx context.Context, text string) (io.ReadCloser, error)
	Name() string
}

// ============================================================
// OpenAI TTS
// ============================================================

type openaiTTS struct {
	client *openai.Client
	voice  openai.SpeechVoice
	model  openai.SpeechModel
}

// Map our friendly name to the openai constant.
// Newer voices may not be in the older SDK; we cast strings if needed.
func openaiVoice(name string) openai.SpeechVoice {
	switch name {
	case "alloy":
		return openai.VoiceAlloy
	case "echo":
		return openai.VoiceEcho
	case "fable":
		return openai.VoiceFable
	case "onyx":
		return openai.VoiceOnyx
	case "nova":
		return openai.VoiceNova
	case "shimmer":
		return openai.VoiceShimmer
	default:
		// Newer voices like sage/ash/coral/verse — pass through as raw string.
		return openai.SpeechVoice(name)
	}
}

func newOpenAITTS(voice string) *openaiTTS {
	if voice == "" {
		voice = "nova"
	}
	return &openaiTTS{
		client: openai.NewClient(os.Getenv("OPENAI_API_KEY")),
		voice:  openaiVoice(voice),
		model:  "gpt-4o-mini-tts",
	}
}

func (t *openaiTTS) Name() string {
	return fmt.Sprintf("openai/%s/%s", t.model, t.voice)
}

func (t *openaiTTS) Synthesize(ctx context.Context, text string) (io.ReadCloser, error) {
	resp, err := t.client.CreateSpeech(ctx, openai.CreateSpeechRequest{
		Model: t.model,
		Input: text,
		Voice: t.voice,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ============================================================
// ElevenLabs TTS
// ============================================================

type elevenLabsTTS struct {
	apiKey  string
	voiceID string
	voice   string // friendly name for logging
	modelID string
}

// Default voice IDs for the most popular ElevenLabs voices.
// IDs are stable; users can also pass a raw voice ID directly via VOICE.
var elevenLabsVoiceIDs = map[string]string{
	"rachel":  "21m00Tcm4TlvDq8ikWAM",
	"adam":    "pNInz6obpgDQGcFmaJgB",
	"antoni":  "ErXwobaYiN019PkySvjV",
	"bella":   "EXAVITQu4vr4xnSDxMaL",
	"brian":   "nPczCjzI2devNBz1zQrb",
	"charlie": "IKne3meq5aSn9XLyUdCD",
	"daniel":  "onwK4e9ZLuTAKqWW03F9",
	"emily":   "LcfcDJNUP1GQjkzn1xUU",
	"sarah":   "EXAVITQu4vr4xnSDxMaL",
	"george":  "JBFqnCBsd6RMkjVDRZzb",
}

func newElevenLabsTTS(voice string) (*elevenLabsTTS, error) {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ELEVENLABS_API_KEY not set")
	}

	if voice == "" {
		voice = "adam"
	}

	// If voice looks like a 20-char alphanumeric ID, pass through.
	// Otherwise look up the friendly name.
	voiceID, ok := elevenLabsVoiceIDs[voice]
	if !ok {
		// Assume it's a raw voice ID
		voiceID = voice
	}

	return &elevenLabsTTS{
		apiKey:  apiKey,
		voiceID: voiceID,
		voice:   voice,
		// eleven_turbo_v2_5 is the best balance of quality and speed.
		// Use eleven_multilingual_v2 for highest quality if needed.
		modelID: "eleven_turbo_v2_5",
	}, nil
}

func (t *elevenLabsTTS) Name() string {
	return fmt.Sprintf("elevenlabs/%s/%s", t.modelID, t.voice)
}

func (t *elevenLabsTTS) Synthesize(ctx context.Context, text string) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", t.voiceID)

	body, _ := json.Marshal(map[string]any{
		"text":     text,
		"model_id": t.modelID,
		"voice_settings": map[string]any{
			"stability":         0.5,
			"similarity_boost":  0.75,
			"style":             0.0,
			"use_speaker_boost": true,
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")
	req.Header.Set("xi-api-key", t.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("elevenlabs %d: %s", resp.StatusCode, string(errBody))
	}
	return resp.Body, nil
}

// ============================================================
// Factory
// ============================================================

// NewTTS picks a provider based on env vars:
//   PROVIDER=openai (default)
//   PROVIDER=elevenlabs
//   VOICE=<voice name or id>
func NewTTS() (TTS, error) {
	provider := os.Getenv("PROVIDER")
	voice := os.Getenv("VOICE")

	switch provider {
	case "elevenlabs", "11labs", "eleven":
		return newElevenLabsTTS(voice)
	case "openai", "":
		return newOpenAITTS(voice), nil
	default:
		return nil, fmt.Errorf("unknown PROVIDER %q (use 'openai' or 'elevenlabs')", provider)
	}
}
