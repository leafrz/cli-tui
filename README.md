# lofi.radio

![CI](https://github.com/leafrz/cli-tui/actions/workflows/ci.yml/badge.svg)

A modular terminal dashboard, built to learn Go — currently centered on an
internet radio player with a warm, lo-fi aesthetic. Built as a
[Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI.

## Download

Grab a prebuilt binary from the [Releases](https://github.com/leafrz/cli-tui/releases)
page (Windows / macOS), or build from source (below). Linux isn't shipped as a
prebuilt binary, but it builds fine from source (needs `libasound2-dev`).

The app boots into a small launcher ("what do you wanna do?") and routes into
modules. The architecture is set up so more can drop in.

## Features

- **Internet radio** via the [Radio Browser](https://www.radio-browser.info/) API
  - Search by name/genre, or browse top German stations
  - Inline ICY metadata (now-playing track), animated equalizer
  - Favorites (persisted), session restore (last volume + station), auto-reconnect
  - Mute, volume control, sleep timer (15/30/60 min)
- **Audio-reactive visualizer** — the player's spectrum reacts to the
  radio's live audio (real FFT, not faked); press `v` for full-screen
- **System monitor** — live CPU (overall + per-core), memory, disk, and
  network throughput with gauges and sparklines
- **Ambient** — a "leave it open" screen: 13 animated scenes (starfield,
  matrix, rain, snow, plasma, life, fireworks, dvd, waves, fire, ripples,
  spiral, blank) with optional auto-rotate, a big block clock, live
  **weather**, and a now-playing line + mini-spectrum when the radio is
  going. Jump here from the radio with `a` (music keeps playing), and the
  whole app auto-screensavers into it after a couple minutes idle.
- **Customizable header** — static text, rotating taglines, marquee, or
  context-aware (scrolls the now-playing track)
- **Settings page** — a dashboard module to configure theme, header, weather
  (incl. turning it off), and the screensaver, all live + persisted
- **Themes** — switchable color palettes (lofi / midnight / sepia / forest /
  rosepine / nord / noir), applied live and persisted
- Per-user state stored outside the repo (favorites, header text, etc.)

## Requirements

- **Go 1.24+**
- A working audio output device
- Platform notes for the audio backend (`oto`):
  - **Windows / macOS** — no extra dependencies
  - **Linux** — install ALSA dev headers, e.g. `sudo apt install libasound2-dev`

## Run

```sh
git clone https://github.com/leafrz/cli-tui
cd cli-tui
go run .
```

Or build a binary:

```sh
go build -o lofi-radio .
./lofi-radio
```

> Run it in a real terminal (a TTY). The integrated terminal in your editor is
> fine; a debug console / piped output is not (the alt-screen UI needs a TTY).

## Keybindings

### Dashboard (launcher)
| Key | Action |
|-----|--------|
| `↑` / `↓` | Select module |
| `enter` | Open module |
| `ctrl+t` | Cycle header mode (static → rotate → marquee → context) |
| `ctrl+e` | Edit header text |
| `ctrl+p` | Cycle theme (7 palettes: lofi, midnight, sepia, forest, rosepine, nord, noir) |
| `?` | Global help |
| `ctrl+c` | Quit |

### Radio module
| Context | Key | Action |
|---------|-----|--------|
| Search | `enter` | Search (empty = top DE) |
| Search | `ctrl+f` | Show favorites |
| Search | `ctrl+r` | Resume last station |
| Search | `esc` | Back to dashboard |
| List | `f` | Toggle favorite |
| List | `/` | Filter |
| List | `enter` | Play station |
| Player | `space` | Play / pause |
| Player | `+` / `-` | Volume |
| Player | `m` | Mute |
| Player | `v` | Full-screen visualizer |
| Player | `a` | Ambient mode (keeps playing) |
| Player | `t` | Sleep timer |
| Player | `f` | Favorite this station |
| Player | `esc` / `q` | Back to list |

### Ambient module
| Key | Action |
|-----|--------|
| `space` / `s` | Next scene |
| `S` | Previous scene |
| `r` | Auto-rotate scenes (every ~30s) |
| `c` | Toggle clock |
| `h` | 12 / 24-hour clock |
| `w` | Set weather location (type a city, or empty for auto) |
| `esc` | Back to dashboard |

The dashboard also **auto-drops into ambient after ~2 min of no input** (a
real screensaver); any key wakes it back to where you were. Ambient remembers
your last scene + clock settings.

Press `?` inside a module for module-specific help.

## Configuration & state

User state is stored as JSON in your OS config directory (never in the repo):

| OS | Path |
|----|------|
| Windows | `%AppData%\lofi-radio\state.json` |
| macOS | `~/Library/Application Support/lofi-radio/state.json` |
| Linux | `~/.config/lofi-radio/state.json` |

It holds favorites, last volume, last station, the header config (mode,
custom text, taglines), the selected theme, and the weather location. Delete
the file to reset to defaults. You can also edit `taglines` by hand for the
rotating/marquee header modes.

### Weather location

The ambient module shows live weather (via [Open-Meteo](https://open-meteo.com),
no API key). Configure it in the **settings** module (including turning it
**off**), press `w` in the ambient module, or edit the `weather` block in
`state.json`:

```json
"weather": { "mode": "auto", "city": "", "lat": 0, "lon": 0 }
```

- `mode: "auto"` — locate by public IP (one call to `ip-api.com`)
- `mode: "manual"` with a `city` — geocoded via Open-Meteo (no IP lookup)
- `mode: "manual"` with `lat`/`lon` — fixed coordinates, no lookup at all
- `mode: "off"` — no weather, no network calls

Weather data itself always comes from Open-Meteo over the network — there's no
offline weather source. Manual mode only removes the *location* lookup.

### Ambient / screensaver

The `ambient` block remembers your scene + clock prefs and configures the
auto-screensaver:

```json
"ambient": { "scene": "plasma", "hide_clock": false, "clock12": false,
             "rotate": false, "idle_off": false, "idle_secs": 120 }
```

- `idle_secs` — seconds of no input before auto-screensaver (default 120)
- `idle_off: true` — disable the auto-screensaver entirely

### Debugging

Set `RADIO_DEBUG=1` to write a `radio_debug.log` next to the binary (the TUI
itself never logs to stdout).

## Architecture

```
main.go          Thin entrypoint: audio init + run the root model
module.go        Module interface (Name / Init / Update / View / Status)
dashboard.go     Root model: launcher, global header, routing to modules
header.go        Header modes + marquee + config
radiomodule.go   The radio module (search / list / player)
sysmon.go        The system-monitor module (gopsutil)
ambient.go       The ambient module (compositor, clock, weather, now-playing)
scenes.go        The 13 ambient scenes (scene interface)
settings.go      The settings module (live config, reloadConfigMsg)
weather.go       IP geolocation + Open-Meteo (no API key)
radio/meter.go   Audio tap + FFT powering the visualizer
api.go           Radio Browser API + station type
store.go         Per-user state persistence (merge-safe writes)
favorites.go     Favorites helpers
styles.go        Palette + shared UI components (EQ, volume bar, help)
themes.go        Theme definitions + live palette switching
radio/           Audio engine: streaming, decode, ICY metadata, noise gate
```

A **module** implements the `Module` interface; the root dashboard renders the
global header and delegates the rest. Adding a new module is roughly:

```go
// in newRoot()'s entries:
{icon: "☀", name: "weather", desc: "current conditions", module: newWeatherModule()},
```

## Built with

[Bubble Tea](https://github.com/charmbracelet/bubbletea) ·
[Bubbles](https://github.com/charmbracelet/bubbles) ·
[Lip Gloss](https://github.com/charmbracelet/lipgloss) ·
[beep](https://github.com/faiface/beep) (audio)

## Known issues

- The radio visualizer can occasionally get "stuck" / stop reacting until you
  re-enter the player. Audio is unaffected.

## Roadmap

- [x] System monitor module
- [x] Audio-reactive visualizer (in the radio player, `v` for full-screen)
- [x] Ambient module (13 scenes + clock + weather + now-playing)
- [ ] More modules (it's a dashboard, after all)

---

A hobby project for learning Go. PRs and ideas welcome.
