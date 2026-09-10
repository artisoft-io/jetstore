package jetrules_go_adaptor

// The 22-member evaluation population, driven into a rule session the way the
// loader would (agentic_ai Phase 7 `AT.5`, issue I-549).
//
// # What this file is
//
// `briefing_projection_completeness_test.go` beside it asserts hand-written
// claim fixtures and reads the briefing the rules build from them. This file
// asserts the **population** - 22 members, 301 medical claims and 257 pharmacy
// fills committed at `data/unit_test_data/cintel/22_Patients/` in `jets_ws` -
// through the same `claimWriter`, so the two differ in their data rather than in
// how it is written. `at5_side_by_side_test.go` is what then renders each
// member's briefing twice.
//
// # The mapping is the part of this that fails silently, and it is derived
//
// A CSV column is not an `hc:` property. `process_config/ci_workspace_init_db.sql`
// is what turns one into the other, and **it does more than rename**: it renames,
// it drops, and on two columns it cleanses. Guessing it produces a load that is
// wrong in a way nothing downstream reports - the rules run, a briefing is built,
// and it is missing most of its conditions. That happened on 2026-09-09
// (agentic_ai F789): asserting `M54.50` verbatim gave an 89% diagnosis-lookup
// miss rate and a briefing with almost no conditions in it, which reads as a
// finding about the pipeline and is a defect in the harness.
//
// So `at5MedicalColumns` and `at5PharmacyColumns` below are transcribed from that
// SQL, row by row, and the four properties of it that are not a rename are:
//
//   - `Diagnosis_Code1` to `Diagnosis_Code4` map to
//     `hc:Seconday_Diagnosis_Code1` to `4` - note the workspace's own spelling -
//     through `scrub_characters` with argument `.`, so `M54.50` is asserted as
//     `M5450` and `DiagnosisDescriptionLookup` is keyed dotless;
//   - `NDC` maps to `hc:NDC` through `scrub_characters` with argument `-`;
//   - `Primary_Diagnosis_Code` maps with **no** function, which is I-554 and is
//     invisible here because the column is empty in all 301 rows;
//   - six populated columns map to nothing at all - `Claim_Type`,
//     `Patient_Pay_Amount` and `Total_Paid_Amount` in both files, plus
//     `Drug_Name`, `Drug_Strength` and `Drug_Type_Code` in the pharmacy one. The
//     drug name a briefing carries comes from `DrugInfoLookup` keyed on the NDC
//     (`AM_PCreateEvent21`, `jet_rules/clinical_intel/analysis_pharmacy_rules.jr:52`),
//     not from the claim, so mapping the file's own `Drug_Name` would put a second
//     answer beside the pipeline's.
//
// **The cleansing is applied by calling the production function** rather than by
// reimplementing it: `cleansing_functions.ScrubCharacters` is what the loader
// calls (`cleansing_functions.go:376`), so a change to it reaches this harness.
//
// # Positional, not by name
//
// Both claim headers carry **duplicate column names** - `Claim_Type`,
// `Patient_Pay_Amount` and `Total_Paid_Amount` in both, plus `Drug_Name`,
// `Drug_Strength` and `Drug_Type_Code` in the pharmacy one - where the first
// occurrence is populated and the second is blank throughout. A name-keyed reader
// cannot address them and silently takes one of the two (agentic_ai I-567). So
// the header is read once, the **first** occurrence of each mapped name fixes its
// position, and every row is read by position after that.
//
// # `-count=1`
//
// The population and the compiled workspace are both outside this Go module, so
// nothing in the test cache key changes when either moves. A green run without
// `-count=1` is indistinguishable from a real pass. The gate is
// `requireCompiledWorkspace`, from the completeness test's header.

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/cleansing_functions"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// at5PopulationDir is the population's home in the workspace, relative to
// $WORKSPACES_HOME/$WORKSPACE.
const at5PopulationDir = "data/unit_test_data/cintel/22_Patients"

