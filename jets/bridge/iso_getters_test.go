package bridge

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// I-162: GetDatetimeIsoString and GetDateIsoString from many goroutines at once.
// The library's ISO getters returned text in a buffer of the calling C thread, which
// the bridge copied with C.GoString after the cgo call returned. By then the
// goroutine could have moved, and another goroutine's call on the first thread could
// have overwritten the buffer: one call in about 8,000 returned another value's text,
// measured 2026-10-23. The bridge now passes its own buffer. Bounded to about two
// seconds; it stops at the first wrong values it has seen enough of.
func TestIsoGettersFromManyGoroutines(t *testing.T) {
	js, err := LoadJetRules("", "test_data/bridge_test1.db", "")
	if err != nil {
		t.Fatal(err)
	}
	const n = 8
	datetimes := make([]*Resource, n)
	dates := make([]*Resource, n)
	wantDatetime := make([]string, n)
	wantDate := make([]string, n)
	for i := 0; i < n; i++ {
		wantDatetime[i] = fmt.Sprintf("2020-01-%02dT10:11:%02d", i+1, i)
		if datetimes[i], err = js.NewDatetimeLiteral(wantDatetime[i]); err != nil {
			t.Fatal(err)
		}
		wantDate[i] = fmt.Sprintf("2021-02-%02d", i+1)
		if dates[i], err = js.NewDateLiteral(wantDate[i]); err != nil {
			t.Fatal(err)
		}
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	var calls, wrong atomic.Int64
	var wg sync.WaitGroup
	deadline := time.Now().Add(2 * time.Second)
	for g := 0; g < 4*n; g++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for time.Now().Before(deadline) && wrong.Load() < 20 {
				for k := 0; k < 1000; k++ {
					s, err := datetimes[i].GetDatetimeIsoString()
					if err != nil || s != wantDatetime[i] {
						if wrong.Add(1) <= 5 {
							t.Logf("datetime %d: got %q, %v; want %q", i, s, err, wantDatetime[i])
						}
					}
					s, err = dates[i].GetDateIsoString()
					if err != nil || s != wantDate[i] {
						if wrong.Add(1) <= 5 {
							t.Logf("date %d: got %q, %v; want %q", i, s, err, wantDate[i])
						}
					}
					calls.Add(2)
				}
			}
		}(g % n)
	}
	wg.Wait()
	if wrong.Load() > 0 {
		t.Errorf("%d wrong values in %d calls", wrong.Load(), calls.Load())
	}
}
