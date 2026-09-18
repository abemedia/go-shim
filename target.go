package shim

// A Target describes a published build: its asset names and the platform it requires.
//
// The GO fields mirror the Go environment variables of the same name and accept the same values,
// interpreted as Go interprets them: GOARM "7" means "7,hardfloat" and "5" means "5,softfloat".
// GOOS and GOARCH must match the host, and the other GO fields set the minimum the build requires.
// Any field other than Names matches every host when left empty.
type Target struct {
	GOOS      string
	GOARCH    string
	GO386     string
	GOAMD64   string
	GOARM     string
	GOARM64   string
	GOMIPS    string
	GOMIPS64  string
	GOPPC64   string
	GORISCV64 string

	// Libc is the C library a dynamically linked Linux build requires: "gnu" or "musl".
	Libc string

	// Names are the .Target values to try for this build, in order.
	Names []string
}

// Rust names builds by Rust target triple.
var Rust = []Target{
	{GOOS: "android", GOARCH: "386", GO386: "sse2", Names: []string{"i686-linux-android"}},
	{GOOS: "android", GOARCH: "amd64", Names: []string{"x86_64-linux-android"}},
	{GOOS: "android", GOARCH: "arm", GOARM: "7", Names: []string{"armv7-linux-androideabi"}},
	{GOOS: "android", GOARCH: "arm", GOARM: "5", Names: []string{"arm-linux-androideabi"}},
	{GOOS: "android", GOARCH: "arm64", Names: []string{"aarch64-linux-android"}},
	{GOOS: "darwin", GOARCH: "amd64", Names: []string{"x86_64-apple-darwin"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"aarch64-apple-darwin"}},
	{GOOS: "dragonfly", GOARCH: "amd64", Names: []string{"x86_64-unknown-dragonfly"}},
	{GOOS: "freebsd", GOARCH: "386", GO386: "sse2", Names: []string{"i686-unknown-freebsd"}},
	{GOOS: "freebsd", GOARCH: "amd64", Names: []string{"x86_64-unknown-freebsd"}},
	{GOOS: "freebsd", GOARCH: "arm", GOARM: "7", Names: []string{"armv7-unknown-freebsd"}},
	{GOOS: "freebsd", GOARCH: "arm", GOARM: "6", Names: []string{"armv6-unknown-freebsd"}},
	{GOOS: "freebsd", GOARCH: "arm64", Names: []string{"aarch64-unknown-freebsd"}},
	{GOOS: "illumos", GOARCH: "amd64", Names: []string{"x86_64-unknown-illumos"}},
	{GOOS: "linux", GOARCH: "386", GO386: "sse2", Names: []string{"i686-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "386", GO386: "sse2", Libc: "gnu", Names: []string{"i686-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "386", Names: []string{"i586-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "386", Libc: "gnu", Names: []string{"i586-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "amd64", Names: []string{"x86_64-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"x86_64-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Names: []string{"armv7-unknown-linux-musleabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Libc: "gnu", Names: []string{"armv7-unknown-linux-gnueabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "6", Names: []string{"arm-unknown-linux-musleabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "6", Libc: "gnu", Names: []string{"arm-unknown-linux-gnueabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7,softfloat", Names: []string{"armv7-unknown-linux-musleabi"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7,softfloat", Libc: "gnu", Names: []string{"armv7-unknown-linux-gnueabi"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "6,softfloat", Names: []string{"arm-unknown-linux-musleabi"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "6,softfloat", Libc: "gnu", Names: []string{"arm-unknown-linux-gnueabi"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "5", Names: []string{"armv5te-unknown-linux-musleabi"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "5", Libc: "gnu", Names: []string{"armv5te-unknown-linux-gnueabi"}},
	{GOOS: "linux", GOARCH: "arm64", Names: []string{"aarch64-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "gnu", Names: []string{"aarch64-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "loong64", Names: []string{"loongarch64-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "loong64", Libc: "gnu", Names: []string{"loongarch64-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "mips", Names: []string{"mips-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "mips", GOMIPS: "hardfloat", Libc: "gnu", Names: []string{"mips-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "mips64", GOMIPS64: "hardfloat", Names: []string{"mips64-unknown-linux-muslabi64"}},
	{GOOS: "linux", GOARCH: "mips64", GOMIPS64: "hardfloat", Libc: "gnu", Names: []string{"mips64-unknown-linux-gnuabi64"}},
	{GOOS: "linux", GOARCH: "mips64le", GOMIPS64: "hardfloat", Names: []string{"mips64el-unknown-linux-muslabi64"}},
	{GOOS: "linux", GOARCH: "mips64le", GOMIPS64: "hardfloat", Libc: "gnu", Names: []string{"mips64el-unknown-linux-gnuabi64"}},
	{GOOS: "linux", GOARCH: "mipsle", Names: []string{"mipsel-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "mipsle", GOMIPS: "hardfloat", Libc: "gnu", Names: []string{"mipsel-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "ppc64", Names: []string{"powerpc64-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "ppc64", Libc: "gnu", Names: []string{"powerpc64-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "ppc64le", Names: []string{"powerpc64le-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "ppc64le", Libc: "gnu", Names: []string{"powerpc64le-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "riscv64", Names: []string{"riscv64gc-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "riscv64", Libc: "gnu", Names: []string{"riscv64gc-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "s390x", Names: []string{"s390x-unknown-linux-musl"}},
	{GOOS: "linux", GOARCH: "s390x", Libc: "gnu", Names: []string{"s390x-unknown-linux-gnu"}},
	{GOOS: "netbsd", GOARCH: "386", GO386: "sse2", Names: []string{"i686-unknown-netbsd"}},
	{GOOS: "netbsd", GOARCH: "amd64", Names: []string{"x86_64-unknown-netbsd"}},
	{GOOS: "netbsd", GOARCH: "arm", GOARM: "7", Names: []string{"armv7-unknown-netbsd-eabihf"}},
	{GOOS: "netbsd", GOARCH: "arm", GOARM: "6", Names: []string{"armv6-unknown-netbsd-eabihf"}},
	{GOOS: "netbsd", GOARCH: "arm64", Names: []string{"aarch64-unknown-netbsd"}},
	{GOOS: "openbsd", GOARCH: "386", GO386: "sse2", Names: []string{"i686-unknown-openbsd"}},
	{GOOS: "openbsd", GOARCH: "amd64", Names: []string{"x86_64-unknown-openbsd"}},
	{GOOS: "openbsd", GOARCH: "arm64", Names: []string{"aarch64-unknown-openbsd"}},
	{GOOS: "openbsd", GOARCH: "ppc64", Names: []string{"powerpc64-unknown-openbsd"}},
	{GOOS: "openbsd", GOARCH: "riscv64", Names: []string{"riscv64gc-unknown-openbsd"}},
	{GOOS: "solaris", GOARCH: "amd64", Names: []string{"x86_64-pc-solaris"}},
	{GOOS: "windows", GOARCH: "386", GO386: "sse2", Names: []string{"i686-pc-windows-msvc", "i686-pc-windows-gnu", "i686-pc-windows-gnullvm"}},
	{GOOS: "windows", GOARCH: "amd64", Names: []string{"x86_64-pc-windows-msvc", "x86_64-pc-windows-gnu", "x86_64-pc-windows-gnullvm"}},
	{GOOS: "windows", GOARCH: "arm64", Names: []string{"aarch64-pc-windows-msvc", "aarch64-pc-windows-gnullvm"}},
}

// Zig names builds by Zig target triple, both with and without a libc or "none" suffix.
var Zig = []Target{
	{GOOS: "android", GOARCH: "386", GO386: "sse2", Names: []string{"x86-linux-android"}},
	{GOOS: "android", GOARCH: "amd64", Names: []string{"x86_64-linux-android"}},
	{GOOS: "android", GOARCH: "arm", GOARM: "7", Names: []string{"arm-linux-androideabi"}},
	{GOOS: "android", GOARCH: "arm64", Names: []string{"aarch64-linux-android"}},
	{GOOS: "darwin", GOARCH: "amd64", Names: []string{"x86_64-macos", "x86_64-macos-none"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"aarch64-macos", "aarch64-macos-none"}},
	{GOOS: "dragonfly", GOARCH: "amd64", Names: []string{"x86_64-dragonfly", "x86_64-dragonfly-none"}},
	{GOOS: "freebsd", GOARCH: "386", GO386: "sse2", Names: []string{"x86-freebsd", "x86-freebsd-none"}},
	{GOOS: "freebsd", GOARCH: "amd64", Names: []string{"x86_64-freebsd", "x86_64-freebsd-none"}},
	{GOOS: "freebsd", GOARCH: "arm", GOARM: "7", Names: []string{"arm-freebsd-eabihf", "arm-freebsd"}},
	{GOOS: "freebsd", GOARCH: "arm64", Names: []string{"aarch64-freebsd", "aarch64-freebsd-none"}},
	{GOOS: "illumos", GOARCH: "amd64", Names: []string{"x86_64-illumos", "x86_64-illumos-none"}},
	{GOOS: "linux", GOARCH: "386", GO386: "sse2", Names: []string{"x86-linux-musl", "x86-linux"}},
	{GOOS: "linux", GOARCH: "386", GO386: "sse2", Libc: "gnu", Names: []string{"x86-linux-gnu"}},
	{GOOS: "linux", GOARCH: "amd64", Names: []string{"x86_64-linux-musl", "x86_64-linux"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"x86_64-linux-gnu"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Names: []string{"arm-linux-musleabihf", "arm-linux", "arm-linux-musleabi"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Libc: "gnu", Names: []string{"arm-linux-gnueabihf", "arm-linux-gnueabi"}},
	{GOOS: "linux", GOARCH: "arm64", Names: []string{"aarch64-linux-musl", "aarch64-linux"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "gnu", Names: []string{"aarch64-linux-gnu"}},
	{GOOS: "linux", GOARCH: "loong64", Names: []string{"loongarch64-linux-musl", "loongarch64-linux"}},
	{GOOS: "linux", GOARCH: "loong64", Libc: "gnu", Names: []string{"loongarch64-linux-gnu"}},
	{GOOS: "linux", GOARCH: "mips", GOMIPS: "hardfloat", Names: []string{"mips-linux-musleabihf", "mips-linux"}},
	{GOOS: "linux", GOARCH: "mips", GOMIPS: "hardfloat", Libc: "gnu", Names: []string{"mips-linux-gnueabihf"}},
	{GOOS: "linux", GOARCH: "mips", Names: []string{"mips-linux-musleabi"}},
	{GOOS: "linux", GOARCH: "mips", Libc: "gnu", Names: []string{"mips-linux-gnueabi"}},
	{GOOS: "linux", GOARCH: "mips64", GOMIPS64: "hardfloat", Names: []string{"mips64-linux-muslabi64", "mips64-linux"}},
	{GOOS: "linux", GOARCH: "mips64", GOMIPS64: "hardfloat", Libc: "gnu", Names: []string{"mips64-linux-gnuabi64"}},
	{GOOS: "linux", GOARCH: "mips64le", GOMIPS64: "hardfloat", Names: []string{"mips64el-linux-muslabi64", "mips64el-linux"}},
	{GOOS: "linux", GOARCH: "mips64le", GOMIPS64: "hardfloat", Libc: "gnu", Names: []string{"mips64el-linux-gnuabi64"}},
	{GOOS: "linux", GOARCH: "mipsle", GOMIPS: "hardfloat", Names: []string{"mipsel-linux-musleabihf", "mipsel-linux"}},
	{GOOS: "linux", GOARCH: "mipsle", GOMIPS: "hardfloat", Libc: "gnu", Names: []string{"mipsel-linux-gnueabihf"}},
	{GOOS: "linux", GOARCH: "mipsle", Names: []string{"mipsel-linux-musleabi"}},
	{GOOS: "linux", GOARCH: "mipsle", Libc: "gnu", Names: []string{"mipsel-linux-gnueabi"}},
	{GOOS: "linux", GOARCH: "ppc64", Names: []string{"powerpc64-linux-musl", "powerpc64-linux"}},
	{GOOS: "linux", GOARCH: "ppc64", Libc: "gnu", Names: []string{"powerpc64-linux-gnu"}},
	{GOOS: "linux", GOARCH: "ppc64le", Names: []string{"powerpc64le-linux-musl", "powerpc64le-linux"}},
	{GOOS: "linux", GOARCH: "ppc64le", Libc: "gnu", Names: []string{"powerpc64le-linux-gnu"}},
	{GOOS: "linux", GOARCH: "riscv64", Names: []string{"riscv64-linux-musl", "riscv64-linux"}},
	{GOOS: "linux", GOARCH: "riscv64", Libc: "gnu", Names: []string{"riscv64-linux-gnu"}},
	{GOOS: "linux", GOARCH: "s390x", Names: []string{"s390x-linux-musl", "s390x-linux"}},
	{GOOS: "linux", GOARCH: "s390x", Libc: "gnu", Names: []string{"s390x-linux-gnu"}},
	{GOOS: "netbsd", GOARCH: "386", GO386: "sse2", Names: []string{"x86-netbsd", "x86-netbsd-none"}},
	{GOOS: "netbsd", GOARCH: "amd64", Names: []string{"x86_64-netbsd", "x86_64-netbsd-none"}},
	{GOOS: "netbsd", GOARCH: "arm", GOARM: "7", Names: []string{"arm-netbsd-eabihf", "arm-netbsd", "arm-netbsd-eabi"}},
	{GOOS: "netbsd", GOARCH: "arm64", Names: []string{"aarch64-netbsd", "aarch64-netbsd-none"}},
	{GOOS: "openbsd", GOARCH: "386", GO386: "sse2", Names: []string{"x86-openbsd", "x86-openbsd-none"}},
	{GOOS: "openbsd", GOARCH: "amd64", Names: []string{"x86_64-openbsd", "x86_64-openbsd-none"}},
	{GOOS: "openbsd", GOARCH: "arm", GOARM: "7", Names: []string{"arm-openbsd-eabi", "arm-openbsd"}},
	{GOOS: "openbsd", GOARCH: "arm64", Names: []string{"aarch64-openbsd", "aarch64-openbsd-none"}},
	{GOOS: "openbsd", GOARCH: "ppc64", Names: []string{"powerpc64-openbsd", "powerpc64-openbsd-none"}},
	{GOOS: "openbsd", GOARCH: "riscv64", Names: []string{"riscv64-openbsd", "riscv64-openbsd-none"}},
	{GOOS: "windows", GOARCH: "386", GO386: "sse2", Names: []string{"x86-windows-gnu", "x86-windows", "x86-windows-msvc"}},
	{GOOS: "windows", GOARCH: "amd64", Names: []string{"x86_64-windows-gnu", "x86_64-windows", "x86_64-windows-msvc"}},
	{GOOS: "windows", GOARCH: "arm64", Names: []string{"aarch64-windows-gnu", "aarch64-windows", "aarch64-windows-msvc"}},
}

// Node names builds by Node's process.platform and process.arch, with napi-rs's libc suffixes.
var Node = []Target{
	{GOOS: "android", GOARCH: "386", GO386: "sse2", Names: []string{"android-ia32"}},
	{GOOS: "android", GOARCH: "amd64", Names: []string{"android-x64"}},
	{GOOS: "android", GOARCH: "arm", GOARM: "7", Names: []string{"android-arm-eabi", "android-arm"}},
	{GOOS: "android", GOARCH: "arm64", Names: []string{"android-arm64"}},
	{GOOS: "darwin", GOARCH: "amd64", Names: []string{"darwin-x64", "darwin-universal"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"darwin-arm64", "darwin-universal"}},
	{GOOS: "freebsd", GOARCH: "amd64", Names: []string{"freebsd-x64"}},
	{GOOS: "freebsd", GOARCH: "arm64", Names: []string{"freebsd-arm64"}},
	{GOOS: "illumos", GOARCH: "amd64", Names: []string{"sunos-x64"}},
	{GOOS: "linux", GOARCH: "386", GO386: "sse2", Libc: "musl", Names: []string{"linux-ia32-musl"}},
	{GOOS: "linux", GOARCH: "386", GO386: "sse2", Libc: "gnu", Names: []string{"linux-ia32-gnu"}},
	{GOOS: "linux", GOARCH: "386", GO386: "sse2", Names: []string{"linux-ia32"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "musl", Names: []string{"linux-x64-musl"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"linux-x64-gnu"}},
	{GOOS: "linux", GOARCH: "amd64", Names: []string{"linux-x64"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Libc: "musl", Names: []string{"linux-arm-musleabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Libc: "gnu", Names: []string{"linux-arm-gnueabihf"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Names: []string{"linux-arm"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "musl", Names: []string{"linux-arm64-musl"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "gnu", Names: []string{"linux-arm64-gnu"}},
	{GOOS: "linux", GOARCH: "arm64", Names: []string{"linux-arm64"}},
	{GOOS: "linux", GOARCH: "loong64", Libc: "musl", Names: []string{"linux-loong64-musl"}},
	{GOOS: "linux", GOARCH: "loong64", Libc: "gnu", Names: []string{"linux-loong64-gnu"}},
	{GOOS: "linux", GOARCH: "loong64", Names: []string{"linux-loong64"}},
	{GOOS: "linux", GOARCH: "ppc64le", Libc: "musl", Names: []string{"linux-ppc64-musl"}},
	{GOOS: "linux", GOARCH: "ppc64le", Libc: "gnu", Names: []string{"linux-ppc64-gnu"}},
	{GOOS: "linux", GOARCH: "ppc64le", Names: []string{"linux-ppc64"}},
	{GOOS: "linux", GOARCH: "riscv64", Libc: "musl", Names: []string{"linux-riscv64-musl"}},
	{GOOS: "linux", GOARCH: "riscv64", Libc: "gnu", Names: []string{"linux-riscv64-gnu"}},
	{GOOS: "linux", GOARCH: "riscv64", Names: []string{"linux-riscv64"}},
	{GOOS: "linux", GOARCH: "s390x", Libc: "musl", Names: []string{"linux-s390x-musl"}},
	{GOOS: "linux", GOARCH: "s390x", Libc: "gnu", Names: []string{"linux-s390x-gnu"}},
	{GOOS: "linux", GOARCH: "s390x", Names: []string{"linux-s390x"}},
	{GOOS: "openbsd", GOARCH: "amd64", Names: []string{"openbsd-x64"}},
	{GOOS: "solaris", GOARCH: "amd64", Names: []string{"sunos-x64"}},
	{GOOS: "windows", GOARCH: "386", GO386: "sse2", Names: []string{"win32-ia32-msvc", "win32-ia32", "win32-ia32-gnu"}},
	{GOOS: "windows", GOARCH: "amd64", Names: []string{"win32-x64-msvc", "win32-x64", "win32-x64-gnu"}},
	{GOOS: "windows", GOARCH: "arm64", Names: []string{"win32-arm64-msvc", "win32-arm64", "win32-arm64-gnu"}},
}

// Deno names builds by the target triples deno compile --target accepts.
var Deno = []Target{
	{GOOS: "darwin", GOARCH: "amd64", Names: []string{"x86_64-apple-darwin"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"aarch64-apple-darwin"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"x86_64-unknown-linux-gnu"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "gnu", Names: []string{"aarch64-unknown-linux-gnu"}},
	{GOOS: "windows", GOARCH: "amd64", Names: []string{"x86_64-pc-windows-msvc"}},
	{GOOS: "windows", GOARCH: "arm64", Names: []string{"aarch64-pc-windows-msvc"}},
}

// Bun names builds as bun build --compile does, without the bun- prefix of its --target flag.
var Bun = []Target{
	{GOOS: "darwin", GOARCH: "amd64", Names: []string{"darwin-x64"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"darwin-arm64"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "musl", Names: []string{"linux-x64-musl"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"linux-x64"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "musl", Names: []string{"linux-arm64-musl"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "gnu", Names: []string{"linux-arm64"}},
	{GOOS: "windows", GOARCH: "amd64", Names: []string{"windows-x64"}},
	{GOOS: "windows", GOARCH: "arm64", Names: []string{"windows-arm64"}},
}

// DotNet names builds by .NET runtime identifier, as used by dotnet publish -r.
var DotNet = []Target{
	{GOOS: "android", GOARCH: "amd64", Names: []string{"linux-bionic-x64"}},
	{GOOS: "android", GOARCH: "arm64", Names: []string{"linux-bionic-arm64"}},
	{GOOS: "darwin", GOARCH: "amd64", Names: []string{"osx-x64"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"osx-arm64"}},
	{GOOS: "freebsd", GOARCH: "amd64", Names: []string{"freebsd-x64"}},
	{GOOS: "freebsd", GOARCH: "arm64", Names: []string{"freebsd-arm64"}},
	{GOOS: "illumos", GOARCH: "amd64", Names: []string{"illumos-x64"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "musl", Names: []string{"linux-musl-x64"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"linux-x64"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Libc: "musl", Names: []string{"linux-musl-arm"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Libc: "gnu", Names: []string{"linux-arm"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "musl", Names: []string{"linux-musl-arm64"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "gnu", Names: []string{"linux-arm64"}},
	{GOOS: "linux", GOARCH: "loong64", Libc: "gnu", Names: []string{"linux-loongarch64"}},
	{GOOS: "linux", GOARCH: "ppc64le", Libc: "gnu", Names: []string{"linux-ppc64le"}},
	{GOOS: "linux", GOARCH: "riscv64", Libc: "gnu", Names: []string{"linux-riscv64"}},
	{GOOS: "linux", GOARCH: "s390x", Libc: "gnu", Names: []string{"linux-s390x"}},
	{GOOS: "solaris", GOARCH: "amd64", Names: []string{"solaris-x64"}},
	{GOOS: "windows", GOARCH: "386", GO386: "sse2", Names: []string{"win-x86"}},
	{GOOS: "windows", GOARCH: "amd64", Names: []string{"win-x64"}},
	{GOOS: "windows", GOARCH: "arm64", Names: []string{"win-arm64"}},
}

// Dart names builds by the Dart SDK's platform names, which dart compile exe projects commonly use.
var Dart = []Target{
	{GOOS: "darwin", GOARCH: "amd64", Names: []string{"macos-x64"}},
	{GOOS: "darwin", GOARCH: "arm64", Names: []string{"macos-arm64"}},
	{GOOS: "linux", GOARCH: "amd64", Libc: "gnu", Names: []string{"linux-x64"}},
	{GOOS: "linux", GOARCH: "arm", GOARM: "7", Libc: "gnu", Names: []string{"linux-arm"}},
	{GOOS: "linux", GOARCH: "arm64", Libc: "gnu", Names: []string{"linux-arm64"}},
	{GOOS: "linux", GOARCH: "riscv64", Libc: "gnu", Names: []string{"linux-riscv64"}},
	{GOOS: "windows", GOARCH: "amd64", Names: []string{"windows-x64"}},
	{GOOS: "windows", GOARCH: "arm64", Names: []string{"windows-arm64"}},
}
