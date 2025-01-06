package compute_pipes

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// Utility function to access s3

func GetS3FileKeys(processName, sessionId, mainInputStepId, jetsPartitionLabel string) ([]*FileKeyInfo, error) {
	s3BaseFolder := fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/jets_partition=%s",
		jetsS3StagePrefix, processName, sessionId, mainInputStepId, jetsPartitionLabel)

	s3Objects, err := awsi.ListS3Objects("", &s3BaseFolder)
	if err != nil || s3Objects == nil {
		return nil, fmt.Errorf("failed to download list of files from s3: %v", err)
	}
	fileKeys := make([]*FileKeyInfo, 0)
	for i := range s3Objects {
		if s3Objects[i].Size > 0 {
			fileKeys = append(fileKeys,
				&FileKeyInfo{key: s3Objects[i].Key, size: int(s3Objects[i].Size)})
		}
	}
	return fileKeys, nil
}