const (
	at5EligibilityFile = "M021-M040_M007_M008_eligibility.csv"
	at5MedicalFile     = "M021-M040_M007_M008_medical_claims.csv"
	at5PharmacyFile    = "M021-M040_M007_M008_pharmacy_claims.csv"
)

// at5Members is the population's size, asserted rather than counted, so a file
// that loses a member fails instead of measuring 21.
const at5Members = 22

// The population's row counts, from the directory's own README, which measured
// them with a CSV reader because neither file ends in a newline and `wc -l`
// therefore reports one fewer.
const (
	at5MedicalRows  = 301
	at5PharmacyRows = 257
)

// at5Column is one row of `process_mapping`: an input column, the data property
// it becomes, and the cleansing function applied on the way.
type at5Column struct {
	input string
	prop  string
	// scrub is the `argument` of a `scrub_characters` row, or "" for a row with
	// no function. No other function appears in this workspace's mapping.
	scrub string
	// kind is how the literal is typed, from the class declaration in
	// `data_model/hc_model.jr` rather than from the value's appearance.
	kind at5Kind
}

type at5Kind int

const (
	at5Text at5Kind = iota
	at5Date
	at5Double
)

// at5MedicalColumns is `CI_D1_hc:MedicalClaim`'s mapping, restricted to the
// columns the population actually populates.
//
// **Restricted rather than complete, and that is a decision.** The SQL carries 53
// rows and 41 of the file's 59 columns are blank in every row - the whole
// `Billing_*`, `Provider_*` and `Subscriber_*` blocks. Asserting a blank as a
// literal is not what the loader does, so the columns are listed here and the
// empty ones are skipped at assertion time anyway; listing only the populated
// ones keeps this table readable and the skip below keeps it correct if a future
// file populates one.
var at5MedicalColumns = []at5Column{
	{input: "Member_ID", prop: "hc:Member_ID"},
	{input: "Service_Date_From", prop: "hc:Service_Date_From", kind: at5Date},
	{input: "Service_Date_To", prop: "hc:Service_Date_To", kind: at5Date},
	{input: "Member_DOB", prop: "hc:Member_DOB", kind: at5Date},
	{input: "Member_Gender", prop: "hc:Member_Gender"},
	{input: "Claim_ID", prop: "hc:Claim_ID"},
	{input: "Place_Of_Service_Code", prop: "hc:Place_Of_Service_Code"},
	// The four that cleanse. `.` is the argument; `M54.50` is asserted `M5450`.
	{input: "Diagnosis_Code1", prop: "hc:Seconday_Diagnosis_Code1", scrub: "."},
	{input: "Diagnosis_Code2", prop: "hc:Seconday_Diagnosis_Code2", scrub: "."},
	{input: "Diagnosis_Code3", prop: "hc:Seconday_Diagnosis_Code3", scrub: "."},
	{input: "Diagnosis_Code4", prop: "hc:Seconday_Diagnosis_Code4", scrub: "."},
	// Not cleansed, and that asymmetry is I-554. Empty in all 301 rows here.
	{input: "Primary_Diagnosis_Code", prop: "hc:Primary_Diagnosis_Code"},
	{input: "Procedure_Code1", prop: "hc:Procedure_Code1"},
	{input: "Procedure_Code2", prop: "hc:Procedure_Code2"},
	{input: "Procedure_Code3", prop: "hc:Procedure_Code3"},
	{input: "Procedure_Code4", prop: "hc:Procedure_Code4"},
}

