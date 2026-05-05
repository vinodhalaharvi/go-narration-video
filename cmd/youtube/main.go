// youtube uploads out.mp4 to YouTube using metadata from walkthrough/youtube.md.
//
// Credentials are read from ~/.config/go-narration-video/credentials.json
// (the OAuth client_secret JSON downloaded from Google Cloud Console).
//
// First run: opens browser for OAuth consent. Refresh token saved at
// ~/.config/go-narration-video/token.json for subsequent uploads.
//
// See docs/youtube-setup.md for the one-time setup.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	Title      string   `yaml:"title"`
	Tags       []string `yaml:"tags"`
	Visibility string   `yaml:"visibility"`
}

// installedCredentials matches the JSON shape Google Cloud Console
// produces for "Desktop app" OAuth clients.
type installedCredentials struct {
	Installed struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		AuthURI      string   `json:"auth_uri"`
		TokenURI     string   `json:"token_uri"`
		RedirectURIs []string `json:"redirect_uris"`
	} `json:"installed"`
}

func main() {
	if len(os.Args) < 2 {
		fail("usage: youtube <video.mp4>")
	}
	videoPath := os.Args[1]

	if _, err := os.Stat(videoPath); err != nil {
		fail("video file not found: %s", videoPath)
	}

	metaPath := "walkthrough/youtube.md"
	if _, err := os.Stat(metaPath); err != nil {
		fail("metadata not found: %s\n  Each walkthrough needs a youtube.md", metaPath)
	}

	title, description, tags := parseMeta(metaPath)
	if title == "" {
		fail("title not found in %s", metaPath)
	}

	cfg, err := loadOAuthConfig()
	if err != nil {
		fail("%v", err)
	}

	token, err := loadOrAuth(cfg)
	if err != nil {
		fail("auth: %v", err)
	}

	ctx := context.Background()
	client := cfg.Client(ctx, token)
	yt, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		fail("youtube service: %v", err)
	}

	video := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       title,
			Description: description,
			Tags:        tags,
			CategoryId:  "28", // Science & Technology
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus:           "public",
			SelfDeclaredMadeForKids: false,
		},
	}

	f, err := os.Open(videoPath)
	if err != nil {
		fail("open %s: %v", videoPath, err)
	}
	defer f.Close()

	fmt.Fprintln(os.Stderr, "→ Uploading to YouTube (this can take 30-60s)...")
	call := yt.Videos.Insert([]string{"snippet", "status"}, video)
	resp, err := call.Media(f).Do()
	if err != nil {
		fail("upload: %v", err)
	}

	fmt.Println()
	fmt.Println("✓ Uploaded successfully")
	fmt.Printf("  Video ID: %s\n", resp.Id)
	fmt.Printf("  URL:      https://youtube.com/watch?v=%s\n", resp.Id)
	if strings.Contains(strings.ToLower(description), "#shorts") {
		fmt.Printf("  Shorts:   https://youtube.com/shorts/%s\n", resp.Id)
	}
	fmt.Printf("  Studio:   https://studio.youtube.com/video/%s/edit\n", resp.Id)
}

// loadOAuthConfig reads credentials.json from ~/.config/go-narration-video/
// and constructs an oauth2.Config.
func loadOAuthConfig() (*oauth2.Config, error) {
	credPath := credentialsFile()
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("credentials not found: %s\n\n"+
			"  1. Download OAuth Desktop client JSON from\n"+
			"     https://console.cloud.google.com/apis/credentials\n"+
			"  2. Move it: mv ~/Downloads/client_secret_*.json %s\n"+
			"  3. chmod 600 %s\n\n"+
			"  See docs/youtube-setup.md for full setup.",
			credPath, credPath, credPath)
	}

	var creds installedCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse %s: %w", credPath, err)
	}

	if creds.Installed.ClientID == "" {
		return nil, fmt.Errorf("%s: no client_id (is this a Desktop app credentials file?)", credPath)
	}

	return &oauth2.Config{
		ClientID:     creds.Installed.ClientID,
		ClientSecret: creds.Installed.ClientSecret,
		RedirectURL:  "http://localhost:8723/callback",
		Scopes:       []string{youtube.YoutubeUploadScope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  creds.Installed.AuthURI,
			TokenURL: creds.Installed.TokenURI,
		},
	}, nil
}

func parseMeta(path string) (title, description string, tags []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fail("read %s: %v", path, err)
	}
	content := string(data)

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		fail("%s: malformed frontmatter (need two --- markers)", path)
	}

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		fail("yaml parse: %v", err)
	}

	return fm.Title, strings.TrimSpace(parts[2]), fm.Tags
}

func loadOrAuth(cfg *oauth2.Config) (*oauth2.Token, error) {
	tokenPath := tokenFile()
	if data, err := os.ReadFile(tokenPath); err == nil {
		var tok oauth2.Token
		if err := json.Unmarshal(data, &tok); err == nil {
			return &tok, nil
		}
	}

	tok, err := authBrowser(cfg)
	if err != nil {
		return nil, err
	}

	os.MkdirAll(filepath.Dir(tokenPath), 0700)
	data, _ := json.MarshalIndent(tok, "", "  ")
	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}
	fmt.Fprintf(os.Stderr, "✓ token saved to %s\n", tokenPath)
	return tok, nil
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "go-narration-video")
}

func credentialsFile() string { return filepath.Join(configDir(), "credentials.json") }
func tokenFile() string       { return filepath.Join(configDir(), "token.json") }

func authBrowser(cfg *oauth2.Config) (*oauth2.Token, error) {
	codeCh := make(chan string)
	errCh := make(chan error)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			fmt.Fprintln(w, "<h1>Auth failed</h1><p>"+errMsg+"</p>")
			errCh <- fmt.Errorf("oauth callback: %s", errMsg)
			return
		}
		fmt.Fprintln(w, "<h1>Auth complete</h1><p>You can close this tab.</p>")
		codeCh <- code
	})

	srv := &http.Server{Addr: ":8723", Handler: mux}
	go srv.ListenAndServe()
	defer srv.Shutdown(context.Background())

	authURL := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Fprintln(os.Stderr, "→ Opening browser for YouTube authorization...")
	fmt.Fprintf(os.Stderr, "  If it doesn't open, visit:\n  %s\n", authURL)
	openBrowser(authURL)

	select {
	case code := <-codeCh:
		return cfg.Exchange(context.Background(), code)
	case err := <-errCh:
		return nil, err
	}
}

func openBrowser(u string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		return
	}
	args = append(args, u)
	exec.Command(cmd, args...).Start()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}
