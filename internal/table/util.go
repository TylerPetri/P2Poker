package table

import "time"

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}

func removeStr(ss *[]string, x string) {
	out := (*ss)[:0]
	for _, s := range *ss {
		if s != x {
			out = append(out, s)
		}
	}
	*ss = out
}

func maxDur(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
