// Command vitis-data prepares the server resource pack for a Minecraft version.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/vitismc/vitis/internal/data/vanilla"
)

// main generates versioned vanilla resources from a locally cached server JAR.
func main() {
	version := flag.String("version", "1.21.4", "Minecraft version")
	serverJAR := flag.String("jar", "", "path to a verified server JAR")
	output := flag.String("output", "", "destination resource pack directory")
	unpack := flag.Bool("unpack", false, "extract the nested server JAR and libraries into a new directory instead of generating reports")
	flag.Parse()
	if *serverJAR == "" {
		*serverJAR = filepath.Join(".mc-decompiled", "downloads", *version, "server.jar")
	}
	if *unpack && *output == "" {
		fmt.Fprintln(os.Stderr, "-unpack requires -output pointing to a new directory")
		os.Exit(2)
	}
	if *output == "" {
		*output = filepath.Join(".mcdata", *version, "vanilla")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if *unpack {
		err = vanilla.Unpack(ctx, *serverJAR, *version, *output)
	} else {
		err = vanilla.Generate(ctx, *serverJAR, *version, *output)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
