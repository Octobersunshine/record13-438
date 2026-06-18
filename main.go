package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"slowlog-http/slowlog"
)

type APIResponse struct {
	Total int               `json:"total"`
	Data  []slowlog.SlowQuery `json:"data"`
	Error string            `json:"error,omitempty"`
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	defaultLogPath := flag.String("log", "slowquery.log", "Default slow query log file path")
	flag.Parse()

	http.HandleFunc("/api/slowlog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(APIResponse{Error: "method not allowed"})
			return
		}

		logPath := r.URL.Query().Get("path")
		if logPath == "" {
			logPath = *defaultLogPath
		}

		minTimeStr := r.URL.Query().Get("min_time")
		sortBy := r.URL.Query().Get("sort")
		limitStr := r.URL.Query().Get("limit")

		queries, err := slowlog.ParseFile(logPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIResponse{Error: err.Error()})
			return
		}

		if minTimeStr != "" {
			if minTime, err := strconv.ParseFloat(minTimeStr, 64); err == nil {
				filtered := make([]slowlog.SlowQuery, 0, len(queries))
				for _, q := range queries {
					if q.QueryTime >= minTime {
						filtered = append(filtered, q)
					}
				}
				queries = filtered
			}
		}

		switch strings.ToLower(sortBy) {
		case "time":
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].Time.After(queries[j].Time)
			})
		case "query_time", "duration", "":
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].QueryTime > queries[j].QueryTime
			})
		case "rows_examined":
			sort.Slice(queries, func(i, j int) bool {
				return queries[i].RowsExamined > queries[j].RowsExamined
			})
		}

		if limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(queries) {
				queries = queries[:limit]
			}
		}

		json.NewEncoder(w).Encode(APIResponse{
			Total: len(queries),
			Data:  queries,
		})
	})

	fmt.Printf("Slow query log HTTP server starting on %s\n", *addr)
	fmt.Printf("Default log file: %s\n", *defaultLogPath)
	fmt.Printf("Usage: GET /api/slowlog?path=<log_path>&min_time=<seconds>&sort=<time|query_time|rows_examined>&limit=<N>\n")
	log.Fatal(http.ListenAndServe(*addr, nil))
}
