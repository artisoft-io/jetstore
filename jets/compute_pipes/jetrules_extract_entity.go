package compute_pipes

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/prose"
	togo "github.com/toon-format/toon-go"
)

func (ce *JrSpecialColumnEncoding) EncodeColumnData(rdfSession JetRdfSession, subject RdfNode) any {
	if ce.Config == nil {
		log.Panicf("bug: JrSpecialColumnEncoding.Config is nil")
	}
	// For toon and json encoding, we extract the entire object as a map[string]any
	// log.Printf("*** Extracting json/toon obj - start")
	entityObj := make(map[string]any)
	extractAsEntity(rdfSession, ce.Config.RemoveModelPrefixes, subject, entityObj, ce.ExcludeProperties)
	// log.Printf("*** Extracting json/toon obj - end")
	if ce.Config.EntityEncoding == "briefing_prose" {
		// The deterministic briefing (agentic_ai Phase 7 AT.3): the same entity
		// map the toon and json arms encode, rendered as the briefing itself.
		//
		// **It hangs here rather than beside the operator on purpose.** Plan
		// §1.10.8's recommendation is "render from extractAsEntity's map with an
		// empty exclusion set - one function, two callers, and the exclusion list
		// the only difference between what the template sees and what the model
		// sees", which is what criterion 76 needs: the template arm and the model
		// arm must differ in the renderer and in nothing else. A second
		// extraction path would be a second thing to keep in step, and this
		// repository has measured what a second encoder costs (P2 M.5).
		//
		// Two settings a column carrying this encoding needs, neither of which
		// this function can impose: remove_model_prefixes must be true, which
		// prose.Render refuses by name when it is not; and the exclusion list
		// must **not** carry cintel:Briefing_Disclaimer, which the toon column
		// does exclude - the notice is out of the prompt and is the first thing
		// in the artefact.
		text, err := prose.Render(entityObj)
		if err != nil {
			err = fmt.Errorf("error: failed to render the briefing prose for subject %s: %v", subject, err)
			log.Println(err)
			return err
		}
		return text
	}
	if ce.Config.EntityEncoding == "toon" {
		// For toon encoding, we need to convert the map to a toon string
		toonBytes, err := togo.Marshal(entityObj, togo.WithTimeFormatter(func(t time.Time) string {
			switch {
			case t.IsZero():
				return ""
			case t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0:
				return t.Format("2006-01-02")
			default:
				return t.Format("2006-01-02T15:04:05")
			}
		}))
		if err != nil {
			err = fmt.Errorf("error: failed to marshal entity object to toon for subject %s: %v", subject, err)
			log.Println(err)
			return err
		}
		// log.Printf("*** toon encoded obj:\n%s", string(toonBytes))
		return string(toonBytes)
	} else {
		// For json encoding, we need to convert the map to a json string
		jsonBytes, err := json.Marshal(entityObj)
		if err != nil {
			err = fmt.Errorf("error: failed to marshal entity object to json for subject %s: %v", subject, err)
			log.Println(err)
			return err
		}
		// log.Printf("*** json encoded obj:\n%s", string(jsonBytes))
		return string(jsonBytes)
	}
}

