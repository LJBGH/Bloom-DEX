package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"bld-backend/apps/exchange/internal/wal/wal_analysis"
)

func main() {
	var (
		path    string
		fromLSN uint64
		limit   int
		pretty  bool
	)
	flag.StringVar(&path, "path", "./apps/exchange/data/exchange.wal", "wal file path")
	flag.Uint64Var(&fromLSN, "from-lsn", 0, "only dump records with lsn > from-lsn")
	flag.IntVar(&limit, "limit", 0, "max number of rows, 0 means no limit")
	flag.BoolVar(&pretty, "pretty", false, "pretty print JSON rows")
	flag.Parse()

	rows, err := wal_analysis.DumpRows(path, fromLSN, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "waldump error: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			fmt.Fprintf(os.Stderr, "encode row error: %v\n", err)
			os.Exit(1)
		}
	}
}
