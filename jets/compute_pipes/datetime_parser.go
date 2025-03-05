package compute_pipes

import (
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func ParseDate(date string) (*time.Time, error) {
	return rdf.ParseDate(date)
}

func ParseDateStrict(date string) (*time.Time, error) {
	return rdf.ParseDateStrict(date)
}

func ParseDatetime(datetime string) (*time.Time, error) {
	return rdf.ParseDatetime(datetime)
}
