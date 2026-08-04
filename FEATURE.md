# Predator — Feature Roadmap

> Generated from deep codebase analysis. Prioritized by impact/effort ratio.

---

## High Impact — Medium Effort

| # | Feature | Description | Effort | Priority |
|---|---------|-------------|--------|----------|
| 1 | **Download Scheduling / Queue Persistence** | Persist queue to disk (`~/.predator/queue.json`); survive app restarts; schedule downloads for off-peak hours | Medium | *** |
| 2 | **Batch URL Import/Export** | Import from `.txt`, `.csv`, `.json`; export queue/history | Medium | *** |
| 3 | **Smart Filename Templates** | Custom naming: `{title} [{id}] ({resolution}).{ext}` with sanitization, length limits, date tokens | Medium | *** |
| 4 | **Built-in Media Preview** | Inline `<video>`/`<audio>` player in history tab | Medium | ** |
| 5 | **Per-Download Speed Limit** | Per-task or global bandwidth throttling via `aria2c --max-download-limit` | Low-Med | ** |

---

## High Impact — Lower Effort

| # | Feature | Description | Effort | Priority |
|---|---------|-------------|--------|----------|
| 6 | **Auto-Retry Failed Downloads** | Configurable retry with backoff; right-click → "Retry" in history | Low | *** |
| 7 | **Keyboard Shortcuts** | `Ctrl+V` paste→detect, `Enter` queue, `Ctrl+Shift+V` batch paste | Low | ** |
| 8 | **System Tray / Background Mode** | Minimize to tray; continue downloads when window closed | Low-Med | ** |
| 9 | **Theme Variants** | Add "System", "Light", "High Contrast" beyond current Dark | Low | * |
| 10 | **Download Categories/Tags** | User-defined tags per download ("Music", "Tutorials", "Archive") with filter UI | Low-Med | ** |

---

## Quality-of-Life / Polish

| # | Feature | Description | Effort |
|---|---------|-------------|--------|
| 11 | **Drag-and-Drop URLs** | Drop `.url` files, text files, or URLs directly onto window | Low |
| 12 | **Subtitle/Caption Download** | Option to download subtitles (`.srt`, `.vtt`); embed option | Medium |
| 13 | **SponsorBlock Integration** | Auto-remove sponsored segments via yt-dlp `--sponsorblock-remove` | Low |
| 14 | **Proxy / VPN Support** | Per-download or global proxy config (yt-dlp `--proxy`) | Low-Med |
| 15 | **Cookies/Authentication** | Import browser cookies for age-restricted/private content | Medium |
| 16 | **Duplicate Resolution Options** | "Skip", "Overwrite", "Rename (auto-number)", "Keep both" — not just confirm dialog | Low |
| 17 | **Download Statistics Dashboard** | Total downloaded, bandwidth used, success rate, by platform/resolution/format | Medium |
| 18 | **Auto-Update Checker** | Check GitHub releases; in-app notification | Low-Med |

---

## Architecture / Technical Debt

| Area | Issue | Recommendation |
|------|-------|----------------|
| **Monolithic `app.go`** | 1965 lines; all logic in one file | Split into packages: `downloader`, `queue`, `history`, `settings`, `platforms` |
| **Frontend single file** | `main.js` = 1156 lines | Modularize: `ui/`, `api/`, `components/`, `utils/` |
| **Hardcoded format strings** | Complex builders in `app.go:458-637` | Externalize to config or template system |
| **Zero tests** | No unit/integration tests | Add tests for helpers (`formatBytes`, `extractResolution`, URL validators) + integration with test URLs |
| **Error handling** | Many `log.Printf` without user-facing errors | Structured error types with friendly messages |
| **Settings schema** | Only 3 settings (theme, outputDir, autoUpdate) | Extendable schema with versioning |

---

## Suggested Implementation Phases

### Phase 1 — Foundation (1-2 weeks)
- [ ] Split `app.go` into packages
- [ ] Add unit tests for helpers
- [ ] Settings schema versioning
- [ ] Keyboard shortcuts (#7)

### Phase 2 — Core Features (2-3 weeks)
- [ ] Queue persistence (#1)
- [ ] Batch URL import/export (#2)
- [ ] Filename templates (#3)
- [ ] Per-download speed limit (#5)

### Phase 3 — Media Features (2-3 weeks)
- [ ] Subtitle download + embed (#12)
- [ ] SponsorBlock integration (#13)
- [ ] Media preview in history (#4)
- [ ] Proxy/cookies support (#14, #15)

### Phase 4 — Polish (1-2 weeks)
- [ ] System tray (#8)
- [ ] Statistics dashboard (#17)
- [ ] Auto-updater (#18)
- [ ] Theme variants (#9)

---

## Quick Wins (Can Implement This Session)

| Feature | Lines | User Value |
|---------|-------|------------|
| Queue Persistence | ~100 | High — survives restarts |
| Filename Templates | ~50 backend + UI | High — solves naming complaints |
| Batch URL Import | ~80 | High — power user favorite |

---

*Last updated: 2025-07-19 — Generated from codebase analysis*