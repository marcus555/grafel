# Downstream release workflow

This fork keeps upstream synchronization separate from local integration and
validated builds:

- `main` mirrors the upstream `main` branch.
- `feat/*` and `fix/*` isolate local changes.
- `dev` reconciles upstream changes with local work.
- `release` contains the validated state used to build local binaries.

## Version format

Local releases use:

```text
v<upstream-version>-local.<revision>
```

For example, the first local release based on upstream `v0.1.7.4` is
`v0.1.7.4-local.1`.

- Increment `revision` when publishing another local release from the same
  upstream base.
- Reset `revision` to `1` when the upstream base version changes.
- Tag the exact `release` commit with the same version embedded in the binary.

## Windows build

Run from the repository root after building and copying the dashboard bundle:

```powershell
$Version = "v0.1.7.4-local.1"
$Commit = git rev-parse --short HEAD
$Date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$Ldflags = "-s -w -X github.com/cajasmota/grafel/internal/version.Version=$Version -X github.com/cajasmota/grafel/internal/version.Commit=$Commit -X github.com/cajasmota/grafel/internal/version.Date=$Date"

go build -trimpath -tags osusergo -ldflags $Ldflags -o ".\dist\grafel-$Version-windows-amd64.exe" .\cmd\grafel
& ".\dist\grafel-$Version-windows-amd64.exe" version
```

Do not publish a release binary built without these linker values; otherwise
the UI reports `0.0.0-dev` and may show an unknown commit or build date.
