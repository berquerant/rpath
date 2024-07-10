package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/berquerant/rpath"
)

const usage = `rpath - find path of the element present at specified position

Usage:

  rpath [flags] CATEGORY [FILE]

Available CATEGORY:
- yaml, yml

Flags:`

func Usage() {
	fmt.Fprintln(os.Stderr, usage)
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("rpath: ")

	var (
		line    = flag.Int("line", 0, "Line number of target, 1-based")
		column  = flag.Int("column", 0, "Column number of target, 1-based")
		offset  = flag.Int("offset", -1, "Offset of target, 0-based")
		verbose = flag.Bool("verbose", false, "Verbose output")
		debug   = flag.Bool("debug", false, "Enable debug logs")
	)

	flag.Usage = Usage
	flag.Parse()

	if *debug {
		rpath.EnableDebug()
	}

	var (
		category = flag.Arg(0)
		filename = flag.Arg(1)
	)

	runner := &runner{
		category: category,
		filename: filename,
		verbose:  *verbose,
		line:     *line,
		column:   *column,
		offset:   *offset,
	}

	if err := runner.run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		Usage()
		os.Exit(1)
	}
}

type runner struct {
	category string
	verbose  bool
	line     int
	column   int
	offset   int
	filename string
}

func (r runner) getQueryer() (rpath.Queryer, error) {
	switch r.category {
	case "yaml", "yml":
		var query rpath.YamlQuery
		return &query, nil
	default:
		return nil, fmt.Errorf("Invalid category: %s", r.category)
	}
}

func (r runner) run() error {
	query, err := r.getQueryer()
	if err != nil {
		return err
	}

	var file io.Reader = os.Stdin
	if r.filename != "" {
		if file, err = os.Open(r.filename); err != nil {
			return err
		}
		defer file.(*os.File).Close()
	}

	var result *rpath.Result
	if result, err = query.Query(file, &rpath.Position{
		Line:   r.line,
		Column: r.column,
		Offset: r.offset,
	}); err != nil {
		return fmt.Errorf("%w: failed to query", err)
	}

	if r.verbose {
		var b []byte
		if b, err = json.Marshal(result); err != nil {
			return fmt.Errorf("%w: failed to marshal result", err)
		}
		fmt.Printf("%s\n", b)
		return nil
	}
	fmt.Print(result.Path)
	return nil
}
