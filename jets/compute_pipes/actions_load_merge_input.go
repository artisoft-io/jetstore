package compute_pipes

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
)

func (cpCtx *ComputePipesContext) loadMergeInput(computePipesInputCh chan []any,
	inputChannelConfig *InputChannelConfig, fileNamesCh chan FileName) (err error) {

	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			fmt.Fprintf(&buf, "loadMergeInput: recovered error: %v\n", r)
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
		}
		close(computePipesInputCh)
	}()

	// Load the merge input files
	if inputChannelConfig == nil || inputChannelConfig.Type != "stage" {
		err = fmt.Errorf("unexpected error: invalid input channel config for loadMergeInput, must be type 'stage', got: %+v", inputChannelConfig)
		log.Println(err)
		cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
		return
	}
	channelInfo := GetChannelSpec(cpCtx.CpConfig.Channels, inputChannelConfig.Name)
	if channelInfo == nil {
		err = fmt.Errorf("unexpected error: Channel info not found for channel '%s'", inputChannelConfig.Name)
		log.Println(err)
		cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
		return
	}
	inputDomainClass := channelInfo.ClassName

	var castToRdfTxtTypeFncs []CastToRdfTxtFnc
	if len(inputDomainClass) > 0 {
		castToRdfTxtTypeFncs, err = BuildCastToRdfTxtFunctions(inputDomainClass, channelInfo.Columns)
		if err != nil {
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
			return
		}
	}

	samplingMaxCount := int64(inputChannelConfig.SamplingMaxCount)
	var count, totalRowCount, badRowCount, totalBadRowCount int64
	inputFormat := inputChannelConfig.Format
	gotMaxRecordCount := false

	for localInFile := range fileNamesCh {
		if gotMaxRecordCount {
			// Don't read more records
			os.Remove(localInFile.LocalFileName)
			continue
		}
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d Loading merge file '%s'", cpCtx.SessionId, cpCtx.NodeId, localInFile.InFileKeyInfo.key)
		}
		// Encapsulte the switch so to factor out file handling
		err = func() error {
			var err error
			var fileHd *os.File
			fileHd, err = os.Open(localInFile.LocalFileName)
			if err != nil {
				err = fmt.Errorf("while opening temp file '%s' (LoadFiles): %v", localInFile.LocalFileName, err)
				log.Println(err)
				cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount, Err: err}
				return err
			}
			defer func() {
				fileHd.Close()
				os.Remove(localInFile.LocalFileName)
			}()

			switch inputFormat {
			case "csv", "headerless_csv":
				count, badRowCount, err = cpCtx.ReadCsvFile(
					&localInFile, fileHd, castToRdfTxtTypeFncs, computePipesInputCh, nil)

			default:
				err = fmt.Errorf("%s node %d, error: unsupported file format: %s", cpCtx.SessionId, cpCtx.NodeId, inputFormat)
				log.Println(err)
				cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount, Err: err}
				return err
			}
			return nil
		}()

		totalRowCount += count
		totalBadRowCount += badRowCount
		if err != nil {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "LoadMergeFile returned error", err)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount, Err: err}
			return
		}
		if samplingMaxCount > 0 && totalRowCount >= samplingMaxCount {
			gotMaxRecordCount = true
		}
	}
	cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount}
	return
}