// Navigate recursively the object properties and extract their values into a map[string]any
// excluding the properties starting with _0:
func extractAsEntity(rdfSession JetRdfSession, removeModelPrefixes bool, subject RdfNode,
	entityObj map[string]any, excludeProp map[string]bool) {

	var objProperties map[string]RdfNode
	itor := rdfSession.FindS(subject)
	defer itor.Release()
	for !itor.IsEnd() {
		// log.Printf("*** Triple (%s, %s (%s), %s)", itor.GetSubject(), itor.GetPredicate(), itor.GetPredicate().Type(), itor.GetObject())
		prop := itor.GetPredicate()
		if strings.HasPrefix(prop.String(), "_0:") || prop.Type() != "named_resource" || excludeProp[prop.String()] {
			itor.Next()
			continue
		}
		// Check if it's an obj property
		if isEntity(rdfSession, itor.GetObject()) {
			if objProperties == nil {
				objProperties = make(map[string]RdfNode)
			}
			objProperties[prop.String()] = prop
		} else {
			// It's a literal property, extract its value.
			//
			// GetRdfNodeValue rather than Value(): a node's raw Value for a date
			// is an rdf.LDate and for a datetime an rdf.LDatetime, both structs
			// with one exported field, so json and toon marshalled them as a
			// nested object - `"Service_Date":{"Date":"2025-08-14T00:00:00Z"}` -
			// and the WithTimeFormatter this function passes to togo.Marshal
			// never fired, because it matches time.Time and the value was never
			// a time.Time. The ordinary column path beside this one has always
			// unwrapped (`extractLiteralValue`,
			// `jets/compute_pipes/jetrules_pool_worker.go:434`), so a date was a
			// date in a column and a nested object in the prompt.
			// Found 2026-09-05 by agentic_ai's AK.2, whose provenance rule over a
			// service date could not read one.
			addToEntityObj(entityObj, removeModelPrefixes, prop.String(), GetRdfNodeValue(itor.GetObject()))
		}
		itor.Next()
	}
	// extract the object properties recursively
	for prop, node := range objProperties {
		jtor := rdfSession.FindSP(subject, node)
		for !jtor.IsEnd() {
			subEntityObj := make(map[string]any)
			addToEntityObj(entityObj, removeModelPrefixes, prop, subEntityObj)
			extractAsEntity(rdfSession, removeModelPrefixes, jtor.GetObject(), subEntityObj, excludeProp)
			jtor.Next()
		}
	}
	// Order every list this level produced, AFTER the recursion above.
	//
	// See sortEntityLists. The placement is the whole of the children-before-
	// parents property: each recursive call sorts its own sub-entity's lists
	// before returning, so when this call computes an ordering key over a
	// sub-entity map, that map is already in its final form. Sorting before the
	// recursion, or in EncodeColumnData over the finished root map alone, would
	// key a parent list on children whose own order had not settled.
	sortEntityLists(entityObj)
}

// sortEntityLists puts a total, content-derived order on every list in one
// entity map.
//
// # WHY THIS EXISTS: THE PROMPT WAS NOT REPRODUCIBLE
//
// A jetrules multi-valued property is a *set*. It has no order, and
// extractAsEntity above builds its lists by appending in the order the graph's
// iterators happen to drain - which is a range over a `sync.Map` (`WSetType`,
// `jets/jetrules/rdf/base_graph.go:26`) and therefore a function of the
// process's map hash seed rather than of the data.
//
// That was measured rather than assumed, by agentic_ai's Phase 7 (plan §1.21.1,
// F801): **200 encodings of one unchanged rule session inside one process gave
// exactly one ordering; the same binary run as twelve separate processes gave
// five of the six possible orderings of three conditions, one per process.**
// A cpipes worker is a process, so a run is internally consistent - nothing
// looks wrong - and the next run over identical data produces a different
// prompt. Downstream (F836, F837) the TOON prompt differed byte for byte in 22
// of 22 members between two runs and as a multiset of lines in 16 of 22, while
// the deterministic template arm was byte-identical in only 4 of 22 and
// identical as a bag of words in 22 of 22 - its variation was list order and
// nothing else.
//
// # THE RULE: LEXICOGRAPHIC ON A CANONICAL RENDERING OF THE VALUE
//
// This is `join_values`' rule one level up. That operator folds a multi-valued
// property into one string and sorts the rendered values byte-wise ascending,
// on exactly this argument - see the header of
// `jets/jetrules/rete/expr_operator_math_join_values.go`, "THE OUTPUT IS SORTED,
// AND THE REASON IS REPRODUCIBILITY RATHER THAN TASTE". Since a set has no
// order, any total order is as defensible as any other, and the one already in
// the codebase wins on consistency alone: the dates *inside* a joined
// medication value were sorted while the list of events carrying them was not,
// which is one repository holding two answers to one question.
//
// The canonical rendering is the element's JSON encoding, because `json.Marshal`
// sorts map keys and so gives two sub-entity maps with the same content the same
// key regardless of how either was built. ISO-8601 dates encode as RFC 3339 and
// therefore sort lexically into chronological order, which is the same happy
// accident join_values relies on.
//
// # THE KEY IS TOTAL, AND WHAT THAT BUYS
//
// The key is the canonical JSON followed by the element's dynamic Go type, so
// two elements compare equal only when they are indistinguishable in content
// *and* in type - and two such elements serialise identically in every arm
// (toon, json and the prose template), so which of them is placed first cannot
// change any output. sort.SliceStable is used rather than sort.Slice for the
// residual case: it costs nothing at these sizes and it means a tie is broken
// by the input order rather than by pdqsort's pivot choice, which keeps the
// function's behaviour explainable even where its result is provably
// unobservable.
//
// # WHAT WAS REJECTED
//
//   - **Sorting in the renderers.** The model arm reads the TOON and the
//     template arm reads this same map (`Render`,
//     `jets/agentic/briefing/prose/prose.go:125`). Ordering one alone would make
//     the two arms differ in something other than the renderer, which is exactly
//     what agentic_ai's criterion 76 forbids. One function, one place, both
//     consumers.
//   - **Ordering the graph container itself.** It would fix every consumer at
//     once, and it asserts an order the rdf data model does not have, changes a
//     RETE-engine container shared with rule evaluation, and would have to be
//     mirrored in the C++ engine to keep the two in step. Far more than the
//     defect needs.
//   - **Preserving insertion order.** That trades a hash seed for rule-firing
//     order, which is no more a specified property of the data than the seed is,
//     and it needs the same container change.
//   - **Sorting scalars only.** The event lists are sub-entity maps, and they
//     are where nearly all of the observed prompt variation lived.
//   - **Hashing the canonical form.** Total and cheap, and it produces an order
//     no reader can predict. Lexicographic on the rendering gives chronological
//     dates and alphabetical drug names for free, which is legible in a prompt.
//   - **Sorting the map keys.** Not this function's business and not needed:
//     `encoding/json` sorts them, and so does `togo.Marshal` (`slices.SortFunc`
//     over the object's fields, toon-go `internal/codec/normalize.go`). Measured
//     in TestMapKeyOrderIsNotASecondSourceOfVariation.
func sortEntityLists(entityObj map[string]any) {
	for _, v := range entityObj {
		list, ok := v.([]any)
		if !ok || len(list) < 2 {
			continue
		}
		keyed := make([]struct {
			key string
			val any
		}, len(list))
		for i := range list {
			keyed[i].key = orderingKey(list[i])
			keyed[i].val = list[i]
		}
		sort.SliceStable(keyed, func(a, b int) bool { return keyed[a].key < keyed[b].key })
		for i := range keyed {
			list[i] = keyed[i].val
		}
	}
}

