# Contributing

## Submodules policy

This repository includes a `frontend/` submodule. To maintain reproducible builds and CI integrity:

- Never commit a dirty submodule state. Ensure `git submodule status` shows no `dirty` markers.
- Update submodules deterministically using:
  - `git submodule sync --recursive`
  - `git submodule update --init --recursive --checkout`
- When you change the `frontend/` code, commit and push inside the submodule first, then update the pointer in this repo:
  1. `cd frontend`
  2. Work and commit: `git add -A && git commit -m "feat: ..." && git push`
  3. `cd .. && git add frontend && git commit -m "chore(frontend): bump submodule" && git push`

CI will block merges that contain uninitialized, mismatched, or dirty submodule pointers (see `.github/workflows/submodule-check.yml`).

## Development quickstart

- Clone with submodules: `git clone --recurse-submodules https://github.com/Aswanidev-vs/Predator.git`
- Or initialize after clone: `git submodule update --init --recursive`
- Go toolchain: see `go.mod` (`go 1.25`). Build/dev commands:
  - Dev: `wails dev`
  - Build: `wails build`
