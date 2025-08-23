package date_utils

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// his file contains utility function to parse date with configurable pivot year

var pivotYear int

func init() {
	var err error
	pivot := os.Getenv("JETS_PIVOT_YEAR_TIME_PARSING")
	if len(pivot) > 0 {
		pivotYear, err = strconv.Atoi(pivot)
		switch {
		case err != nil:
			log.Printf("Warning: env var JETS_PIVOT_YEAR_TIME_PARSING with value %s cannot be parsed as int\n", pivot)
			pivotYear = 0
		case pivotYear < 0:
			log.Printf("Warning: env var JETS_PIVOT_YEAR_TIME_PARSING with value %s cannot be used as pivot year\n", pivot)
			pivotYear = 0
		}
	}
}

// JetStore time parsing function using go-style layout.
// This uses the pivot year define in env var JETS_PIVOT_YEAR_TIME_PARSING
func ParseDateTime(layout, value string) (time.Time, error) {
	tm, err := time.Parse(layout, value)
	if err != nil {
		return tm, err
	}
	if pivotYear > 0 && !strings.Contains(layout, "2006") {
		y := tm.Year()
		if y > 1900 {
			// log.Printf("*** adjusting parsed year %d on %s :: %v\n", y, value, tm)
			// undo what time.Parse did: For layouts specifying the two-digit year, a value NN >= 69
			//  will be treated as 19NN and a value NN < 69 will be treated as 20NN.
			if y < 2000 {
				y -= 1900
			} else {
				y -= 2000
			}
		}
		// log.Printf("*** pivotYear %d, parsed year %d\n", pivotYear, y)
		if y >= pivotYear {
			y += 1900
		} else {
			y += 2000
		}
		tm = time.Date(y, tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond(), tm.Location())
	}
	// log.Printf("*** returning %v\n", tm)
	return tm, nil
}