// at5PharmacyColumns is `CI_D1_hc:PharmacyClaim`'s mapping, on the same terms.
var at5PharmacyColumns = []at5Column{
	{input: "Member_ID", prop: "hc:Member_ID"},
	{input: "Service_Date", prop: "hc:Service_Date", kind: at5Date},
	{input: "Member_DOB", prop: "hc:Member_DOB", kind: at5Date},
	{input: "Member_Gender", prop: "hc:Member_Gender"},
	{input: "Claim_ID", prop: "hc:Claim_ID"},
	{input: "DAW_Code", prop: "hc:DAW_Code"},
	{input: "Days_Supply", prop: "hc:Days_Supply", kind: at5Double},
	{input: "Quantity_Dispensed", prop: "hc:Quantity_Dispensed", kind: at5Double},
	// The pharmacy file's only cleansed column.
	{input: "NDC", prop: "hc:NDC", scrub: "-"},
}

// at5EligibilityColumns is `CI_D1_hc:Eligibility`'s mapping. The eligibility row
// is asserted because `CI_CopyMemberProperties` copies `hc:Member_ID` onto the
// profile and `BP_Briefing20` reads it off there - a session with claims and no
// eligibility row produces a briefing with no `Briefing_Member_ID`, which is a
// record that cannot be attributed to a member.
var at5EligibilityColumns = []at5Column{
	{input: "Member_ID", prop: "hc:Member_ID"},
	{input: "Member_DOB", prop: "hc:Member_DOB", kind: at5Date},
	{input: "Member_First_Name", prop: "hc:Member_First_Name"},
	{input: "Member_Last_Name", prop: "hc:Member_Last_Name"},
	{input: "Member_Gender", prop: "hc:Member_Gender"},
	{input: "Member_Relationship", prop: "hc:Member_Relationship"},
	{input: "Member_Zip", prop: "hc:Member_Zip"},
	{input: "Group_Number", prop: "hc:Group_Number"},
	{input: "Effective_Date", prop: "hc:Effective_Date", kind: at5Date},
}

// --- reading the files ----------------------------------------------------

// at5Row is one CSV row with its column positions already resolved, so nothing
// downstream sees a header again.
type at5Row struct {
	// at maps a mapped input column to its position, resolved once from the
	// header. Shared by every row of a file.
	at map[string]int
	// fields is the row.
	fields []string
}

func (r at5Row) value(col string) string {
	i, ok := r.at[col]
	if !ok || i >= len(r.fields) {
		return ""
	}
	return strings.TrimSpace(r.fields[i])
}

// at5Population is the three files, grouped by member.
type at5Population struct {
	memberIDs   []string // sorted, so a run is ordered the same way twice
	eligibility map[string]at5Row
	medical     map[string][]at5Row
	pharmacy    map[string][]at5Row
	// The totals, kept so the non-vacuity test can assert against the files
	// rather than against a constant somebody typed.
	medicalRows, pharmacyRows int
}

// at5ReadFile reads one CSV positionally.
//
// **The first occurrence of a duplicated name wins**, which is not a tie-break
// but the mapping's own semantics: the loader resolves an input column to a
// position and the second `Claim_Type` is blank in every row of both files
// (I-567). A reader taking the last occurrence would map every one of the six to
// an empty string and lose `Drug_Name` and `Days_Supply` silently.
func at5ReadFile(t *testing.T, path string, cols []at5Column) ([]at5Row, error) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	// The three files have 59, 55 and 27 columns and a row is not obliged to be
	// ragged for this to matter; -1 makes a short row a short row rather than an
	// error, and at5Row.value treats a missing position as empty.
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%s holds %d records", path, len(records))
	}
	header := records[0]
	// Strip a UTF-8 BOM from the first header cell if the file carries one.
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	at := make(map[string]int, len(cols))
	for i, name := range header {
		name = strings.TrimSpace(name)
		if _, seen := at[name]; seen {
			continue // first occurrence wins
		}
		at[name] = i
	}
	for _, c := range cols {
		if _, ok := at[c.input]; !ok {
			return nil, fmt.Errorf("%s: the mapping names input column %q and the header does not carry it",
				filepath.Base(path), c.input)
		}
	}
	out := make([]at5Row, 0, len(records)-1)
	for _, rec := range records[1:] {
		out = append(out, at5Row{at: at, fields: rec})
	}
	return out, nil
}

