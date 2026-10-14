package compute_pipes

import (
	"database/sql"
	"fmt"
	"log"
)

// The keys a jetrules error row carries: grouping_key, the domain key of the
// rete session's input, and row_jets_key, the jets:key of the record the row is
// about. Until 2026-10-13 no jetrules site set either, so a rule error could be
// traced to its group only through the subject named in the message, which is
// the output entity rather than the group.

// jetrulesGroupingDomainKey names the domain key whose value is the grouping key
// of the rete sessions fed from source, or "" when there is none, and says why.
//
// A worker task carries only the group's rows, so the key is computed from the
// first row with the domain key the group was made by, rather than passed with
// the task. That domain key is read from the configuration:
//
//   - the group_by or merge operator writing source, when one is in this step: its
//     domain_key, or none when it groups by column names, positions or count,
//     since the group then has no domain key;
//   - otherwise, when source's class has exactly one domain key, that one: the
//     session holds one input row and its domain key is the session's;
//   - otherwise none, rather than a guess between several.
func jetrulesGroupingDomainKey(cpConfig *ComputePipesConfig, source *InputChannel) (string, string) {
	if cpConfig != nil {
		for i := range cpConfig.PipesConfig {
			for j := range cpConfig.PipesConfig[i].Apply {
				spec := &cpConfig.PipesConfig[i].Apply[j]
				if spec.OutputChannel.Name != source.Name {
					continue
				}
				switch {
				case spec.Type == "group_by" && spec.GroupByConfig != nil:
					if dk := spec.GroupByConfig.DomainKey; dk != "" {
						return dk, fmt.Sprintf("the domain key of the group_by writing %s", source.Name)
					}
					return "", fmt.Sprintf("the group_by writing %s does not group by a domain key", source.Name)
				case spec.Type == "merge" && spec.MergeConfig != nil:
					if dk := spec.MergeConfig.MainGroupBy.DomainKey; dk != "" {
						return dk, fmt.Sprintf("the domain key of the merge writing %s", source.Name)
					}
					return "", fmt.Sprintf("the merge writing %s does not group by a domain key", source.Name)
				}
			}
		}
	}
	if source.DomainKeySpec != nil && len(source.DomainKeySpec.DomainKeys) == 1 {
		for dk := range source.DomainKeySpec.DomainKeys {
			return dk, fmt.Sprintf("the only domain key of %s", source.Name)
		}
	}
	return "", fmt.Sprintf("%s is not written by a group_by or merge on a domain key, and does not have exactly one domain key", source.Name)
}

// newJetrulesGroupingKey builds the evaluator of the grouping key of the rete
// sessions fed from source: the domain key named by jetrulesGroupingDomainKey,
// computed as the hash operator's compute_domain_key computes it, so the value is
// the one in the output tables' domain key column. It returns nil when there is
// no such domain key or it cannot be computed on source; error rows then carry no
// grouping key, and nothing else changes.
func (ctx *BuilderContext) newJetrulesGroupingKey(source *InputChannel) *HashEvaluator {
	dk, why := jetrulesGroupingDomainKey(ctx.cpConfig, source)
	if dk == "" {
		log.Printf("jetrules: error rows will carry no grouping_key: %s", why)
		return nil
	}
	evaluator, err := ctx.NewHashEvaluator(source, &HashExpression{DomainKey: dk, ComputeDomainKey: true})
	if err != nil {
		log.Printf("jetrules: error rows will carry no grouping_key: domain key %s (%s) cannot be computed on %s: %v",
			dk, why, source.Name, err)
		return nil
	}
	log.Printf("jetrules: error rows carry the grouping_key of domain key %s, %s", dk, why)
	return evaluator
}

// inputRowJetsKey is the jets:key assertInputRow gives an input row: the row's
// own, in position 0, or else a hash of its values.
func inputRowJetsKey(config *JetrulesSpec, row []any) string {
	if len(row) == 0 {
		return ""
	}
	if jetsKey, ok := row[0].(string); ok {
		return jetsKey
	}
	if len(row) < 3 {
		return ""
	}
	return ComputeRowHash(row[3:], config.CurrentSourcePeriod)
}

// setSessionKeys records the keys of the rete session about to run on
// inputRecords, which every error row of the session carries: the group's domain
// key, and the input row's jets:key when the session has one input row. A
// session over several rows leaves row_jets_key to the sites that know which
// record the error is about.
func (ctx *JrPoolWorker) setSessionKeys(inputRecords *[]any) {
	ctx.sessionGroupingKey = sql.NullString{}
	ctx.sessionRowJetsKey = sql.NullString{}
	if inputRecords == nil || len(*inputRecords) == 0 {
		return
	}
	firstRow := *inputRecords
	nrows := 1
	if ctx.source.HasGroupedRows {
		row, ok := (*inputRecords)[0].([]any)
		if !ok {
			return
		}
		firstRow = row
		nrows = len(*inputRecords)
	}
	if nrows == 1 {
		if key := inputRowJetsKey(ctx.config, firstRow); key != "" {
			ctx.sessionRowJetsKey = sql.NullString{String: key, Valid: true}
		}
	}
	if ctx.groupingKey == nil {
		return
	}
	value, err := ctx.groupingKey.ComputeDomainKey(firstRow)
	if err != nil || value == nil {
		if ctx.config.IsDebug {
			log.Printf("jetrules: grouping key not computed for the session: %v", err)
		}
		return
	}
	if key := fmt.Sprintf("%v", value); key != "" {
		ctx.sessionGroupingKey = sql.NullString{String: key, Valid: true}
	}
}

// newProcessError is the error row of the current rete session, carrying its keys.
func (ctx *JrPoolWorker) newProcessError() ProcessError {
	peRow := ctx.builderContext.NewProcessError("jetrules")
	peRow.GroupingKey = ctx.sessionGroupingKey
	peRow.RowJetsKey = ctx.sessionRowJetsKey
	return peRow
}

// subjectJetsKey is the key of an extracted entity for row_jets_key: its jets:key,
// which is the value the output table's jets:key column holds, or its resource
// name when it has none. The two are the same for an input entity and for one
// made by create_entity; an entity made by create_uuid_resource or
// create_resource has no jets:key.
func subjectJetsKey(rdfSession JetRdfSession, subject RdfNode) string {
	jr := rdfSession.JetResources()
	if obj := rdfSession.GetObject(subject, jr.Jets__key); obj != nil {
		if key, ok := GetRdfNodeValue(obj).(string); ok && key != "" {
			return key
		}
	}
	return subject.String()
}
