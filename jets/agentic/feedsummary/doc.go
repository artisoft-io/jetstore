// Package feedsummary renders the summary of a feed incident: which feed, what
// departed from what declaration and when, the ranked readings, and the
// declaration's author and age as the case against every reading.
//
// # The render is the product
//
// The summary is rendered by jets/agentic/template from a document this
// package embeds (feed_incident_summary.json), so every word is configuration
// and the render cannot fail once the document compiles. A model arm drafts
// the same summary from the same input and is counted against it (Count); a
// rendered template has been measured correct where a model's draft misstated
// the fact set, so the template is what a person reads and the model's draft
// is an experiment that is shown to nobody.
//
// # Rendered at read time
//
// Nothing is stored. Summarise reads the incident's anomalies, the feed's
// source configuration and profile, and the same-day deliveries of the feed,
// and renders. A summary therefore says what the record says now: a profile
// edited after the incident is the profile the summary names, which is the
// same profile the hypotheses' case against was written from only if nobody
// has edited it since. The incident's own evidence keeps what was true then.
//
// # Aggregates only
//
// The input type has no field that can carry a row of a delivery (see Input),
// and the model arm's prompt is the render's document and nothing else, so a
// member-level value has no path into either.
//
// # A burst is one incident
//
// When the same check fired on several deliveries of the same feed registered
// the same day, as a backfill does, shadow triage raises them as a single
// incident on the last of them (triage's feed_burst.go). The summary of that
// incident says how many deliveries it stands for, how many have no incident of
// their own, and names any incident another of them carries for a finding
// outside the burst. Reading the summary changes nothing.
package feedsummary