// orderingKey renders one list element as the string sortEntityLists orders on.
//
// json.Marshal is the canonical form: it sorts map keys, it renders a time.Time
// as RFC 3339 (so lexical order is chronological), and it is the encoding one of
// the two shipped arms actually emits. It can fail - a NaN or an Inf double
// reaches this map as a float64 and is not representable in JSON - so the
// fallback is fmt's %v, which is likewise deterministic (fmt sorts map keys too)
// and total. A panic or a zero key here would be worse than an unlovely
// ordering, because the caller is the encoder for every entity the pipeline
// emits.
//
// The dynamic type is appended so that values which render alike and are not
// alike - int(1) against float64(1), say - do not compare equal. Beyond that the
// key is deliberately not injective and does not need to be: elements with an
// equal key have equal content and equal type, so no arm can tell which one came
// first.
func orderingKey(v any) string {
	var rendered string
	if b, err := json.Marshal(v); err == nil {
		rendered = string(b)
	} else {
		rendered = fmt.Sprintf("%v", v)
	}
	return rendered + "\x00" + fmt.Sprintf("%T", v)
}

func isEntity(rdfSession JetRdfSession, node RdfNode) bool {
	itor := rdfSession.FindS(node)
	defer itor.Release()
	return !itor.IsEnd()
}

func addToEntityObj(entityObj map[string]any, removeModelPrefixes bool, prop string, value any) {
	if value == nil {
		return
	}
	if removeModelPrefixes {
		// Remove model prefixes from the property name, e.g. remove the start of prop upto the char : if present, e.g. jets: or rdf:
		if idx := strings.Index(prop, ":"); idx != -1 {
			prop = prop[idx+1:]
		}
	}
	if existing, ok := entityObj[prop]; ok {
		// If existing is any, then create a slice to hold current and existing values
		// If existing is []any then add to it
		switch existingVal := existing.(type) {
		case []any:
			existingVal = append(existingVal, value)
			entityObj[prop] = existingVal
		case nil:
			entityObj[prop] = value
		default:
			entityObj[prop] = []any{existingVal, value}
		}
	} else {
		entityObj[prop] = value
	}
}
