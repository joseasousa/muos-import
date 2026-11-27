# Repository Guidelines

## Project Structure

- `main.go` contains the SaveSync CLI, ROM identification, version selection, backup storage, and recovery of interrupted writes.
- `main_test.go` exercises the sync engine with temporary directories representing the two firmwares and their shared SD2.
- `packaging/muos/` and `packaging/knulli/` contain the handheld launchers. `package.sh` assembles the Linux ARM64 package trees in `dist/`. Keep generated builds out of source changes.
- `README.md` documents installation and configuration in English.

## Build and Development

Use Go 1.23 or newer. The project uses the Go standard library.

- `go build .` builds the CLI for your machine.
- `go run . -config /path/to/config.json -action list` lists backups and conflicts for a configured test fixture. Use disposable data for local development.
- `go test ./...` runs the Go tests. `go vet ./...` checks the code for common mistakes.
- `./package.sh` cross-compiles a static Linux ARM64 binary and assembles both firmware layouts.

## Style and Testing

Run `gofmt -w` on edited Go files. Follow standard Go naming, keep helpers unexported where possible, and write handheld messages in English. Keep launcher scripts compatible with POSIX `sh`.

Name tests `TestBehavior` and use `t.TempDir()` for ROMs, saves, and shared-card fixtures. For sync or restore changes, cover both transfer directions and check that conflicts and interrupted writes preserve the active save and its backup. Test launcher and save-state compatibility separately on the RG40XX V. Record the firmware, RetroArch, and core builds used.

## Commits and Pull Requests

This checkout has no Git history to establish a commit convention. Use short, imperative subjects that describe the user-visible change. In pull requests, summarize the change, link any relevant issue, and list the checks you ran. Identify handheld checks that still need verification and include a screenshot for visible interface changes.