// at5LoadPopulation reads the three files and groups them by member.
func at5LoadPopulation(t *testing.T) *at5Population {
	t.Helper()
	dir := filepath.Join(os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"), at5PopulationDir)
	elig, err := at5ReadFile(t, filepath.Join(dir, at5EligibilityFile), at5EligibilityColumns)
	if err != nil {
		t.Fatalf("reading the eligibility file: %v", err)
	}
	med, err := at5ReadFile(t, filepath.Join(dir, at5MedicalFile), at5MedicalColumns)
	if err != nil {
		t.Fatalf("reading the medical claim file: %v", err)
	}
	rx, err := at5ReadFile(t, filepath.Join(dir, at5PharmacyFile), at5PharmacyColumns)
	if err != nil {
		t.Fatalf("reading the pharmacy claim file: %v", err)
	}
	p := &at5Population{
		eligibility:  map[string]at5Row{},
		medical:      map[string][]at5Row{},
		pharmacy:     map[string][]at5Row{},
		medicalRows:  len(med),
		pharmacyRows: len(rx),
	}
	for _, r := range elig {
		id := r.value("Member_ID")
		if id == "" {
			continue
		}
		p.eligibility[id] = r
		p.memberIDs = append(p.memberIDs, id)
	}
	sort.Strings(p.memberIDs)
	for _, r := range med {
		if id := r.value("Member_ID"); id != "" {
			p.medical[id] = append(p.medical[id], r)
		}
	}
	for _, r := range rx {
		if id := r.value("Member_ID"); id != "" {
			p.pharmacy[id] = append(p.pharmacy[id], r)
		}
	}
	return p
}

// --- asserting one member's records ---------------------------------------

// at5AssertRow applies one mapping to one row and asserts what it produces.
//
// **An empty input yields no triple**, which is what the loader does and is why
// the blank two-thirds of both claim files does not reach the session. An empty
// string asserted as a text literal is a value, and a rule reading
// `hc:Diagnosis_Codes` would then see one per blank secondary code.
func (c *claimWriter) at5AssertRow(subject string, row at5Row, cols []at5Column) {
	c.t.Helper()
	for _, col := range cols {
		raw := row.value(col.input)
		if raw == "" {
			continue
		}
		if col.scrub != "" {
			scrubbed := cleansing_functions.ScrubCharacters(raw, col.scrub)
			s, ok := scrubbed.(string)
			if !ok || s == "" {
				continue
			}
			raw = s
		}
		switch col.kind {
		case at5Date:
			d, err := time.Parse("2006-01-02", raw)
			if err != nil {
				c.t.Fatalf("%s: %s is %q, which is not an ISO date: %v", subject, col.input, raw, err)
			}
			c.ins(subject, col.prop, c.rm.NewDateLiteral(rdf.LDate{Date: &d}))
		case at5Double:
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				c.t.Fatalf("%s: %s is %q, which is not a number: %v", subject, col.input, raw, err)
			}
			c.ins(subject, col.prop, c.rm.NewDoubleLiteral(v))
		default:
			c.ins(subject, col.prop, c.rm.NewTextLiteral(raw))
		}
	}
}

