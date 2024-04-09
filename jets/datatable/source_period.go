package datatable

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var secPerDay, secPerWeek int64

func init() {
	secPerDay = 24 * 60 * 60
	secPerWeek = 7 * secPerDay
}

// * TODO refactor to use SourcePeriod entity
type SourcePeriod struct {
	Key         int `json:"key"`
	Year        int `json:"year"`
	Month       int `json:"month"`
	Day         int `json:"day"`
	MonthPeriod int `json:"month_period"`
	WeekPeriod  int `json:"week_period"`
	DayPeriod   int `json:"day_period"`
}

// Calculate the month, week, and day period since unix epoch (1/1/1970)
func CalculatePeriod(year, month, day int) (monthPeriod, weekPeriod, dayPeriod int) {
	fileTime := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	unixTime := fileTime.Unix()
	monthPeriod = (year-1970)*12 + month
	weekPeriod = int(unixTime/secPerWeek + 1)
	dayPeriod = int(unixTime/secPerDay + 1)
	return
}

// Load source period info from database by key
func LoadSourcePeriod(dbpool *pgxpool.Pool, key int) (sp SourcePeriod, err error) {
	stmt := "SELECT year, month, day, month_period, week_period, day_period FROM jetsapi.source_period WHERE key=$1"
	err = dbpool.QueryRow(context.Background(), stmt, key).Scan(
		&sp.Year, &sp.Month, &sp.Day, &sp.MonthPeriod, &sp.WeekPeriod, &sp.DayPeriod)
	return
}

// Insert into source_period and returns the source_period.key
// If row already exist on table, return the key of that row without inserting a new one
func InsertSourcePeriod(dbpool *pgxpool.Pool, year, month, day int) (int, error) {
	var key int64
	stmt := "SELECT key FROM jetsapi.source_period WHERE year=$1 AND month=$2 AND day=$3"
	err := dbpool.QueryRow(context.Background(), stmt, year, month, day).Scan(&key)
	switch {
	case err == nil:
		return int(key), nil
	case errors.Is(err, pgx.ErrNoRows):
		// perform the insert
		month_period, week_period, day_period := CalculatePeriod(year, month, day)
		stmt := fmt.Sprintf(`INSERT INTO
							jetsapi.source_period (
								year,	month, day,
								month_period,	week_period, day_period)
						VALUES
							(%d, %d, %d, %d, %d, %d) RETURNING key`,
			year, month, day, month_period, week_period, day_period)
		err = dbpool.QueryRow(context.Background(), stmt).Scan(&key)
		if err != nil {
			return 0, fmt.Errorf("while inserting into source_period table: %v", err)
		}
		return int(key), nil
	}
	return 0, fmt.Errorf("while querying source_period table: %v", err)
}
