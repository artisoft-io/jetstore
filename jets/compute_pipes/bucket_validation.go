package compute_pipes

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/artisoft-io/jetstore/jets/utils"
)

// The startup gate for unresolved buckets.
//
// bucket_resolution.go answers "what does this bucket name name" for one string.
// This file asks that question of every bucket a step is about to write to,
// once, before the step's workers are dispatched.
//
// The failure it exists to prevent has no other signal. A document naming
// ${CORPUS_OUT_BUCKET} where the environment supplies no such variable is
// carried through substitution unchanged, reaches the writer as an external
// bucket name, and the run *succeeds* -- into a bucket spelled with a dollar
// sign in it. Nothing logs a warning, because nothing between the document and
// the S3 call has any opinion about the name. The cost is paid later and
// elsewhere, by whoever goes looking for the deliverables.
//
// So the gate is a refusal rather than a warning: a warning is read by whoever
// happens to be watching a log, and the thing being guarded is discovered by an
// absence somewhere else.

// nodeScopedEnvKeys are the env keys that exist at the worker and not at
// startup: actions_coordinate_cp.go assigns them per node, after the startup
// path has already persisted the env. Substituting against a startup env alone
// would leave a bucket naming one of them looking unresolved when it is not, so
// the check seeds a placeholder for each.
//
// They are seeded rather than special-cased in IsUnresolvedBucket because the
// property being asserted is "the environment supplies a value for this key by
// the time the name is used", and for these two it does.
var nodeScopedEnvKeys = []string{"$SHARD_ID", "$JETS_PARTITION_LABEL"}

// bucketSite is one bucket name found in the document, with the dotted path
// that locates it. The path is the whole value of the diagnostic: "a bucket is
// unresolved" is not actionable in a document with twelve output channels.
type bucketSite struct {
	path  string
	value string
}

// ValidateResolvedBuckets refuses a step whose document names a bucket that
// variable substitution does not reach.
//
// It is called from both startup paths, after ValidatePipeSpecConfig has
// normalised the channels and before either path assembles the config its
// workers run. It is deliberately *not* called from ValidatePipeSpecConfig
// itself: that function is a statement about the shape of a document and is
// used as one by jets/agentic/tools' validate_cpipes_config, which supplies a
// synthetic env. Whether ${CORPUS_OUT_BUCKET} resolves is a fact about a
// deployment and not about a document, and a static validator that answered it
// would answer it wrongly.
//
// Scope, stated because an incomplete check that reads as complete is worse
// than a narrow one that says so:
//
//   - The buckets checked are the document-level schema_providers and
//     output_files, plus every bucket reachable from the step about to run.
//   - Steps other than that one are not checked here. A conditional step's
//     addl_env is applied only when that step is selected (GetComputePipes),
//     so a later step's bucket may legitimately be unresolvable now. Every
//     step re-enters this path when it starts, so every step is checked
//     before its own workers run.
//   - src_bucket and dest_bucket on a schema provider's report_cmds are not
//     checked. They are not on this path at all: RunSchemaProviderReportsCmds
//     hands them to awsi.MultiPartCopy verbatim, with no substitution
//     anywhere, so the check they want is a different and stricter one living
//     in jets/run_reports.
func ValidateResolvedBuckets(cpConfig *ComputePipesConfig, pipeConfig []PipeSpec, env map[string]any) error {
	if cpConfig == nil {
		return nil
	}
	sites := make([]bucketSite, 0, 8)
	collectBucketSites("schema_providers", reflect.ValueOf(cpConfig.SchemaProviders), 0, &sites)
	collectBucketSites("output_files", reflect.ValueOf(cpConfig.OutputFiles), 0, &sites)
	collectBucketSites("pipes_config", reflect.ValueOf(pipeConfig), 0, &sites)
	if len(sites) == 0 {
		return nil
	}

	checkEnv := make(map[string]any, len(env)+len(nodeScopedEnvKeys))
	for k, v := range env {
		checkEnv[k] = v
	}
	for _, k := range nodeScopedEnvKeys {
		if checkEnv[k] == nil {
			checkEnv[k] = "0"
		}
	}

	unresolved := make([]string, 0)
	for _, site := range sites {
		resolved := utils.ReplaceEnvVars(site.value, checkEnv)
		if _, err := ClassifyBucket(resolved); err != nil {
			switch {
			case resolved == site.value:
				unresolved = append(unresolved, fmt.Sprintf("%s = %q", site.path, site.value))
			default:
				unresolved = append(unresolved,
					fmt.Sprintf("%s = %q (substituted to %q)", site.path, site.value, resolved))
			}
		}
	}
	if len(unresolved) == 0 {
		return nil
	}
	return fmt.Errorf(
		"configuration error: %d bucket name(s) still carry an unresolved variable reference, "+
			"the environment supplies no value for them and the run would write to a bucket "+
			"literally so named: %s",
		len(unresolved), strings.Join(unresolved, "; "))
}

// maxBucketWalkDepth bounds the walk. The config is a tree decoded from JSON, so
// it cannot be cyclic, but SynthesizeDefaultErrorChannels and the schema
// provider sync share pointers between nodes and a bound costs nothing.
const maxBucketWalkDepth = 40

// collectBucketSites walks a decoded configuration value and records every
// string field whose json key is exactly "bucket".
//
// Matching on the json key rather than on an enumerated list of types is what
// makes this complete and keeps it complete: FileConfig's Bucket reaches
// SchemaProviderSpec, InputChannelConfig, OutputChannelConfig and
// OutputFileSpec by embedding, ColumnFileSpec declares its own, and a bucket
// field added to a new spec tomorrow is checked without this file changing.
//
// "bucket" exactly, so S3CopyFileSpec's src_bucket and dest_bucket are outside
// it -- see ValidateResolvedBuckets' scope note.
func collectBucketSites(path string, v reflect.Value, depth int, out *[]bucketSite) {
	if depth > maxBucketWalkDepth || !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return
		}
		collectBucketSites(path, v.Elem(), depth+1, out)
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			collectBucketSites(fmt.Sprintf("%s[%d]", path, i), v.Index(i), depth+1, out)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			collectBucketSites(fmt.Sprintf("%s[%v]", path, iter.Key()), iter.Value(), depth+1, out)
		}
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			sf := t.Field(i)
			// Unexported fields are skipped rather than read: the one that
			// matters is InputChannelConfig.schemaProviderConfig, a back
			// pointer the sync installs, and following it would walk the
			// schema providers a second time under a misleading path.
			if sf.PkgPath != "" {
				continue
			}
			name := jsonKeyOf(sf)
			if name == "-" {
				continue
			}
			childPath := path
			if !sf.Anonymous {
				// An embedded struct flattens into its host in JSON, so it
				// contributes no path segment.
				childPath = path + "." + name
			}
			field := v.Field(i)
			if name == "bucket" && field.Kind() == reflect.String {
				if s := field.String(); len(s) > 0 {
					*out = append(*out, bucketSite{path: childPath, value: s})
				}
				continue
			}
			collectBucketSites(childPath, field, depth+1, out)
		}
	}
}

// jsonKeyOf returns the json key a struct field is encoded under, falling back
// to the Go field name when the tag is absent.
func jsonKeyOf(sf reflect.StructField) string {
	tag, ok := sf.Tag.Lookup("json")
	if !ok {
		return sf.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	switch name {
	case "":
		return sf.Name
	default:
		return name
	}
}
