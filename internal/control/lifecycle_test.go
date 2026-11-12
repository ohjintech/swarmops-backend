package control

import "testing"

func TestCanTransition(t *testing.T) {
	cases := []struct {
		cur, next status
		want      bool
	}{
		{stQueued, stSent, true},
		{stSent, stAcked, true},
		{stAcked, stCompleted, true},
		{stAcked, stSent, false},       // late SENT write after an early ACK
		{stSent, stTimedOut, true},     // never acknowledged
		{stAcked, stTimedOut, false},   // acknowledged, still executing
		{stCompleted, stFailed, false}, // terminal is final
		{stTimedOut, stCompleted, false},
		{stSent, stRejected, true},
	}
	for _, c := range cases {
		if got := canTransition(c.cur, c.next); got != c.want {
			t.Errorf("%v -> %v: got %v want %v", c.cur, c.next, got, c.want)
		}
	}
}