// at5Fixture returns the fixture function for one member: their eligibility row
// and every claim of theirs, and nothing of anybody else's.
//
// **One rule session per member, which is what the pipeline does.**
// `copy_member_properties.jr` says so in its own header - *"this rule session is
// for a single member, therefore hc:Eligibility and cintel:Patient_Profile are
// singletons"* - and `readProjection` reads one `am:AnalysisRoot`. A session
// holding two members would produce one briefing over both.
func at5Fixture(p *at5Population, memberID string) func(*testing.T, *rdf.RdfSession, *rdf.ResourceManager) {
	return func(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) {
		t.Helper()
		c := claims(t, s, rm)
		if row, ok := p.eligibility[memberID]; ok {
			subject := "elig-" + memberID
			c.ins(subject, "rdf:type", rm.NewResource("hc:Eligibility"))
			c.ins(subject, "jets:key", rm.NewTextLiteral(subject))
			c.at5AssertRow(subject, row, at5EligibilityColumns)
		}
		for _, row := range p.medical[memberID] {
			subject := at5ClaimSubject("med", row)
			c.ins(subject, "rdf:type", rm.NewResource("hc:MedicalClaim"))
			c.ins(subject, "jets:key", rm.NewTextLiteral(subject))
			c.at5AssertRow(subject, row, at5MedicalColumns)
		}
		for _, row := range p.pharmacy[memberID] {
			subject := at5ClaimSubject("rx", row)
			c.ins(subject, "rdf:type", rm.NewResource("hc:PharmacyClaim"))
			c.ins(subject, "jets:key", rm.NewTextLiteral(subject))
			c.at5AssertRow(subject, row, at5PharmacyColumns)
		}
	}
}

// at5ClaimSubject namespaces the claim id by file. `Claim_ID` is unique within
// each file and the two files' prefixes happen not to collide today; a prefix
// here is one line and removes the dependence on that.
func at5ClaimSubject(kind string, row at5Row) string {
	return kind + "-" + row.value("Claim_ID")
}

// --- the load is right before anything downstream is believed -------------

