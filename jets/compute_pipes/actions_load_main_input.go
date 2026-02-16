package compute_pipes

import (
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

func (cpCtx *ComputePipesContext) loadMainInput(computePipesInputCh chan []any,
	inputChannelConfig *InputChannelConfig, inputSchemaCh chan ParquetSchemaInfo) (err error) {

	defer close(computePipesInputCh)

	// Start BadRow Channel if configured
	var badRowChannel *BadRowsChannel
	if inputChannelConfig.BadRowsConfig != nil {
		// s3 partitioning, write the partition files in the JetStore's stage path defined by the env var JETS_s3_STAGE_PREFIX
		// baseOutputPath structure is: <JETS_s3_STAGE_PREFIX>/process_name=QcProcess/session_id=123456789/step_id=reduce01/jets_partition=22p/
		// NOTE: All partitions for bad rows are written to partion '0000P' so we can use merge_files operator
		//       (otherwise use cpCtx.JetsPartitionLabel so save in current partition)
		baseOutputPath := fmt.Sprintf("%s/process_name=%s/session_id=%s/step_id=%s/jets_partition=%s",
			awsi.JetStoreStagePrefix(), cpCtx.ProcessName, cpCtx.SessionId, inputChannelConfig.BadRowsConfig.BadRowsStepId, "0000P")

		badRowChannel = NewBadRowChannel(cpCtx.S3DeviceMgr, baseOutputPath, cpCtx.Done, cpCtx.ErrCh)
		defer badRowChannel.Done()
		go badRowChannel.Write(cpCtx.NodeId)
	}

	// Load the main input files
	var mainInputDomainClass string
	if inputChannelConfig.Name == "input_row" {
		mainInputDomainClass = cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.DomainClass
	} else {
		channelInfo := GetChannelSpec(cpCtx.CpConfig.Channels, inputChannelConfig.Name)
		if channelInfo == nil {
			err = fmt.Errorf("unexpected error: Channel info not found for channel '%s'", inputChannelConfig.Name)
			log.Println(err)
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
			return
		}
		mainInputDomainClass = channelInfo.ClassName
	}

	var castToRdfTxtTypeFncs []CastToRdfTxtFnc
	if len(mainInputDomainClass) > 0 {
		castToRdfTxtTypeFncs, err = BuildCastToRdfTxtFunctions(mainInputDomainClass,
			cpCtx.CpConfig.CommonRuntimeArgs.SourcesConfig.MainInput.InputColumns)
		if err != nil {
			cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
			return
		}
	}

	// Get the FixedWidthEncodingInfo from the schema provider in case it is modified
	// downstream (aka anonymize operator)
	var fwEncodingInfo *FixedWidthEncodingInfo
	var xlsxSheetInfo map[string]any
	sp := cpCtx.SchemaManager.GetSchemaProvider(inputChannelConfig.SchemaProvider)
	if sp != nil {
		fwEncodingInfo = sp.FixedWidthEncodingInfo()
		sheetInfoJson := sp.InputFormatDataJson()
		// log.Println(" *** LoadFiles: got xlsx sheet info json:", sheetInfoJson)
		if len(sheetInfoJson) > 0 {
			xlsxSheetInfo, err = ParseInputFormatDataXlsx(&sheetInfoJson)
			if err != nil {
				err = fmt.Errorf("%s while parsing xlsx sheet info metadata: %v", cpCtx.SessionId, err)
				log.Println(err)
				cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: 0, BadRowCount: 0, Err: err}
				return
			}
		}
	}

	samplingMaxCount := int64(inputChannelConfig.SamplingMaxCount)
	readBatchSize := inputChannelConfig.ReadBatchSize
	var count, totalRowCount, badRowCount, totalBadRowCount int64
	inputFormat := inputChannelConfig.Format
	gotMaxRecordCount := false

	for localInFile := range cpCtx.FileNamesCh[0] {
		if gotMaxRecordCount {
			// Don't read more records
			os.Remove(localInFile.LocalFileName)
			continue
		}
		if cpCtx.CpConfig.ClusterConfig.IsDebugMode {
			log.Printf("%s node %d Loading main file '%s'", cpCtx.SessionId, cpCtx.NodeId, localInFile.InFileKeyInfo.key)
		}
		// Encapsulte the switch so to factor out file handling
		err = func() (err error) {

			switch inputFormat {

			case "xlsx", "headerless_xlsx":
				count, badRowCount, err = cpCtx.ReadXlsxFile(&localInFile, xlsxSheetInfo, castToRdfTxtTypeFncs,
					computePipesInputCh, badRowChannel)

			default:
				var fileHd *os.File
				fileHd, err = os.Open(localInFile.LocalFileName)
				if err != nil {
					err = fmt.Errorf("while opening temp file '%s' (LoadMainInput): %v", localInFile.LocalFileName, err)
					log.Println(err)
					cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount, Err: err}
					return
				}
				defer func() {
					fileHd.Close()
					os.Remove(localInFile.LocalFileName)
				}()

				switch inputFormat {
				case "csv", "headerless_csv":
					count, badRowCount, err = cpCtx.ReadCsvFile(
						&localInFile, fileHd, castToRdfTxtTypeFncs, computePipesInputCh, badRowChannel)

				case "parquet", "parquet_select":
					count, err = cpCtx.ReadParquetFileV2(
						&localInFile, fileHd, readBatchSize, castToRdfTxtTypeFncs, inputSchemaCh, computePipesInputCh)
					if inputSchemaCh != nil {
						close(inputSchemaCh)
						inputSchemaCh = nil
					}
					badRowCount = 0

				case "fixed_width":
					count, badRowCount, err = cpCtx.ReadFixedWidthFile(
						&localInFile, fileHd, fwEncodingInfo, castToRdfTxtTypeFncs, computePipesInputCh, badRowChannel)

				default:
					err = fmt.Errorf("%s node %d, error: unsupported file format: %s", cpCtx.SessionId, cpCtx.NodeId, inputFormat)
					log.Println(err)
					cpCtx.ChResults.LoadFromS3FilesResultCh <- LoadFromS3FilesResult{LoadRowCount: totalRowCount, BadRowCount: totalBadRowCount, Err: err}
					return
				}
			}
			return
		}()

		totalRowCount += count
		totalBadRowCount += badRowCount
		if err != nil {
			log.Println(cpCtx.SessionId, "node", cpCtx.NodeId, "LoadFile returned error", err)
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
