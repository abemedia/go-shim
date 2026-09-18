# go-shim

[![Go Reference](https://pkg.go.dev/badge/github.com/abemedia/go-shim.svg)](https://pkg.go.dev/github.com/abemedia/go-shim)

Distribute native binaries built in any language through `go install`.

A shim is a `main` package committed to a non-Go project's repository. On its first run it downloads
the release asset matching its module version and platform, replaces its own executable with the
release binary, and runs it.

## Usage

Commit a `main.go` to the root of your release repository.

```go
package main

import "github.com/abemedia/go-shim"

func main() {
	shim.Main(shim.Config{
		URL:     "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}",
		Targets: shim.Rust,
	})
}
```

Users then run:

```sh
go install github.com/owner/tool@latest
```

The shim downloads the release for the installed version from the module's repository. As with
`go get`, vanity import paths resolve to the repository they point to. For example, installing
`github.com/owner/tool@v0.1.4` downloads from
`https://github.com/owner/tool/releases/download/v0.1.4/`.

## Configuration

Only `URL` is required. `URL` and `Binary` are `text/template` templates rendered with these fields:

| Field        | Description                                                                |
| ------------ | -------------------------------------------------------------------------- |
| `.Name`      | the executable name, `tool`                                                |
| `.Repo`      | the repository URL, `https://github.com/owner/tool`                        |
| `.Tag`       | the release tag, `v0.1.4`                                                  |
| `.Version`   | the tag without its leading `v`, `0.1.4`                                   |
| `.Target`    | a name from the matching target, `x86_64-apple-darwin`                     |
| `.Ext`       | `.zip` on Windows, `.tar.gz` elsewhere                                     |
| `.Exe`       | `.exe` on Windows, empty elsewhere                                         |
| `.GOOS`      | `runtime.GOOS`, `darwin`                                                   |
| `.GOARCH`    | `runtime.GOARCH`, `amd64`                                                  |
| `.GO386`     | the 386 floating-point implementation, `sse2` or `softfloat`               |
| `.GOAMD64`   | the amd64 microarchitecture level, `v1` to `v4`                            |
| `.GOARM`     | the ARM architecture version, `5`, `6` or `7`, with an optional float ABI  |
| `.GOARM64`   | the ARM64 architecture version, `v8.0` to `v9.5`, with optional extensions |
| `.GOMIPS`    | the MIPS floating-point implementation, `hardfloat` or `softfloat`         |
| `.GOMIPS64`  | as `.GOMIPS`, for mips64 and mips64le                                      |
| `.GOPPC64`   | the ppc64 ISA level, `power8`, `power9` or `power10`                       |
| `.GORISCV64` | the RISC-V profile, `rva20u64`, `rva22u64` or `rva23u64`                   |
| `.Libc`      | the C library on Linux, `gnu` or `musl`, empty elsewhere                   |

`.GO386` through `.GORISCV64` are the settings the shim was built with and are empty on
architectures they do not apply to.

### URL

`URL` is the release asset's URL. A bare file name is resolved against the repository's release
download URL, which works for GitHub, GitLab, Bitbucket and forges that share GitHub's URL layout,
such as Gitea and Codeberg. For any other host, give an absolute https URL:

```go
URL: "https://downloads.example.com/{{.Tag}}/{{.Name}}-{{.Target}}{{.Ext}}"
```

If your archive formats differ from `.Ext`, select the extension by `.GOOS`:

```go
URL: `{{.Name}}-{{.Version}}-{{.Target}}{{if eq .GOOS "windows"}}.zip{{else}}.tar.bz2{{end}}`
```

### Targets

`Targets` lists the builds a project publishes and the platform each requires. The shim skips
entries the host cannot run and uses the first remaining name, in order, that has a published asset:

```go
Targets: []shim.Target{
	{GOOS: "linux", GOARCH: "amd64", Names: []string{"x86_64-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"x86_64-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Names: []string{"armv7-unknown-linux-musleabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "6", Names: []string{"arm-unknown-linux-musleabihf"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"aarch64-apple-darwin"}},
}
```

`Libc` is the C library a dynamically linked Linux build needs, `gnu` or `musl`, and stays empty for
static builds. `GOOS` and `GOARCH` must match the host, and the other `GO*` fields, which mirror Go's
environment variables, set the minimum the build requires. An empty field matches any host.

Presets cover common target naming schemes: `Rust`, `Deno`, `Zig`, `Node`, `Bun`, `DotNet` and
`Dart`. On Linux they prefer static builds and otherwise match the host's C library, and on Windows
they list MSVC builds before GNU.

If `Targets` is nil, `.Target` is empty, so name assets by `.GOOS` and `.GOARCH` instead:

```go
URL: "{{.Name}}-{{.Version}}-{{.GOOS}}-{{.GOARCH}}{{.Ext}}"
```

### Binary

`Binary` is the executable's path within an archive. If it is unset, the shim looks for
`{{.Name}}{{.Exe}}` in the directory layouts that common release tooling produces, such as
`{{.Name}}-{{.Version}}-{{.Target}}/`, and then at the archive root. Set it for any other layout:

```go
Binary: "{{.Name}}-{{.Version}}-{{.Target}}/bin/{{.Name}}{{.Exe}}"
```

### Format

By default the format is inferred from the asset's file extension. The supported formats are
`.tar.gz`, `.tgz`, `.tar.bz2`, `.tbz2`, `.tbz`, `.tar`, `.zip`, `.gz` and `.bz2`, as well as
uncompressed executables with no extension or an `.exe` extension. Set `Format` to
`shim.FormatBinary` for an uncompressed executable whose name contains a dot, such as a version
number.

### Version

`Version` is the release tag to download and defaults to the installed version. To test a shim
locally against a specific release, set it from a variable injected with `-ldflags -X`:

```go
var version string

func main() {
	shim.Main(shim.Config{
		URL:     "{{.Name}}-{{.Version}}-{{.Target}}{{.Ext}}",
		Targets: shim.Rust,
		Version: version,
	})
}
```

```sh
go run -ldflags "-X main.version=v0.1.4" . --version
```

## Major versions

Releases tagged `v2.0.0` and above can be installed with or without a `go.mod`. Without one, the
install command stays the same, and the go-shim dependency is unpinned, so it resolves to its latest
version at install time. With one, semantic import versioning requires the module path to end in
`/v2`, so the install command becomes `go install github.com/owner/tool/v2@latest`. Add the `go.mod`
before tagging `v2.0.0`: if it is added later without the `/v2` suffix, tags from `v2.0.0` onwards
cannot be installed until the module path is updated.
