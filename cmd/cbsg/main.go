package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/thegalkin/corporate-slop-generator-russian/internal/generator"
)

// version — версия проекта; меняется вручную при релизе.
const version = "1.0.0"

func main() {
	var (
		n     = flag.Int("n", 1, "сколько предложений выдать")
		seed  = flag.Int64("seed", 0, "зерно генератора (0 — случайное)")
		mode  = flag.String("mode", "sentence", "sentence | paragraph | workshop | report | title")
		quiet = flag.Bool("quiet", false, "не печатать сводку")
		showV = flag.Bool("version", false, "показать версию")
	)
	flag.Parse()

	if *showV {
		fmt.Println("corporate-slop-generator-russian", version)
		return
	}

	if *seed == 0 {
		*seed = nowSeed()
	}
	g, err := generator.New(*seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}

	var out string
	switch *mode {
	case "sentence":
		out = g.Workshop(*n)
	case "paragraph":
		out = g.Paragraph(*n)
	case "workshop":
		out = g.Workshop(*n)
	case "report":
		out = g.Report(*n)
	case "title":
		out = g.Title()
	default:
		fmt.Fprintf(os.Stderr, "неизвестный режим: %s\n", *mode)
		os.Exit(2)
	}

	fmt.Println(strings.TrimSpace(out))
	if !*quiet {
		fmt.Fprintf(os.Stderr, "\n%s (seed=%d)\n", g.Summarize(out), *seed)
	}
}
