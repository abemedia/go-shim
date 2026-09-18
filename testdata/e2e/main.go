package main

import "github.com/abemedia/go-shim"

func main() {
	shim.Main(shim.Config{
		URL:     "stagelint-{{.Version}}-{{.Target}}{{.Ext}}",
		Targets: shim.Rust,
		Version: "v0.2.0",
	})
}
