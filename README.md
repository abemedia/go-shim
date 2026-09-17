# go-shim

[![Go Reference](https://pkg.go.dev/badge/github.com/abemedia/go-shim.svg)](https://pkg.go.dev/github.com/abemedia/go-shim)

Distribute binaries from any compiled language via `go install`.

A shim is a small Go program committed to your release repository. Installed with `go install`, it
downloads the release asset for its version and platform, caches it, and hands over to it.

## Usage

Commit a `main.go` to the root of your release repository.

```go
package main

import "github.com/abemedia/go-shim"

func main() {
	shim.Run(shim.Config{
		URL:     "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}",
		Targets: shim.Rust,
	})
}
```

Users then run:

```sh
go install github.com/owner/tool@latest
```

Everything else is derived. The repository comes from the module path, so a release tagged `v0.1.4`
is fetched from `https://github.com/owner/tool/releases/download/v0.1.4/`, and the command is named
after the last element of the module path. Binaries are cached under `os.UserCacheDir`, by command
and version.

## Configuration

Only `URL` is required. `URL` and `Binary` are `text/template` templates over these fields:

| Field        | Description                                                        |
| ------------ | ------------------------------------------------------------------ |
| `.Name`      | the command, `tool`                                                |
| `.Repo`      | the repository, `https://github.com/owner/tool`                    |
| `.Tag`       | the release, `v0.1.4`                                              |
| `.Version`   | the release without its leading `v`, `0.1.4`                       |
| `.Target`    | one name from `Targets`, `x86_64-apple-darwin`                     |
| `.Ext`       | `.zip` on Windows, `.tar.gz` elsewhere                             |
| `.Exe`       | `.exe` on Windows, empty elsewhere                                 |
| `.GOOS`      | the operating system, `darwin`                                     |
| `.GOARCH`    | the architecture, `amd64`                                          |
| `.GO386`     | the 386 floating point implementation, `sse2` or `softfloat`       |
| `.GOAMD64`   | the amd64 microarchitecture level, `v1` to `v4`                    |
| `.GOARM`     | the ARM architecture, `5`, `6` or `7`, with an optional float ABI  |
| `.GOARM64`   | the ARM64 architecture, `v8.0` to `v9.5`, with optional extensions |
| `.GOMIPS`    | the mips floating point implementation, `hardfloat` or `softfloat` |
| `.GOMIPS64`  | as `.GOMIPS`, for mips64 and mips64le                              |
| `.GOPPC64`   | the ppc64 instruction set, `power8`, `power9` or `power10`         |
| `.GORISCV64` | the RISC-V profile, `rva20u64`, `rva22u64` or `rva23u64`           |

Each `GO*` field carries the setting the shim was built with, and is empty on a platform that does
not use it.

### URL

The release asset. A bare file name is resolved against the release page of the module path, which
is known for GitHub, GitLab, Bitbucket and forges with GitHub's layout such as Gitea and Codeberg.
Anywhere else, give the whole https address:

```go
URL: "https://downloads.example.com/{{.Tag}}/{{.Name}}-{{.Target}}{{.Ext}}"
```

An extension pair that `.Ext` does not cover can be selected with a conditional:

```go
URL: `{{.Name}}-{{.Version}}-{{.Target}}{{if eq .GOOS "windows"}}.zip{{else}}.tar.bz2{{end}}`
```

### Targets

The builds a project publishes and what each needs to run. The shim keeps the entries its host can
use, in the order given, and tries each of their names in turn:

```go
Targets: []shim.Target{
	{GOOS: "linux", GOARCH: "amd64", Names: []string{"x86_64-unknown-linux-musl", "x86_64-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Names: []string{"armv7-unknown-linux-musleabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "6", Names: []string{"arm-unknown-linux-musleabihf"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"aarch64-apple-darwin"}},
}
```

The other fields are Go environment variables such as `GOARM` or `GOAMD64` and give the least the
build needs; a host at or above it matches. An entry with neither `GOOS` nor `GOARCH` matches every
host, so it can go last as a fallback.

Presets ship for the common naming schemes: `Rust`, `Deno`, `Zig`, `Node`, `Bun`, `DotNet` and
`Dart`. Each is a plain slice, so copy and edit one if your release matrix differs. Presets try musl
before glibc and msvc before gnu.

With `Targets` unset `{{.Target}}` renders as nothing, so name the asset with `.GOOS` and `.GOARCH`
instead:

```go
URL: "{{.Name}}-{{.Version}}-{{.GOOS}}-{{.GOARCH}}{{.Ext}}"
```

On `darwin/amd64` that is `tool-1.2.3-darwin-amd64.tar.gz`.

### Binary

The path to the binary inside an archive. Unset, it looks for `{{.Name}}{{.Exe}}` under the
directory names release tooling commonly produces, such as `{{.Name}}-{{.Version}}-{{.Target}}`, and
then at the archive root. Set it when an archive puts the binary somewhere else:

```go
Binary: "{{.Name}}-{{.Version}}-{{.Target}}/bin/{{.Name}}{{.Exe}}"
```

### Format

The format comes from the asset name: `.tar.gz`, `.tgz`, `.tar.bz2`, `.tbz2`, `.tbz`, `.tar`,
`.zip`, `.gz` and `.bz2` are supported, as is a raw binary with no extension or `.exe`. Set `Format`
to `shim.FormatBinary` if you publish raw binaries whose names contain a dot, such as a version
number, since go-shim will treat it like an unsupported file extension.

### Version

The release to download. It defaults to the version `go install` stamped in, so a shim built with
`go build` has nothing to download. Wire a variable to it and set that with `-ldflags` to exercise a
shim locally:

```go
var version string // -ldflags "-X main.version=v0.1.4"

func main() {
	shim.Run(shim.Config{
		URL:     "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}",
		Targets: shim.Rust,
		Version: version,
	})
}
```

```sh
go run -ldflags "-X main.version=v0.1.4" . --version
```

A set `Version` takes precedence over the stamped one, so leave the variable empty in releases.

## Major versions

A release tagged `v2.0.0` or higher installs with or without a `go.mod`. Without one, the install
command never changes, and go-shim is resolved at its latest release on every install. With one, Go
requires the module path to end in `/v2`, so the command becomes
`go install github.com/owner/tool/v2@latest`. Add that `go.mod` before tagging: one added later
makes tags from `v2` up unreachable until the path gains its suffix.
