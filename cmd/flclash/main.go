package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BIGGASSS/FlClash-MyGo/internal/app"
)

func main() {
	safe := flag.Bool("safe-mode", false, "Preview the UI without starting the core or changing networking")
	dir := flag.String("data-dir", "", "Separate app data directory")
	core := flag.String("core", "", "Path to the FlClash core executable")
	render := flag.String("render", "", "Render the native UI to a PNG without opening a window")
	page := flag.String("page", "dashboard", "Screen to render")
	dark := flag.Bool("dark", false, "Render in dark mode")
	version := flag.Bool("version", false, "Print version")
	flag.Parse()
	if *version {
		fmt.Println("FlClash MyGo " + app.Version)
		return
	}
	if *core == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		name := "flclash-core"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		*core = filepath.Join(filepath.Dir(exe), name)
	}
	a, err := app.New(*dir, *core, *safe || *render != "")
	if err != nil {
		log.Fatal(err)
	}
	if *render != "" {
		a.SetPage(*page)
		err = a.Render(*render, 1200, 800, *dark)
	} else {
		err = a.Run()
	}
	if err != nil {
		log.Fatal(err)
	}
}
