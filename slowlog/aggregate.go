package slowlog

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"time"
)

type FingerprintStats struct {
	Fingerprint   string    `json:"fingerprint"`
	FingerprintMD5 string   `json:"fingerprint_md5"`
	SampleSQL     string    `json:"sample_sql"`
	Count         int       `json:"count"`
	QueryTimeSum     float64 `json:"query_time_sum"`
	QueryTimeAvg     float64 `json:"query_time_avg"`
	QueryTimeMax     float64 `json:"query_time_max"`
	QueryTimeMin     float64 `json:"query_time_min"`
	LockTimeSum      float64 `json:"lock_time_sum"`
	LockTimeAvg      float64 `json:"lock_time_avg"`
	RowsSentSum      int64   `json:"rows_sent_sum"`
	RowsSentAvg      float64 `json:"rows_sent_avg"`
	RowsExaminedSum  int64   `json:"rows_examined_sum"`
	RowsExaminedAvg  float64 `json:"rows_examined_avg"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

func AggregateByFingerprint(queries []SlowQuery) []FingerprintStats {
	group := make(map[string]*FingerprintStats)

	for _, q := range queries {
		fp := Fingerprint(q.SQL)
		if fp == "" {
			continue
		}

		stats, ok := group[fp]
		if !ok {
			stats = &FingerprintStats{
				Fingerprint:   fp,
				FingerprintMD5: md5hex(fp),
				SampleSQL:     q.SQL,
				QueryTimeMin:  q.QueryTime,
				FirstSeen:     q.Time,
			}
			group[fp] = stats
		}

		stats.Count++
		stats.QueryTimeSum += q.QueryTime
		stats.LockTimeSum += q.LockTime
		stats.RowsSentSum += q.RowsSent
		stats.RowsExaminedSum += q.RowsExamined

		if q.QueryTime > stats.QueryTimeMax {
			stats.QueryTimeMax = q.QueryTime
			stats.SampleSQL = q.SQL
		}
		if q.QueryTime < stats.QueryTimeMin {
			stats.QueryTimeMin = q.QueryTime
		}
		if q.Time.After(stats.LastSeen) || stats.LastSeen.IsZero() {
			stats.LastSeen = q.Time
		}
		if !q.Time.IsZero() && q.Time.Before(stats.FirstSeen) {
			stats.FirstSeen = q.Time
		}
	}

	result := make([]FingerprintStats, 0, len(group))
	for _, s := range group {
		if s.Count > 0 {
			s.QueryTimeAvg = s.QueryTimeSum / float64(s.Count)
			s.LockTimeAvg = s.LockTimeSum / float64(s.Count)
			s.RowsSentAvg = float64(s.RowsSentSum) / float64(s.Count)
			s.RowsExaminedAvg = float64(s.RowsExaminedSum) / float64(s.Count)
		}
		result = append(result, *s)
	}

	return result
}

func SortFingerprintStats(stats []FingerprintStats, by string) {
	switch by {
	case "count":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Count > stats[j].Count
		})
	case "query_time_sum", "total_time", "":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].QueryTimeSum > stats[j].QueryTimeSum
		})
	case "query_time_avg", "avg_time":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].QueryTimeAvg > stats[j].QueryTimeAvg
		})
	case "query_time_max", "max_time":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].QueryTimeMax > stats[j].QueryTimeMax
		})
	case "rows_examined_sum":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].RowsExaminedSum > stats[j].RowsExaminedSum
		})
	case "first_seen":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].FirstSeen.After(stats[j].FirstSeen)
		})
	case "last_seen":
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].LastSeen.After(stats[j].LastSeen)
		})
	}
}

func md5hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
