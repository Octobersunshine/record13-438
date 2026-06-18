package slowlog

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type SlowQuery struct {
	Time          time.Time `json:"time"`
	User          string    `json:"user"`
	Host          string    `json:"host"`
	QueryTime     float64   `json:"query_time"`
	LockTime      float64   `json:"lock_time"`
	RowsSent      int64     `json:"rows_sent"`
	RowsExamined  int64     `json:"rows_examined"`
	SQL           string    `json:"sql"`
	SQLLength     int       `json:"sql_length"`
	SQLTruncated  bool      `json:"sql_truncated"`
}

var (
	timeRegex         = regexp.MustCompile(`^# Time:\s+(.+)$`)
	userHostRegex     = regexp.MustCompile(`^# User@Host:\s+(\S+?)\[.*?\]\s+@\s+(\S+?)\s+`)
	queryTimeRegex    = regexp.MustCompile(`Query_time:\s*([\d.]+)`)
	lockTimeRegex     = regexp.MustCompile(`Lock_time:\s*([\d.]+)`)
	rowsSentRegex     = regexp.MustCompile(`Rows_sent:\s*(\d+)`)
	rowsExaminedRegex = regexp.MustCompile(`Rows_examined:\s*(\d+)`)
)

func ParseFile(path string) ([]SlowQuery, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var results []SlowQuery
	var current *SlowQuery
	var sqlLines []string
	collectingSQL := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "# Time:") {
			if current != nil {
				current.SQL = normalizeSQL(sqlLines)
				current.SQLLength = len(current.SQL)
				results = append(results, *current)
			}
			current = &SlowQuery{}
			sqlLines = nil
			collectingSQL = false

			if m := timeRegex.FindStringSubmatch(line); len(m) > 1 {
				t, err := parseTime(m[1])
				if err == nil {
					current.Time = t
				}
			}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "# User@Host:") {
			collectingSQL = false
			if m := userHostRegex.FindStringSubmatch(line); len(m) > 2 {
				current.User = m[1]
				current.Host = m[2]
			}
			continue
		}

		if strings.HasPrefix(line, "# Query_time:") || strings.Contains(line, "Query_time:") {
			collectingSQL = false
			if m := queryTimeRegex.FindStringSubmatch(line); len(m) > 1 {
				fmt.Sscanf(m[1], "%f", &current.QueryTime)
			}
			if m := lockTimeRegex.FindStringSubmatch(line); len(m) > 1 {
				fmt.Sscanf(m[1], "%f", &current.LockTime)
			}
			if m := rowsSentRegex.FindStringSubmatch(line); len(m) > 1 {
				fmt.Sscanf(m[1], "%d", &current.RowsSent)
			}
			if m := rowsExaminedRegex.FindStringSubmatch(line); len(m) > 1 {
				fmt.Sscanf(m[1], "%d", &current.RowsExamined)
			}
			continue
		}

		if strings.HasPrefix(line, "SET timestamp=") {
			collectingSQL = true
			continue
		}

		if collectingSQL {
			if strings.TrimSpace(line) == "" {
				continue
			}
			sqlLines = append(sqlLines, line)
		}
	}

	if current != nil {
		current.SQL = normalizeSQL(sqlLines)
		current.SQLLength = len(current.SQL)
		results = append(results, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}

	return results, nil
}

func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000000",
		"060102 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown time format: %s", s)
}

func normalizeSQL(lines []string) string {
	var parts []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts = append(parts, trimmed)
	}
	result := strings.Join(parts, " ")
	result = strings.TrimSuffix(result, ";")
	return result
}

const truncationMarker = "... [truncated]"

func (q *SlowQuery) TruncateSQL(maxLen int) {
	if maxLen <= 0 {
		return
	}
	if q.SQLLength == 0 {
		q.SQLLength = len(q.SQL)
	}
	if len(q.SQL) > maxLen {
		cutAt := maxLen - len(truncationMarker)
		if cutAt < 0 {
			cutAt = 0
		}
		q.SQL = q.SQL[:cutAt] + truncationMarker
		q.SQLTruncated = true
	}
}
