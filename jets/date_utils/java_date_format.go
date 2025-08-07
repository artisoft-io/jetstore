package date_utils

import "strings"

func FromJavaDateFormat(format string, forRead bool) string {
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
// Qualify as a date:
// 	- len < 30
// 	- contains digits, letters, space, comma, dash, slash, column, apostrophe
// Example of longest date to expect:
// 23 November 2025 13:10 AM