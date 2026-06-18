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

type AggregateResponse struct {
	Total int                      `json:"total"`
	Data  []slowlog.FingerprintStats `json:"data"`
	Error string                   `json:"error,omitempty"`
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	defaultLogPath := flag.String("log", "slowquery.log", "Default slow query log file path")
	defaultSQLMaxLen := flag.Int("sql-max-len", 1000, "Default max SQL length, 0 means no truncation")
	flag.Parse()

	parseQueries := func(r *http.Request) ([]slowlog.SlowQuery, error) {
		logPath := r.URL.Query().Get("path")
		if logPath == "" {
			logPath = *defaultLogPath
		}
		return slowlog.ParseFile(logPath)
	}

	http.HandleFunc("/api/slowlog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(APIResponse{Error: "method not allowed"})
			return
		}

		queries, err := parseQueries(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIResponse{Error: err.Error()})
			return
		}

		minTimeStr := r.URL.Query().Get("min_time")
		sortBy := r.URL.Query().Get("sort")
		limitStr := r.URL.Query().Get("limit")
		sqlMaxLenStr := r.URL.Query().Get("sql_max_len")

		sqlMaxLen := *defaultSQLMaxLen
		if sqlMaxLenStr != "" {
			if v, err := strconv.Atoi(sqlMaxLenStr); err == nil {
				sqlMaxLen = v
			}
		}

		if sqlMaxLen > 0 {
			for i := range queries {
				queries[i].TruncateSQL(sqlMaxLen)
			}
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

	http.HandleFunc("/api/slowlog/aggregate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(AggregateResponse{Error: "method not allowed"})
			return
		}

		queries, err := parseQueries(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AggregateResponse{Error: err.Error()})
			return
		}

		minTimeStr := r.URL.Query().Get("min_time")
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

		stats := slowlog.AggregateByFingerprint(queries)

		sortBy := r.URL.Query().Get("sort")
		slowlog.SortFingerprintStats(stats, sortBy)

		limitStr := r.URL.Query().Get("limit")
		if limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(stats) {
				stats = stats[:limit]
			}
		}

		sqlMaxLenStr := r.URL.Query().Get("sql_max_len")
		sqlMaxLen := *defaultSQLMaxLen
		if sqlMaxLenStr != "" {
			if v, err := strconv.Atoi(sqlMaxLenStr); err == nil {
				sqlMaxLen = v
			}
		}
		if sqlMaxLen > 0 {
			for i := range stats {
				if len(stats[i].SampleSQL) > sqlMaxLen {
					cutAt := sqlMaxLen - len("... [truncated]")
					if cutAt < 0 {
						cutAt = 0
					}
					stats[i].SampleSQL = stats[i].SampleSQL[:cutAt] + "... [truncated]"
				}
				if len(stats[i].Fingerprint) > sqlMaxLen {
					cutAt := sqlMaxLen - len("... [truncated]")
					if cutAt < 0 {
						cutAt = 0
					}
					stats[i].Fingerprint = stats[i].Fingerprint[:cutAt] + "... [truncated]"
				}
			}
		}

		json.NewEncoder(w).Encode(AggregateResponse{
			Total: len(stats),
			Data:  stats,
		})
	})

	fmt.Printf("Slow query log HTTP server starting on %s\n", *addr)
	fmt.Printf("Default log file: %s\n", *defaultLogPath)
	fmt.Printf("Default SQL max length: %d (0 = no truncation)\n", *defaultSQLMaxLen)
	fmt.Printf("Endpoints:\n")
	fmt.Printf("  GET /api/slowlog\n")
	fmt.Printf("  GET /api/slowlog/aggregate\n")
	fmt.Printf("Query params: path, min_time, sort, limit, sql_max_len\n")
	log.Fatal(http.ListenAndServe(*addr, nil))
}