// **A count over an empty set passes for the wrong reason.** Everything `AT.5`
// reports rests on the population reaching the rule session correctly, and the
// failure mode that matters is silent: a mis-derived mapping produces a session
// that runs, a briefing that is built, and conditions that are not in it.
//
// So this test asserts three things per member that a wrong mapping breaks:
// the briefing's event counts agree with the CSV, the condition summary is
// non-empty for a member whose claims carry a resolvable diagnosis, and the
// joined medication value is present and well formed. It is the gate on the
// side-by-side test beside it.
func TestTheEvaluationPopulationLoadsAsTheMappingSpecifiesIt(t *testing.T) {
	requireCompiledWorkspace(t)
	p := at5LoadPopulation(t)

	// The population as a whole, from the files rather than from a summary.
	if got := len(p.memberIDs); got != at5Members {
		t.Fatalf("the eligibility file holds %d members, expecting %d", got, at5Members)
	}
	if p.medicalRows != at5MedicalRows || p.pharmacyRows != at5PharmacyRows {
		t.Errorf("the claim files hold %d medical and %d pharmacy rows, expecting %d and %d",
			p.medicalRows, p.pharmacyRows, at5MedicalRows, at5PharmacyRows)
	}
	for _, id := range p.memberIDs {
		if len(p.medical[id]) == 0 || len(p.pharmacy[id]) == 0 {
			t.Errorf("%s has %d medical and %d pharmacy claims; the population is documented as "+
				"every member carrying both", id, len(p.medical[id]), len(p.pharmacy[id]))
		}
	}

	// And per member, through the rules.
	membersWithConditions, membersWithMedications := 0, 0
	for _, id := range p.memberIDs {
		t.Run(id, func(t *testing.T) {
			s, rm := ruleSessionOver(t, at5Fixture(p, id))
			pr := readProjection(t, s, rm)

			// (1) The counts. The briefing's own `Medical_Event_Count` is the
			// rule session's arithmetic (`AT.1`(i)); the CSV row count is the
			// input. **They are not the same number and must not be asserted
			// equal**: `AM_PCreateEvent20` indexes pharmacy events by NDC, so
			// three fills of one drug are one event, and a medical event is one
			// claim. What is checkable is the medical identity and the pharmacy
			// bound, and both fail if the load drops a row.
			if got, want := intValue(t, s, rm, pr.briefing, "cintel:Medical_Event_Count"), len(p.medical[id]); got != want {
				t.Errorf("Medical_Event_Count is %d and the member has %d medical claims", got, want)
			}
			rxEvents := intValue(t, s, rm, pr.briefing, "cintel:Pharmacy_Event_Count")
			distinctNDC := map[string]bool{}
			for _, row := range p.pharmacy[id] {
				if v, ok := cleansing_functions.ScrubCharacters(row.value("NDC"), "-").(string); ok {
					distinctNDC[v] = true
				}
			}
			if rxEvents != len(distinctNDC) {
				t.Errorf("Pharmacy_Event_Count is %d and the member's fills carry %d distinct NDCs; "+
					"AM_PCreateEvent20 indexes pharmacy events by NDC", rxEvents, len(distinctNDC))
			}
			// And the briefing's own event lists agree with its counts, which is
			// what makes the two a cross-check rather than a restatement.
			if got := len(pr.briefingMedical); got != len(p.medical[id]) {
				t.Errorf("the briefing carries %d medical events and the member has %d claims",
					got, len(p.medical[id]))
			}
			if got := len(pr.briefingRx); got != rxEvents {
				t.Errorf("the briefing carries %d pharmacy events and Pharmacy_Event_Count is %d",
					got, rxEvents)
			}

			// (2) Conditions. **Not asserted non-empty for every member**, and
			// the reason is I-566 rather than tolerance: `M54.50` and `M54.51`
			// are in 27 of the 301 medical claims and resolve in no lookup, so a
			// member all of whose codes are those two has no condition and is
			// correct. What is asserted is the implication: a member with a
			// resolvable diagnosis on a briefing event has it in the summary.
			summary := values(s, rm, pr.briefing, "cintel:Condition_Summary")
			var flattened []string
			for _, vals := range pr.briefingMedical {
				flattened = append(flattened, vals...)
			}
			if len(flattened) > 0 && len(summary) == 0 {
				t.Errorf("the briefing events carry %d diagnoses and Condition_Summary is empty", len(flattened))
			}
			if len(summary) > 0 {
				membersWithConditions++
			}
			for _, v := range flattened {
				found := false
				for _, w := range summary {
					if v == w {
						found = true
					}
				}
				if !found {
					t.Errorf("diagnosis %q is on a briefing event and not in Condition_Summary", v)
				}
			}

			// (3) The medication. Present, one per event, and opening with its
			// own components - the same property `TestTheMedicationComponentsAreOnTheNodeAndAgree`
			// asserts over the hand-written fixture, here over real fills.
			for ev, names := range pr.briefingRx {
				node := rm.NewResource(ev)
				if len(names) != 1 {
					t.Errorf("briefing pharmacy event %s carries %d drug names, expecting 1", ev, len(names))
					continue
				}
				medication := textValue(t, s, rm, node, "cintel:Medication")
				maintenance := textValue(t, s, rm, node, "cintel:Maintenance")
				if !hasPrefix(medication, names[0]+" (maintenance "+maintenance) {
					t.Errorf("cintel:Medication %q does not open with its own components (%q, %q)",
						medication, names[0], maintenance)
				}
				count := intValue(t, s, rm, node, "cintel:Fill_Count")
				if fills := values(s, rm, node, "cintel:Fill_Date"); count != len(fills) {
					t.Errorf("%s: Fill_Count is %d and the node carries %d Fill_Date values",
						names[0], count, len(fills))
				}
				membersWithMedications++
			}
		})
	}

	// Non-vacuity over the population, so a run in which every member produced
	// an empty briefing fails rather than reporting 22 passes.
	if membersWithConditions < at5Members/2 {
		t.Errorf("only %d of %d members have any condition at all; the diagnosis mapping is "+
			"probably not scrubbing the dot (agentic_ai F789)", membersWithConditions, at5Members)
	}
	if membersWithMedications < at5Members {
		t.Errorf("the population produced %d pharmacy events across %d members, which is fewer "+
			"than one each", membersWithMedications, at5Members)
	}
}
