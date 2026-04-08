package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"bld-backend/apps/exchange/internal/wal/wal_analysis"
)

func main() {
	var (
		path    string
		out     string
		fromLSN uint64
		limit   int
		pretty  bool
	)
	flag.StringVar(&path, "path", "./apps/exchange/data/exchange.wal", "wal file path")
	flag.StringVar(&out, "out", "./apps/exchange/test/exchange.wal.txt", "output text file path")
	flag.Uint64Var(&fromLSN, "from-lsn", 0, "only dump records with lsn > from-lsn")
	flag.IntVar(&limit, "limit", 0, "max number of rows, 0 means no limit")
	flag.BoolVar(&pretty, "pretty", false, "pretty print JSON rows")
	flag.Parse()

	rows, err := wal_analysis.DumpRows(path, fromLSN, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "waldump error: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir error: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output file error: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	if pretty {
		enc.SetIndent("", "  ")
	}
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			fmt.Fprintf(os.Stderr, "encode row error: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("wal dump saved to %s (rows=%d)\n", out, len(rows))
}
