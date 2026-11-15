package compute_pipes

import (
	"fmt"
	"log"
)

// # Checking a staged file's columns against the channel that reads it
//
// A stage file is csv read by position: its header line records the columns it
// was written with, and the reader skips that line and gives the values the
// columns of its own channel spec. When a channel reads the stage through
// file_key, the files may have been written by another run, before a change to
// the class behind the channel. A property added to the class then shifts every
// column after it, and the values land in the wrong columns: a text value in a
// number column fails a cast, and a text value in a text column passes unnoticed.
//
// So such a read compares each file's header line with the columns of its
// channel spec, position by position, and refuses the file at the first column
// whose name differs, naming the file and the column. A file that only lacks
// trailing columns (a property appended to the class), or carries extra trailing
// ones, reads every shared column at its place: it is logged and read, as before.
//
// The check applies to a csv channel of type stage with a file_key, main input or
// merge channel, whose channel spec lists its columns (not same_columns_as_input
// nor has_dynamic_columns). A read of this session's own steps (read_step_id) is
// written by the same run with the same specs and is not checked.

// stageColumnsToCheck returns the columns a file of the channel must have been
// written with, or nil when the channel's files are not checked.
func stageColumnsToCheck(ic *InputChannelConfig, spec *ChannelSpec) []string {
	if ic == nil || spec == nil || !readsStageByFileKey(ic) || ic.Format != "csv" {
		return nil
	}
	if spec.SameColumnsAsInput || spec.HasDynamicColumns {
		return nil
	}
	return spec.Columns
}

// checkStageFileColumns compares the header line of a staged file with the
// columns its channel reads, as described above.
func checkStageFileColumns(key, channel string, header, expected []string) error {
	n := min(len(header), len(expected))
	for i := range n {
		if header[i] != expected[i] {
			return fmt.Errorf(
				"stage file %s was written with other columns than channel %s reads: column %d is '%s' in the file "+
					"and '%s' in the channel (file has %d columns, channel %d). The file was likely written before a "+
					"change to the channel's class: delete its session, or map it again, before reading it",
				key, channel, i+1, header[i], expected[i], len(header), len(expected))
		}
	}
	if len(header) != len(expected) {
		log.Printf("Stage file %s has %d columns and channel %s reads %d; the first %d agree, so the file is read "+
			"(missing trailing columns are empty, extra ones are ignored)", key, len(header), channel, len(expected), n)
	}
	return nil
}
