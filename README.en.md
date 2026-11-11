# go-navidrome-tui

[简体中文](./README.md) | **English**

[![Go](https://img.shields.io/github/go-mod/go-version/anccnuer/go-navidrome-tui?logo=go)](go.mod)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-4c566a)](#install-and-run)
[![mpv](https://img.shields.io/badge/audio-mpv-orange)](https://mpv.io/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](#license)
[![Stars](https://img.shields.io/github/stars/anccnuer/go-navidrome-tui?style=social)](https://github.com/anccnuer/go-navidrome-tui)

Listen to your Navidrome library from the terminal. The UI is modelled on [go-musicfox](https://github.com/go-musicfox/go-musicfox) and playback is handed to [mpv](https://mpv.io/) — mpv owns decoding and format support, this program only owns the interface and the queue, which keeps it light.

You need three things to run it: a Navidrome server you can reach, mpv installed, and a terminal with 24-bit colour (Windows Terminal, Kitty, iTerm2 and Alacritty all work; Windows CMD and the stock macOS Terminal do not).

## What it looks like

```
 » 主页 / 全部歌曲                navidrome-tui · [3/120]                 ● 已连接   home

                             ← 全部歌曲 · 128 首

                             =>   1. Hysteria           Muse
                                2. Absolution         Muse
                                3. Time Is Running Out  Muse
                                4. Plug In Baby       Muse
                                ...（仅渲染视口内可见行）

                             ↑↓/jk 导航  Enter 播放  b/Esc 返回  / 过滤

                             Hysteria
                             Muse · Absolution

 [顺序播放] 80% ♫ ♪ ♫ ♪ ♥ Hysteria  Muse
 ####################........................ 00:42/03:45
```

The program's own UI is Chinese today, so the mock-up above is the real output rather than a translation.

The status bar on top shows where you are, the connection state and the server name; the menu sits in the middle; the bottom two rows are the playback bar. Press `z` to switch to the full-screen now-playing page: artwork and title on the left, scrolling lyrics on the right.

## What it does

- Browsing has exactly two entry points: **All songs** and **Playlists**. Artists, albums and genres are deliberately left out.
- `Ctrl+F` searches the whole library, `/` filters the current list.
- Play, pause, previous/next track, seek, volume, four playback modes (sequential / repeat all / repeat one / shuffle), queue editing and play-next.
- Synced lyrics, half-block album art, like / add to playlist, open in browser, theme switching, help screen, configurable keybindings.
- `z` opens the full-screen now-playing page; `h`/`l` seek and `j`/`k` change the volume there.
- Last.fm scrobbling and Navidrome play-count reporting, both off by default; bring your own credentials.
- Mouse support: click input fields and list rows, double-click to play, click the progress bar to seek, scroll to page.

Narrow the window and the layout collapses to a single column. A missing mpv, a dead network or absent lyrics produce a hint or a placeholder instead of a blank screen.

## Install and run

### Install mpv first

- Windows: download it from [mpv.io](https://mpv.io/installation/), or `scoop install mpv` / `winget install mpv`. If you would rather not touch PATH, put the full path in `player.mpvPath`.
- Linux / macOS: `apt install mpv` / `brew install mpv`.

Use a monospaced font in your terminal; for Chinese text a monospaced CJK font keeps the columns aligned.

### Build

Go 1.26 or newer is required.

```bash
git clone https://github.com/anccnuer/go-navidrome-tui.git
cd go-navidrome-tui
make build     # Windows produces bin/ndtui.exe, other platforms bin/ndtui
make run       # run it directly
make test      # run the tests
```

No make? That works too (PowerShell):

```powershell
go build -o bin/ndtui.exe ./cmd/ndtui
.\bin\ndtui.exe
```

Linux / macOS:

```bash
go build -o bin/ndtui ./cmd/ndtui
./bin/ndtui
```

| Flag | Purpose |
| --- | --- |
| `--config <path>` | Use a specific config file (same as `NAVIDROME_TUI_CONFIG`) |
| `--debug` | Enable debug logging, written to the log file only, never to the screen |
| `--version` | Print the version |
| `--sixel-check` | Probe the terminal (DA1 / cell pixels) and print two Sixel reference images, then exit |
| `--sixel-dump <file>` | Write the self-check Sixel payload to a file, for comparing against `chafa -f sixel` |

## Try it out

There is a public [demo instance](https://demo.navidrome.org), account and password are both `demo`. Use it to try browsing, search and playback — please do not favourite things or edit playlists there.

| Item | Value |
| --- | --- |
| Address | `https://demo.navidrome.org` (`demo.navidrome.org` also works, https is added for you) |
| User / password | `demo` / `demo` |

If you would rather skip the wizard, use environment variables (PowerShell):

```powershell
$env:NAVIDROME_TUI_URL='https://demo.navidrome.org'
$env:NAVIDROME_TUI_USER='demo'
$env:NAVIDROME_TUI_PASSWORD='demo'
.\bin\ndtui.exe
```

That instance is rate-limited fairly often, so retry later if it refuses you; the wizard also retries once on timeout. The repository also ships a read-only integration test that is skipped by default:

```bash
NAVIDROME_TUI_DEMO_TEST=1 go test ./internal/subsonic -run TestPublicDemoReadOnly -v
```

## Configuration

The first start opens a wizard: name, address, user name, password. Pressing Enter pings the server before saving. Input fields accept mouse clicks, and the address can be pasted as-is (leading/trailing spaces and full-width characters are cleaned up, a missing scheme is added). `Ctrl+C` leaves the wizard; when you re-authenticate from the main screen with `w`, `Esc` cancels.

Config file locations:

| Platform | Path |
| --- | --- |
| Windows | `%AppData%\navidrome-tui\config.toml` |
| Linux | `$XDG_CONFIG_HOME/navidrome-tui/config.toml` or `~/.config/navidrome-tui/config.toml` |
| macOS | `~/Library/Application Support/navidrome-tui/config.toml` |

Upgrading from the old name (`subsonic-tui`) does not require logging in again: if the new directory has no config, the old one is still read, and the old `SUBSONIC_TUI_*` environment variables keep working — the new names take precedence.

For the full list of options see [`config.example.toml`](./config.example.toml) (currently in Chinese). The parts people actually touch:

- `[[servers]]` can be repeated; `active` selects the current one.
- Passwords are stored as `enc:<hex>`, which only keeps them out of plain sight — **it is not encryption**. If you would rather not write it to disk, omit `password` and use environment variables:

```bash
NAVIDROME_TUI_URL=https://music.example.com \
NAVIDROME_TUI_USER=alice \
NAVIDROME_TUI_PASSWORD=secret \
./bin/ndtui
```

Other environment variables: `NAVIDROME_TUI_CONFIG`, `NAVIDROME_TUI_THEME`, `NAVIDROME_TUI_MPV`, `NAVIDROME_TUI_LOG_LEVEL`, `NAVIDROME_TUI_SERVER_NAME`.

- Every key except the built-in navigation keys can be remapped in `[keybindings.app]`, see [`docs/shortcuts.md`](./docs/shortcuts.md) (currently in Chinese). `Ctrl+T` switches themes; built in are `musicfox` (default, NetEase red), `aurora`, `dracula`, `nord`, `gruvbox` and `mono`, and `theme.customPath` can point at your own TOML palette.
- `[lastfm]` is off by default; fill in `apiKey` / `apiSecret` / `sessionKey`. Tracks are scrobbled after more than half the duration or four minutes; `scrobbleAt = 0` means only after a track finishes.

## Keybindings

The built-in navigation keys are fixed, everything else can be changed in `[keybindings.app]`. While an input field is focused keys are text; `Esc` goes back one view on the search and help screens, and only closes the filter on the filter box; `q` or `Ctrl+C` quits at any time. `c` / `y` / `F2` / `z` toggle back when pressed again, and the now-playing page exits with `Esc` / `b` / `z`.

| Key | Action | Key | Action |
| --- | --- | --- | --- |
| `j` `k` / `↑` `↓` | Move up/down (same column in two-column mode: 1→3→5, 2→4→6) | `Space` | Play / pause |
| `h` `l` / `←` `→` | Move left/right (same row in two-column mode: 1↔2; go back / enter in one-column mode) | `[` `]` | Previous / next track |
| `g` `G` | Top / bottom | `-` `=` | Volume down / up |
| `Enter` `n` | Enter / play | `x` `X` `v` `V` | Seek back 1/5 s, forward 5/10 s |
| `b` `Esc` | Back | `p` | Cycle playback mode |
| `/` | Filter the current list | `Ctrl+F` | Search the whole library |
| `c` | Playback queue | `y` | Synced lyrics |
| `?` | Help | `F2` | Settings |
| `z` | Now playing, full screen (press again to go back) |  | Click the progress bar to seek |
| `e` `E` | Play next / append to the end | `\` | Remove from the queue |
| `,` `.` | Like / unlike (currently playing) | `<` `>` | Like / unlike (selected item) |
| `Tab` | Add to playlist | `O` | Open the selection in the browser |
| `f` | Similar songs of the playing track | `Ctrl+T` | Switch theme |
| `u` | Clear caches | `w` | Switch server |
| `q` | Quit | `r` | Redraw |

The mouse is enabled by default; if your terminal turns mouse events into garbage characters, set `enableMouse = false` in `[ui]` — everything is reachable from the keyboard (the progress bar is not clickable below 14 rows of terminal height).

## Why it stays fast

- Network requests, cover decoding and mpv commands all run in the background, so the UI never stalls.
- Lists only draw the rows visible on screen, and the next page is fetched when you reach the bottom.
- Songs, playlists, search results, lyrics and covers are cached; styles and gradients are precomputed.
- mpv reports through events and the UI refreshes every 500 ms, so the whole screen is not repainted per frame.

Logs go to the log file at `warn` level by default; tokens, salts and passwords in URLs are redacted.

## Cover art: half blocks or Sixel

By default the cover is drawn with half blocks: every character cell is `▀`, foreground colour for the top half pixel, background colour for the bottom one. No graphics protocol involved, so it works over SSH, in tmux and in 16-colour terminals.

If your terminal supports Sixel (Windows Terminal 1.22+, mlterm, foot, wezterm and others), verify it in this order **inside a real terminal**:

```bash
chafa -f sixel some-image.png     # step 1: prove the terminal. plain `chafa image.png` uses half blocks
ndtui --sixel-check               # step 2: prove the program
```

`--sixel-check` does three things:

1. **Real probing**: it sends DA1 (`CSI c`, attribute 4 = Sixel) and a cell-size query (`CSI 16 t`), reads the replies with a timeout, and prints the raw reply plus the measured cell size in pixels.
2. **Two reference images**: image 1 is a hand-written 12×6 magenta block (a known-good constant), image 2 is a 240×120 gradient produced by this program's encoder. Both visible = terminal and encoder are fine; only the magenta block = the encoder output is being rejected (export it with `ndtui --sixel-dump sixel.txt` and diff against chafa); neither = this window does not render Sixel.
3. **The version**: printed on the first line, so results from a stale binary cannot be mistaken for a new bug.

Once an image shows up, enable it in the config (`sixelCellWidth/Height` should match the cell pixels the self-check reported):

```toml
[ui]
coverProtocol = "sixel"
sixelCellWidth = 8      # pixels per character cell, tune to your font
sixelCellHeight = 16
sixelColors = 256       # palette size, 2-256
```

The palette is not a fixed 4×4×4 colour cube: it is built from the artwork itself with median cut (up to 256 colours) and then dithered with Floyd-Steinberg; the payload is run-length encoded per six-pixel band and trailing transparent cells are dropped. A 200×200 cover therefore costs roughly 30 KB, and skin tones and gradients are not crushed into a few patches.

Sixel cannot live inside a frame that gets repainted line by line (Bubble Tea would truncate, skip or erase it), so the program uses a side channel: after the text frame has been written, the image is sent separately at a screen position, and the now-playing page keeps a fixed blank area for it. Covers are cached and only re-sent on track change, resize, entering/leaving the page, or when a lyric scroll redraws the rows the artwork covers; the 500 ms progress-bar and status-bar refreshes never re-transmit it. Short control sequences (alt-screen switches, cursor visibility) do not consume the pending image early.

A few honest limitations:

- **The default is `text`**; `coverProtocol = "sixel"` is an explicit opt-in. Environment variables are only logged, they never silently fall back to `text`: with `TERM=dumb`, without `WT_SESSION`, over SSH or inside tmux the DCS is still sent as configured. Always run `--sixel-check` first; the only fallbacks are stdout redirected to a regular file and a failed encode.
- `WT_SESSION`, `TERM` and `COLORTERM` are **hints only**: shells such as nushell frequently do not forward `WT_SESSION`, IDE terminals (xterm.js) do not support Sixel at all, and `TERM` is usually empty in native Windows processes. Do not draw conclusions from them — use the DA1 probe and the two reference images from `--sixel-check`.
- The encoder sends `ESC P 0;1;0q`, setting `P2` to 1 (zero bits stay transparent) just like Chafa / libsixel. A bare `ESC P q` defaults to `P2=0`, so the colour passes within a pixel band erase each other with the background colour, which Windows Terminal shows as a few blank rows instead of an image.
- Windows Terminal 1.22+ moves the cursor below a finished Sixel image (xterm/foot/WezTerm leave it on the starting row). The program brackets the DCS with `ESC 7` / `ESC 8` to save and restore the cursor, otherwise Bubble Tea's relative cursor moves desync and overwrite the image.
- Bubble Tea repaints whole lines, so a lyric scroll also rewrites the rows the cover occupies and the image must be re-sent. To keep that erase-then-redraw invisible, the text frame and the DCS payload leave in a single write wrapped in DECSET 2026 (synchronized output); Windows Terminal supports it since 1.23.20211.0, terminals that do not know the mode ignore it and behave exactly as before.
- Sixel has no "delete one image" command. Leaving the now-playing page relies on a clear-and-replay; parts wiped by a lyric redraw are re-sent automatically, and if a particular terminal still shows gaps, press `r` to force a redraw or set `coverProtocol` back to `text`.
- Wrong proportions? Tune `sixelCellWidth` / `sixelCellHeight`.

## FAQ

| Symptom | What to do |
| --- | --- |
| The bottom bar says mpv was not found | Install mpv and put it on PATH, or set `player.mpvPath`. Browsing still works, only playback does not. |
| Sound keeps playing after exit | Recent versions terminate mpv with the program (Job Object on Windows, Pdeathsig on Linux, SIGHUP on macOS). Leftovers are usually from an older version; kill it once. |
| Cannot connect / 401 | Check the address for `http(s)://`, the port, the user name and password; for a self-signed certificate set `insecure = true`. |
| Connection times out | The wizard retries once; the demo instance is often loaded, so wait a bit or use your own Navidrome. |
| Mouse clicks do nothing | Make sure `[ui] enableMouse` is not `false` (an old config may set it) and that your terminal reports mouse events. |
| Lyrics stay empty | Navidrome needs embedded lyrics or a sibling `.lrc` file; the program prefers OpenSubsonic `getLyricsBySongId` and falls back to `getLyrics`. |
| The cover is a pile of colour blocks | That is the half-block renderer doing its job. Turn on `ui.trueColor = true` for true colour; with it off you get 256 colours. For a real picture try Sixel (previous section). |
| I enabled Sixel and still get blocks | First prove the terminal with `chafa -f sixel image.png` (plain chafa uses half blocks, an image there proves nothing), then run `ndtui --sixel-check` and look at the two reference images. Set `sixelCellWidth/Height` to the cell pixels the self-check reported. |
| The self-check shows only the magenta block, not the gradient | The terminal supports Sixel but rejects this program's payload. Export it with `ndtui --sixel-dump sixel.txt` and compare against `chafa -f sixel` output before filing a report. |
| Neither reference image appears | This window does not render Sixel: conhost, IDE terminals such as VS Code / CodeBuddy (xterm.js) and SSH/tmux sessions all fall into this bucket. Try a Windows Terminal 1.22+ tab. |
| The layout is crooked | Use a monospaced font (CJK included); on wide screens set `ui.doubleColumn = false` for a single column. |
| Keys do nothing | Check whether an input field is focused and press `Esc`; `F2` shows keybinding conflicts. |
| `q` does not quit the help screen | The help screen starts in filter mode: press `Esc` to go back, or `Ctrl+C` to quit. |
| Every key suddenly stops working | Try `q` / `Ctrl+C` first. If that quits, it was just the current screen's key semantics (`h/l/j/k` seek and change the volume on the now-playing page, and a hint says so). If nothing works the terminal is not delivering keys — try `[ui] enableMouse = false` or another terminal. |

## Development

```bash
make test     # go test ./...
make lint     # go vet ./...
make tidy     # go mod tidy
make windows linux darwin   # cross-compile; needs Git Bash / MSYS / WSL on Windows
```

Releases are not built on every push. Tagging a version with `v*` triggers `.github/workflows/release.yml`: it runs the tests, cross-compiles five targets for Windows / Linux / macOS, packages them with `SHA256SUMS` and publishes a GitHub Release.

```bash
git tag v1.0.0
git push origin v1.0.0
```

You can also run the Release workflow manually from the Actions page and pass an existing tag to re-publish.

Layout:

```
cmd/ndtui/               entry point
internal/app/            dependency wiring and graceful shutdown
internal/config/         TOML config, paths, atomic save
internal/subsonic/       (Open)Subsonic REST client and DTOs
internal/model/          domain model and enums
internal/cache/          generic TTL + LRU
internal/player/         mpv JSON IPC and playback queue state machine
internal/lyrics/         LRC / structured lyrics parsing
internal/cover/          image decoding and half-block rendering
internal/keymap/         keybinding parsing, conflict detection, help data
internal/theme/          themes and the Lip Gloss style factory
internal/scrobbler/      Navidrome / Last.fm play reporting
internal/ui/             root Model, messages, routing, wizard, views
internal/ui/components/  pure rendering components
```

## License

MIT
