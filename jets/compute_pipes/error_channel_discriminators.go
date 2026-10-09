package compute_pipes

// The discriminator columns of jetsapi.process_errors, and the configuration check
// that reports an error channel which cannot carry them.
//
// The table gained cpipes_step_id, error_channel and operator_type so that an error
// row can name the operator, the channel and the step that produced it. The row is
// assembled against the *channel's* declared columns, not against the table's, so a
// configuration whose error channel spec predates the widening writes NULL for all
// three and nothing anywhere says so. This check is what says so.
//
// It warns rather than failing. Ten of the corpus's forty-seven pipeline
// configurations wire an error channel and none of them declares these columns yet;
// failing would refuse ten live configurations for a diagnostic they never had.

import (
	"log"
	"strings"
)

// validateErrorChannelSpecs runs the single-writer rule of validateErrorChannels and
// then reports, without failing, any error channel whose spec cannot carry the
// discriminators. It is the whole of what ValidatePipeSpecConfig owes the error
// channels, so the call site stays one line.
func validateErrorChannelSpecs(cpConfig *ComputePipesConfig, pipeConfig []PipeSpec) error {
	if err := validateErrorChannels(pipeConfig); err != nil {
		return err
	}
	warnMissingErrorChannelDiscriminators(cpConfig, pipeConfig)
	return nil
}

// warnMissingErrorChannelDiscriminators logs one line per error channel whose
// channel spec omits any of ProcessErrorDiscriminatorColumns. A channel spec that
// is not found is skipped rather than reported: whether a named spec exists is
// validateOutputChConfig's question and it has already been asked.
func warnMissingErrorChannelDiscriminators(cpConfig *ComputePipesConfig, pipeConfig []PipeSpec) {
	if cpConfig == nil {
		return
	}
	reported := make(map[string]bool)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			transformationConfig := &pipeConfig[i].Apply[j]
			errorChannel := errorChannelConfig(transformationConfig)
			if errorChannel == nil || len(errorChannel.Name) == 0 || reported[errorChannel.Name] {
				continue
			}
			specName := errorChannel.SpecName
			if len(specName) == 0 {
				specName = errorChannel.Name
			}
			spec := cpConfig.GetChannelSpec(specName)
			if spec == nil {
				continue
			}
			missing := missingDiscriminatorColumns(spec)
			if len(missing) == 0 {
				continue
			}
			reported[errorChannel.Name] = true
			log.Printf(
				"warning: error channel '%s' of operator '%s' uses channel spec '%s', which does not declare %s; "+
					"error rows from this operator will carry no step, channel or operator on jetsapi.process_errors",
				errorChannel.Name, transformationConfig.Type, specName, strings.Join(missing, ", "))
		}
	}
}

// missingDiscriminatorColumns returns the discriminator columns the spec does not
// declare, in the order they are declared in ProcessErrorDiscriminatorColumns.
func missingDiscriminatorColumns(spec *ChannelSpec) []string {
	declared := make(map[string]bool, len(spec.Columns))
	for _, c := range spec.Columns {
		declared[c] = true
	}
	missing := make([]string, 0, len(ProcessErrorDiscriminatorColumns))
	for _, c := range ProcessErrorDiscriminatorColumns {
		if !declared[c] {
			missing = append(missing, c)
		}
	}
	return missing
}
