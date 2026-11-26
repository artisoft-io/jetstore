package main

import (
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/purge_database/delegate"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"
)

// Sample lambda function in go to purge historical data

type config struct {
	AWSRegion     string
	AWSDnsSecret  string
	RetentionDays int
	DryRun        bool
	IsValid       bool
}

var logger *zap.SugaredLogger
var c config

// Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// RETENTION_DAYS site global rentention days, delete sessions if > 0
// PURGE_DRY_RUN a dry run unless exactly "false": the purge reports and deletes nothing
// AGENT_AUDIT_RETENTION_DAYS optional: the agent audit trail is purged after this many days; unset, never
// DOMAIN_TABLES_RETENTION_DAYS optional: a session's rows in the domain tables and process_errors are
// deleted after this many days, the session kept to RETENTION_DAYS; unset, RETENTION_DAYS

func main() {
	// Create logger.
	var err error
	utils.UseJetStoreLogger()
	logger = zap.L().Sugar()

	// Check required env var
	c.IsValid = true
	c.AWSRegion = os.Getenv("JETS_REGION")
	if c.AWSRegion == "" {
		logger.Error("env JETS_REGION not set")
		c.IsValid = false
	}
	c.AWSDnsSecret = os.Getenv("JETS_DSN_SECRET")
	if c.AWSDnsSecret == "" {
		logger.Error("env JETS_DSN_SECRET not set")
		c.IsValid = false
	}
	rd := os.Getenv("RETENTION_DAYS")
	logger.Infof("env RETENTION_DAYS: %s", rd)
	if rd == "" {
		logger.Error("env RETENTION_DAYS not set")
		c.IsValid = false
	}
	c.RetentionDays, err = strconv.Atoi(rd)
	if err != nil || c.RetentionDays < 1 {
		logger.Errorf("env RETENTION_DAYS '%s' is not valid, must be > 0", rd)
		c.IsValid = false
	}
	c.DryRun = delegate.IsDryRun(os.Getenv("PURGE_DRY_RUN"))
	logger.Infof("env PURGE_DRY_RUN: '%s', dry run: %v", os.Getenv("PURGE_DRY_RUN"), c.DryRun)
	auditDays, err := delegate.AuditRetentionDays(os.Getenv("AGENT_AUDIT_RETENTION_DAYS"))
	if err != nil {
		logger.Error(err)
		c.IsValid = false
	}
	logger.Infof("env AGENT_AUDIT_RETENTION_DAYS: '%s', audit trail kept for %d days (0: never purged)",
		os.Getenv("AGENT_AUDIT_RETENTION_DAYS"), auditDays)
	// A bad DOMAIN_TABLES_RETENTION_DAYS is not fatal here: the purge refuses the
	// run with the database open, so the refusal is recorded as the job's failure.
	tableDays, capped, err := delegate.DomainTablesRetentionDays(os.Getenv("DOMAIN_TABLES_RETENTION_DAYS"), c.RetentionDays)
	if err != nil {
		logger.Error(err, "; every run is refused until it is corrected")
	} else {
		logger.Infof("env DOMAIN_TABLES_RETENTION_DAYS: '%s', table data kept for %d days (capped to RETENTION_DAYS: %v)",
			os.Getenv("DOMAIN_TABLES_RETENTION_DAYS"), tableDays, capped)
	}
	if !c.IsValid {
		logger.Fatal("Invalid configuration, exiting program")
	}

	// Start handler.
	lambda.Start(handler)
}

// The handler functions
func handler() (err error) {
	logger.Info("Starting in ", zap.String("AWS Region", c.AWSRegion))
	err = delegate.DoPurgeSessions()
	if err != nil {
		logger.Error("while updating status (delegate.DoPurgeSessions()):", zap.NamedError("error", err))
		return err
	}

	return
}
