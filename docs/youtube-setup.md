# YouTube Upload Setup (one-time, ~10 min)

The `make publish` command uploads `out.mp4` to YouTube. Setup is one-time.

## 1. Create OAuth Desktop credentials

```bash
# Open Google Cloud Console
open "https://console.cloud.google.com/projectcreate"
# → Name: go-narration-video → Create

open "https://console.cloud.google.com/apis/library/youtube.googleapis.com"
# → Click "Enable"

open "https://console.cloud.google.com/apis/credentials/consent"
# → External → Create
# → App name: go-narration-video
# → Email: your gmail
# → Save and Continue (skip scopes)
# → Test users: ADD USERS → your gmail → Save

open "https://console.cloud.google.com/apis/credentials"
# → Create Credentials → OAuth client ID
# → Type: Desktop app
# → Name: go-narration-cli
# → Create
# → Click the download icon (⬇) next to your new client
```

A file like `client_secret_XXX.apps.googleusercontent.com.json` lands in `~/Downloads/`.

## 2. Move the credentials file into place

```bash
mkdir -p ~/.config/go-narration-video
mv ~/Downloads/client_secret_*.json ~/.config/go-narration-video/credentials.json
chmod 600 ~/.config/go-narration-video/credentials.json
```

## 3. First publish

```bash
make publish
```

You'll see:
1. Metadata preview
2. `Upload this video to YouTube? [N/y]` — type `y`
3. Browser opens for OAuth consent
4. **Warning: "Google hasn't verified this app"** — click **Advanced → Go to go-narration-video (unsafe)** → **Continue** → **Allow**
5. Browser shows "Auth complete"
6. Terminal uploads the video and prints the URL

Token saved to `~/.config/go-narration-video/token.json`. Future runs: silent.

## Daily workflow

```bash
make use-fseam N=01-intro
SHORT=1 VOICE=onyx make audio
make render
make publish
```

## Troubleshooting

**"This app is blocked"** → You're not in the Test users list. Add your email at https://console.cloud.google.com/apis/credentials/consent under "Test users."

**"redirect_uri_mismatch"** → You created a "Web application" client instead of "Desktop app." Create a new Desktop app client.

**"Token expired"** → `rm ~/.config/go-narration-video/token.json` and run `make publish` again. Tokens in Testing-mode apps expire weekly.

**"Quota exceeded"** → 1,600 quota units per upload, 10,000/day default. ~6 uploads/day. Wait 24h or request increase.

**Lost credentials.json?** → Re-download from https://console.cloud.google.com/apis/credentials (click the download icon next to your client).
