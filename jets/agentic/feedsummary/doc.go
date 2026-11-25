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
// source configuration and profile, and the feed's condition at the locus with
// the incidents and verdicts on it, and renders. A summary therefore says what the record says now: a profile
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
// # A condition is one incident
//
// A feed that sends the same faulty delivery every week is one fault that
// persists: shadow triage raises one incident for the feed's consecutive
// deliveries with a finding at one locus, a condition, on its first delivery,
// and the later deliveries join it (triage's feed_condition.go). The summary
// says how many deliveries the incident holds, from when to when, whether the
// condition is still open or how it ended, and the incident before it at the
// locus with its verdict.
//
// **It reads the condition through jets/agentic/feedcondition, the resolver
// the writer places a finding with**, not a reading of its own: which incident
// each delivery of the condition belongs to is feedcondition's Resolve, so the
// summary and the writer cannot disagree about it. And it reads the previous
// incident and its verdict when the summary is read, not from the incident's
// basis, which records only what was known when the incident was raised: a
// backfill triaged newest first raises a later condition before the one before
// it, whose incident the summary then names once it exists. Reading the summary
// changes nothing.
package feedsummary
