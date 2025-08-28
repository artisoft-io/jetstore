package date_utils

import "strings"

// Go Layout Reference
//	Year: "2006" "06"
//	Month: "Jan" "January" "01" "1"
//	Day of the week: "Mon" "Monday"
//	Day of the month: "2" "_2" "02"
//	Day of the year: "__2" "002"
//	Hour: "15" "3" "03" (PM or AM)
//	Minute: "4" "04"
//	Second: "5" "05"
//	AM/PM mark: "PM"
//
// Numeric time zone offsets format as follows:
//
//	"-0700"     ±hhmm
//	"-07:00"    ±hh:mm
//	"-07"       ±hh
//	"-070000"   ±hhmmss
//	"-07:00:00" ±hh:mm:ss

func FromJavaDateFormat(format string, forRead bool) string {
	if forRead {
		switch {
		case !strings.Contains(format, "yy"):
			forRead = false
		case !strings.Contains(format, "dd"):
			forRead = false
		case !strings.ContainsAny(format, "/-"):
			forRead = false
		}
	}
	if forRead {
		format = strings.Replace(format, "yyyy", "2006", 1)
		format = strings.Replace(format, "yy", "06", 1)
		format = strings.Replace(format, "MMMM", "January", 1)
		format = strings.Replace(format, "MMM", "Jan", 1)
		format = strings.Replace(format, "MM", "1", 1)
		format = strings.Replace(format, "dd", "2", 1)
		format = strings.Replace(format, "D", "__2", 1)
		format = strings.Replace(format, "EEEE", "Monday", 1)
		format = strings.Replace(format, "EEE", "Mon", 1)
		format = strings.Replace(format, "hh", "03", 1)
		format = strings.Replace(format, "HH", "15", 1)
		format = strings.Replace(format, "mm", "04", 1)
		format = strings.Replace(format, "ss", "05", 1)
		format = strings.Replace(format, "aa", "PM", 1)
	} else {
		format = strings.Replace(format, "yyyy", "2006", 1)
		format = strings.Replace(format, "yy", "06", 1)
		format = strings.Replace(format, "MMMM", "January", 1)
		format = strings.Replace(format, "MMM", "Jan", 1)
		format = strings.Replace(format, "MM", "01", 1)
		format = strings.Replace(format, "dd", "02", 1)
		format = strings.Replace(format, "D", "002", 1)
		format = strings.Replace(format, "EEEE", "Monday", 1)
		format = strings.Replace(format, "EEE", "Mon", 1)
		format = strings.Replace(format, "hh", "03", 1)
		format = strings.Replace(format, "HH", "15", 1)
		format = strings.Replace(format, "mm", "04", 1)
		format = strings.Replace(format, "ss", "05", 1)
		format = strings.Replace(format, "aa", "PM", 1)
	}
	return format
}
