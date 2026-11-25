// Package feedfixture is the synthetic fixture the feed detectors are tested
// against: a set of feeds and their deliveries, installed into a throwaway
// Postgres as the JetStore rows a feed detector reads, with a declared profile
// per feed and a written expected result per detector. It holds no member
// data: every claim, member and amount is generated here, deterministically.
//
// # What it installs
//
//   - jetsapi.source_config, one row per feed, with is_cumulative;
//   - jetsapi.input_registry, one row per delivery (and, for the re-processing
//     feeds, per re-registration), joined to its feed on
//     table_name. Every feed of a client registers under the same domain key
//     (DomainKey), as RegisterFileKeys writes it, so a detector that joins a
//     delivery to its feed on object_type finds every feed of the client under
//     one name;
//   - jetsapi.source_period, one row per date a file key names, with
//     month_period, week_period and day_period computed the way
//     datatable.CalculatePeriod computes them;
//   - jetsapi.file_key_staging, one row per file key, carrying the size. Its
//     last_update is a sync time (SyncTime), the same for every row, because
//     that is what a sync leaves there: it is not the arrival;
//   - one claim-set table per profiled feed, in the public schema, carrying
//     session_id, a business key, a service date, a member column, a paid
//     amount and one non-business column that differs between deliveries.
//
// The arrival of a delivery is its session id, the registration time in
// milliseconds, as RegisterFileKeys seeds it. Nothing reads the clock: Clock is
// the detection time the expected result is written for.
//
// # Re-processing
//
// A delivery is a file at its first registration. ReprocessingFeeds are two
// more feeds whose files are registered again, as JetStore records a
// re-processing: a re-map that registers every earlier file of one feed in a
// burst, and a backfill of past-period files each registered two or three
// times within seconds. Each re-registration is an input_registry row with the
// file's key and period and a session of its own, and the file's claims loaded
// again under that session. InstallReprocessing writes them on top of Install,
// with the profiles at ReprocessingProfilesPath, and their expected verdicts
// are expected.json's reprocessing section; the base feeds and their written
// totals are unchanged by them.
//
// # Not-judged cases
//
// A content check that cannot evaluate a delivery says why. Two of the reasons
// are carried by base feeds (a feed with no profile, and a claim-set table
// holding none of a feed's deliveries); NotJudgedFeeds are four more feeds that
// carry the rest: a delivery whose claims were not loaded, so its successor's
// comparisons are with an empty set (predecessor_no_claim_rows); a delivery
// none of whose rows carries a date of service (no_service_dates); and a
// profile naming a column the claim-set table lacks, once for the member
// column every check reads and once for a required field (column_absent). The
// last also declares no size range, so delivery_size judges it against its
// trailing band (insufficient_history, then a firing). InstallNotJudged writes
// them, alone or beside Install, with the profiles at NotJudgedProfilesPath,
// and their expected verdicts are expected.json's not_judged section.
//
// arrival_unknown has no case: the detectors read a registration as a delivery
// only when its session id is a registration time, so no installed row can
// reach it.
//
// # What it declares, and what it expects
//
// testdata/feed_profiles.json holds one profile per profiled feed, keyed by
// table_name, in the shape a profile file is read in. testdata/expected.json is
// the written expected result: for each of the eight feed detectors and each
// feed, which deliveries fire, which are silent, and which are not judged and
// why, together with the row counts an install leaves and the rule each
// verdict was computed under. The detector tests assert against it; this
// package's own test checks it against the installed rows.
//
// # Canaries
//
// Every member value carries MemberCanary and every business key KeyCanary, so
// a test can assert that no anomaly, incident, hypothesis, summary or prompt
// carries a member-level value by searching for the two tokens.
//
// # Running the test
//
// The install test needs JETS_TEST_DSN, a throwaway database it may write to,
// and is skipped otherwise. Run it with -count=1: it reads jets_schema.json,
// which the test cache does not see.
//
//	docker run -d --rm --name feedfixture-pg -e POSTGRES_PASSWORD=pw -p 5497:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5497/postgres \
//	  go test -count=1 ./jets/agentic/observe/feedfixture/
package feedfixture
