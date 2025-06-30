package compute_pipes

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"
)

type ComputePipesContextTestBuilder struct {
	AddionalInputHeaders    []string
	BadRowsConfig           *BadRowsSpec
	Compression             string
	CpipesMode              string
	Delimiter               rune
	DetectEncoding          bool
	Encoding                string
	EnforceRowMaxLength     bool
	EnforceRowMinLength     bool
	Format                  string
	InputColumns            []string
	NbrRowsInRecord         int64
	NoQuotes                bool
	PartFileKeyComponents   []CompiledPartFileComponent
	QuoteAllRecords         bool
	ReadBatchSize           int64
	SamplingMaxCount        int
	SamplingRate            int
	ShardOffset             int
	TrimColumns             bool
	UseLazyQuotes           bool
	VariableFieldsPerRecord bool
}

func (b ComputePipesContextTestBuilder) build() *ComputePipesContext {
	return &ComputePipesContext{
		AddionalInputHeaders:  b.AddionalInputHeaders,
		PartFileKeyComponents: b.PartFileKeyComponents,
		CpConfig: &ComputePipesConfig{
			CommonRuntimeArgs: &ComputePipesCommonArgs{
				CpipesMode:  b.CpipesMode,
				ProcessName: "Anonymize_File",
				SourcesConfig: SourcesConfigSpec{
					MainInput: &InputSourceSpec{
						InputColumns: b.InputColumns,
						// DomainKeys:   cpipesStartup.MainInputDomainKeysSpec,
						// DomainClass:  cpipesStartup.MainInputDomainClass,
					},
				},
			},
			ClusterConfig: &ClusterSpec{
				ShardOffset:      b.ShardOffset,
				S3WorkerPoolSize: 1,
				IsDebugMode:      true,
			},
			PipesConfig: []PipeSpec{{
				InputChannel: InputChannelConfig{
					FileConfig: FileConfig{
						BadRowsConfig:           &BadRowsSpec{BadRowsStepId: "bad_rows_step_id"},
						Compression:             b.Compression,
						Delimiter:               b.Delimiter,
						DetectEncoding:          b.DetectEncoding,
						Encoding:                b.Encoding,
						EnforceRowMaxLength:     b.EnforceRowMaxLength,
						EnforceRowMinLength:     b.EnforceRowMinLength,
						Format:                  b.Format,
						NbrRowsInRecord:         b.NbrRowsInRecord,
						NoQuotes:                b.NoQuotes,
						QuoteAllRecords:         b.QuoteAllRecords,
						ReadBatchSize:           b.ReadBatchSize,
						TrimColumns:             b.TrimColumns,
						UseLazyQuotes:           b.UseLazyQuotes,
						VariableFieldsPerRecord: b.VariableFieldsPerRecord,
					},
					SamplingMaxCount: b.SamplingMaxCount,
					SamplingRate:     b.SamplingRate,
				},
			}},
		},
		Done:  make(chan struct{}),
		ErrCh: make(chan error, 1000),
	}
}

func buildFWSchemaProvider(columns []SchemaColumnSpec) (SchemaProvider, error) {
	spec := &SchemaProviderSpec{
		FileConfig: FileConfig{
			Format: "fixed_width",
		},
		Columns: columns,
	}
	sp := NewDefaultSchemaProvider()
	return sp, sp.Initialize(nil, spec, nil, true)
}

// Positive test
func TestReadCsv01(t *testing.T) {
	reader, columns, size := dataSet01()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 18 {
		t.Errorf("expecting 18 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Test w/ 2 short row, no bad rows
func TestReadCsv02(t *testing.T) {
	reader, columns, size := dataSet02()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 18 {
		t.Errorf("expecting 18 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Test w/ 2 short row, 2 bad rows since VariableFieldsPerRecord is false
func TestReadCsv03(t *testing.T) {
	reader, columns, size := dataSet02()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 16 {
		t.Errorf("expecting 16 rows, got %d", count)
	}
	if badRowcount != 2 {
		t.Errorf("expecting 2 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Print(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Headerless Positive test
func TestReadCsv11(t *testing.T) {
	reader, columns, size := dataSet03()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "headerless_csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 18 {
		t.Errorf("expecting 18 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Headerless Test w/ 2 short row, no bad rows
func TestReadCsv12(t *testing.T) {
	reader, columns, size := dataSet04()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "headerless_csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 18 {
		t.Errorf("expecting 18 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Headerless Test w/ 2 short row, 2 bad rows since VariableFieldsPerRecord is false
func TestReadCsv13(t *testing.T) {
	reader, columns, size := dataSet04()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "headerless_csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 16 {
		t.Errorf("expecting 16 rows, got %d", count)
	}
	if badRowcount != 2 {
		t.Errorf("expecting 2 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Print(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Positive test, testing first shard (droping last row)
func TestReadCsv311(t *testing.T) {
	reader, columns, size := dataSet01()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             40,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   size / 3,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 17 {
		t.Errorf("expecting 17 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Positive test, testing second shard (drop first & last row)
func TestReadCsv312(t *testing.T) {
	reader, columns, size := dataSet03()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             40,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: size / 3,
				end:   2 * size / 3,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 16 {
		t.Errorf("expecting 16 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Positive test, testing last shard (drop first row)
func TestReadCsv313(t *testing.T) {
	reader, columns, size := dataSet03()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             40,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 2 * size / 3,
				end:   size,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 17 {
		t.Errorf("expecting 17 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// 2 short, 1 long rows, testing first shard (droping last row)
func TestReadCsv411(t *testing.T) {
	reader, columns, size := dataSet411()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     true,
		EnforceRowMinLength:     true,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             40,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   size / 3,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 14 {
		t.Errorf("expecting 14 rows, got %d", count)
	}
	if badRowcount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// 2 short, 1 long rows, testing second shard (droping first & last row)
func TestReadCsv412(t *testing.T) {
	reader, columns, size := dataSet412()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     true,
		EnforceRowMinLength:     true,
		Format:                  "csv",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             40,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: size / 3,
				end:   2 * size / 3,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 13 {
		t.Errorf("expecting 13 rows, got %d", count)
	}
	if badRowcount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Positive test, added columns
func TestReadCsv51(t *testing.T) {
	reader, columns, size := dataSet01()
	columns = append(columns, "add1", "add2", "key1")
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders: []string{"add1", "add2"},
		BadRowsConfig:        nil,
		Compression:          "none",
		CpipesMode:           "sharding",
		Delimiter:            ',',
		DetectEncoding:       false,
		Encoding:             "",
		EnforceRowMaxLength:  false,
		EnforceRowMinLength:  false,
		Format:               "csv",
		InputColumns:         columns,
		NbrRowsInRecord:      0,
		NoQuotes:             false,
		PartFileKeyComponents: []CompiledPartFileComponent{
			{ColumnName: "key1", Regex: regexp.MustCompile(`value=(.*?)\/`)},
		},
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/value=something/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 18 {
		t.Errorf("expecting 18 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// 2 short, 1 long rows, testing second shard (droping first & last row), added columns
func TestReadCsv512(t *testing.T) {
	reader, columns, size := dataSet412()
	columns = append(columns, "add1", "add2", "key1")
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders: []string{"add1", "add2"},
		BadRowsConfig:        nil,
		Compression:          "none",
		CpipesMode:           "sharding",
		Delimiter:            ',',
		DetectEncoding:       false,
		Encoding:             "",
		EnforceRowMaxLength:  true,
		EnforceRowMinLength:  true,
		Format:               "csv",
		InputColumns:         columns,
		NbrRowsInRecord:      0,
		NoQuotes:             false,
		PartFileKeyComponents: []CompiledPartFileComponent{
			{ColumnName: "key1", Regex: regexp.MustCompile(`value=(.*?)\/`)},
		},
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             40,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	count, badRowcount, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/value=something/key",
				size:  size,
				start: size / 3,
				end:   2 * size / 3,
			},
		}, reader, nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 13 {
		t.Errorf("expecting 13 rows, got %d", count)
	}
	if badRowcount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// 2 short, 1 long rows, err out, testing second shard (droping first & last row), added columns
func TestReadCsv612(t *testing.T) {
	reader, columns, size := dataSet412()
	columns = append(columns, "add1", "add2", "key1")
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders: []string{"add1", "add2"},
		BadRowsConfig:        nil,
		Compression:          "none",
		CpipesMode:           "sharding",
		Delimiter:            ',',
		DetectEncoding:       false,
		Encoding:             "",
		EnforceRowMaxLength:  true,
		EnforceRowMinLength:  true,
		Format:               "csv",
		InputColumns:         columns,
		NbrRowsInRecord:      0,
		NoQuotes:             false,
		PartFileKeyComponents: []CompiledPartFileComponent{
			{ColumnName: "key1", Regex: regexp.MustCompile(`value=(.*?)\/`)},
		},
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             40,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	_, _, err := cpCtx.ReadCsvFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/value=something/key",
				size:  size,
				start: size / 3,
				end:   2 * size / 3,
			},
		}, reader, nil, computePipesInputCh, nil)

	// Close the channels
	close(computePipesInputCh)
	if err == nil {
		t.Fatal(err)
	}
	// t.Error(err)
}

// FW - Positive test
func TestReadFW01(t *testing.T) {
	reader, columns, size := dataSetFW01()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:    nil,
		BadRowsConfig:           nil,
		Compression:             "none",
		CpipesMode:              "sharding",
		Delimiter:               ',',
		DetectEncoding:          false,
		Encoding:                "",
		EnforceRowMaxLength:     false,
		EnforceRowMinLength:     false,
		Format:                  "fixed_width",
		InputColumns:            columns,
		NbrRowsInRecord:         0,
		NoQuotes:                false,
		PartFileKeyComponents:   nil,
		ReadBatchSize:           0,
		SamplingMaxCount:        0,
		SamplingRate:            0,
		ShardOffset:             20,
		TrimColumns:             true,
		UseLazyQuotes:           false,
		VariableFieldsPerRecord: false,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 6 {
		t.Errorf("expecting 6 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// FW - Positive test, 2 short rows, no bad rows
func TestReadFW02(t *testing.T) {
	reader, columns, size := dataSetFW02()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:  nil,
		BadRowsConfig:         nil,
		Compression:           "none",
		CpipesMode:            "sharding",
		Delimiter:             ',',
		DetectEncoding:        false,
		Encoding:              "",
		EnforceRowMaxLength:   false,
		EnforceRowMinLength:   false,
		Format:                "fixed_width",
		InputColumns:          columns,
		NbrRowsInRecord:       0,
		NoQuotes:              false,
		PartFileKeyComponents: nil,
		ReadBatchSize:         0,
		SamplingMaxCount:      0,
		SamplingRate:          0,
		ShardOffset:           20,
		TrimColumns:           true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 6 {
		t.Errorf("expecting 6 rows, got %d", count)
	}
	if badRowcount != 0 {
		t.Errorf("expecting 0 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// FW - Positive test, 2 short rows, 2 bad rows
func TestReadFW03(t *testing.T) {
	reader, columns, size := dataSetFW02()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:  nil,
		BadRowsConfig:         nil,
		Compression:           "none",
		CpipesMode:            "sharding",
		Delimiter:             ',',
		DetectEncoding:        false,
		Encoding:              "",
		EnforceRowMaxLength:   false,
		EnforceRowMinLength:   true,
		Format:                "fixed_width",
		InputColumns:          columns,
		NbrRowsInRecord:       0,
		NoQuotes:              false,
		PartFileKeyComponents: nil,
		ReadBatchSize:         0,
		SamplingMaxCount:      0,
		SamplingRate:          0,
		ShardOffset:           20,
		TrimColumns:           true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 4 {
		t.Errorf("expecting 4 rows, got %d", count)
	}
	if badRowcount != 2 {
		t.Errorf("expecting 2 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// FW - Positive test, 2 short, 1 long rows, 3 bad rows
func TestReadFW04(t *testing.T) {
	reader, columns, size := dataSetFW03()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:  nil,
		BadRowsConfig:         nil,
		Compression:           "none",
		CpipesMode:            "sharding",
		Delimiter:             ',',
		DetectEncoding:        false,
		Encoding:              "",
		EnforceRowMaxLength:   true,
		EnforceRowMinLength:   true,
		Format:                "fixed_width",
		InputColumns:          columns,
		NbrRowsInRecord:       0,
		NoQuotes:              false,
		PartFileKeyComponents: nil,
		ReadBatchSize:         0,
		SamplingMaxCount:      0,
		SamplingRate:          0,
		ShardOffset:           20,
		TrimColumns:           true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   0,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 3 {
		t.Errorf("expecting 3 rows, got %d", count)
	}
	if badRowcount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// FW 1 short, 1 long rows, 2 bad rows, fist shard (drop last row - which is short as well)
func TestReadFW141(t *testing.T) {
	reader, columns, size := dataSetFW03()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:  nil,
		BadRowsConfig:         nil,
		Compression:           "none",
		CpipesMode:            "sharding",
		Delimiter:             ',',
		DetectEncoding:        false,
		Encoding:              "",
		EnforceRowMaxLength:   true,
		EnforceRowMinLength:   true,
		Format:                "fixed_width",
		InputColumns:          columns,
		NbrRowsInRecord:       0,
		NoQuotes:              false,
		PartFileKeyComponents: nil,
		ReadBatchSize:         0,
		SamplingMaxCount:      0,
		SamplingRate:          0,
		ShardOffset:           180,
		TrimColumns:           true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 0,
				end:   size / 3,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 3 {
		t.Errorf("expecting 3 rows, got %d", count)
	}
	if badRowcount != 2 {
		t.Errorf("expecting 2 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// FW 1 short, 1 long rows, 2 bad rows, second shard (drop first & last row - which is short as well)
func TestReadFW142(t *testing.T) {
	reader, columns, size := dataSetFW03()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:  nil,
		BadRowsConfig:         nil,
		Compression:           "none",
		CpipesMode:            "sharding",
		Delimiter:             ',',
		DetectEncoding:        false,
		Encoding:              "",
		EnforceRowMaxLength:   true,
		EnforceRowMinLength:   true,
		Format:                "fixed_width",
		InputColumns:          columns,
		NbrRowsInRecord:       0,
		NoQuotes:              false,
		PartFileKeyComponents: nil,
		ReadBatchSize:         0,
		SamplingMaxCount:      0,
		SamplingRate:          0,
		ShardOffset:           180,
		TrimColumns:           true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: size / 3,
				end:   2 * size / 3,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 2 {
		t.Errorf("expecting 2 rows, got %d", count)
	}
	if badRowcount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// FW 2 short, 1 long rows, 3 bad rows, last shard (drop first row)
func TestReadFW143(t *testing.T) {
	reader, columns, size := dataSetFW03()
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders:  nil,
		BadRowsConfig:         nil,
		Compression:           "none",
		CpipesMode:            "sharding",
		Delimiter:             ',',
		DetectEncoding:        false,
		Encoding:              "",
		EnforceRowMaxLength:   true,
		EnforceRowMinLength:   true,
		Format:                "fixed_width",
		InputColumns:          columns,
		NbrRowsInRecord:       0,
		NoQuotes:              false,
		PartFileKeyComponents: nil,
		ReadBatchSize:         0,
		SamplingMaxCount:      0,
		SamplingRate:          0,
		ShardOffset:           180,
		TrimColumns:           true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/key",
				size:  size,
				start: 2 * size / 3,
				end:   size,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 2 {
		t.Errorf("expecting 2 rows, got %d", count)
	}
	if badRowcount != 4 {
		t.Errorf("expecting 4 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// FW 1 short, 1 long rows, 2 bad rows, second shard (drop first & last row - which is short as well)
// Added columns
func TestReadFW542(t *testing.T) {
	reader, columns, size := dataSetFW03()
	columns = append(columns, "add1", "add2", "key1")
	cpCtx := ComputePipesContextTestBuilder{
		AddionalInputHeaders: []string{"add1", "add2"},
		BadRowsConfig:        nil,
		Compression:          "none",
		CpipesMode:           "sharding",
		Delimiter:            ',',
		DetectEncoding:       false,
		Encoding:             "",
		EnforceRowMaxLength:  true,
		EnforceRowMinLength:  true,
		Format:               "fixed_width",
		InputColumns:         columns,
		NbrRowsInRecord:      0,
		NoQuotes:             false,
		PartFileKeyComponents: []CompiledPartFileComponent{
			{ColumnName: "key1", Regex: regexp.MustCompile(`value=(.*?)\/`)},
		},
		ReadBatchSize:    0,
		SamplingMaxCount: 0,
		SamplingRate:     0,
		ShardOffset:      180,
		TrimColumns:      true,
	}.build()

	computePipesInputCh := make(chan []any, 50)
	badRowChannel := &BadRowsChannel{
		s3DeviceManager: nil,
		s3BasePath:      "",
		OutputCh:        make(chan []byte, 50),
		doneCh:          cpCtx.Done,
		errCh:           cpCtx.ErrCh,
	}
	sp, err := buildFWSchemaProvider([]SchemaColumnSpec{
		{Name: "first_name", Length: 25},
		{Name: "last_name", Length: 25},
		{Name: "other1", Length: 25},
		{Name: "city", Length: 25},
		{Name: "mbr_id", Length: 10},
		{Name: "other2", Length: 25},
		{Name: "other3", Length: 25},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, badRowcount, err := cpCtx.ReadFixedWidthFile(
		&FileName{
			InFileKeyInfo: FileKeyInfo{
				key:   "file/value=something/key",
				size:  size,
				start: size / 3,
				end:   2 * size / 3,
			},
		}, reader, sp.FixedWidthEncodingInfo(), nil, computePipesInputCh, badRowChannel)

	// Close the channels
	close(computePipesInputCh)
	// close(badRowChannel.OutputCh)
	if badRowChannel != nil {
		badRowChannel.Done()
	}

	if err != nil {
		t.Errorf("got err: %v", err)
	}
	fmt.Println("Got count", count, "badRowCount", badRowcount)
	if count != 2 {
		t.Errorf("expecting 2 rows, got %d", count)
	}
	if badRowcount != 3 {
		t.Errorf("expecting 3 bad rows, got %d", badRowcount)
	}

	// Check the data
	fmt.Println("THE DATA")
	var checkCount int64 = 0
	for row := range computePipesInputCh {
		checkCount++
		fmt.Println(row)
	}
	if checkCount != count {
		t.Errorf("check count does not match, checkCount is %d", checkCount)
	}
	fmt.Println("THE BAD ROWS")
	checkCount = 0
	for row := range badRowChannel.OutputCh {
		checkCount++
		fmt.Println(string(row))
	}
	if checkCount != badRowcount {
		t.Errorf("bad row check count does not match, checkCount is %d", checkCount)
	}
	// t.Errorf("OK")
}

// Good, no bad rows
func dataSet01() (ReaderAtSeeker, []string, int) {
	rawData := `col1,col2,col3,col4
row01c1,row01c2,row01c3,row01c4
row02c1,row02c2,row02c3,row02c4
row03c1,row03c2,row03c3,row03c4
row04c1,row04c2,row04c3,row04c4
row05c1,row05c2,row05c3,row05c4
row06c1,row06c2,row06c3,row06c4
row07c1,row07c2,row07c3,row07c4
row08c1,row08c2,row08c3,row08c4
row09c1,row09c2,row09c3,row09c4
row11c1,row11c2,row11c3,row11c4
row12c1,row12c2,row12c3,row12c4
row13c1,row13c2,row13c3,row13c4
row14c1,row14c2,row14c3,row14c4
row15c1,row15c2,row15c3,row15c4
row16c1,row16c2,row16c3,row16c4
row17c1,row17c2,row17c3,row17c4
row18c1,row18c2,row18c3,row18c4
row19c1,row19c2,row19c3,row19c4
`
	headers := []string{"col1", "col2", "col3", "col4"}
	buf := bytes.NewReader([]byte(rawData))
	return buf, headers, len(rawData)
}

// 2 short rows
func dataSet02() (ReaderAtSeeker, []string, int) {
	rawData := `col1,col2,col3,col4
row01c1,row01c2,row01c3,row01c4
row02c1,row02c2,row02c3,row02c4
row03c1,row03c2,row03c3,row03c4
row04c1,row04c2,row04c3,row04c4
row05c1,row05c2,row05c3
row06c1,row06c2,row06c3,row06c4
row07c1,row07c2,row07c3,row07c4
row08c1,row08c2,row08c3,row08c4
row09c1,row09c2,row09c3,row09c4
row11c1,row11c2,row11c3,row11c4
row12c1,row12c2,row12c3,row12c4
row13c1,row13c2,row13c3,row13c4
row14c1,row14c2,row14c3,row14c4
row15c1,row15c2,row15c3
row16c1,row16c2,row16c3,row16c4
row17c1,row17c2,row17c3,row17c4
row18c1,row18c2,row18c3,row18c4
row19c1,row19c2,row19c3,row19c4
`
	headers := []string{"col1", "col2", "col3", "col4"}
	buf := bytes.NewReader([]byte(rawData))
	return buf, headers, len(rawData)
}

// 2 short, 1 long rows
func dataSet411() (ReaderAtSeeker, []string, int) {
	rawData := `col1,col2,col3,col4
row01c1,row01c2,row01c3,row01c4
row02c1,row02c2,row02c3,row02c4
row03c1,row03c2,row03c3,row03c4
row04c1,row04c2,row04c3,row04c4
row05c1,row05c2,row05c3
row06c1,row06c2,row06c3,row06c4
row07c1,row07c2,row07c3,row07c4
row08c1,row08c2,row08c3,row08c4
row09c1,row09c2,row09c3,row09c4,THIS IS,TOO MUCH
row11c1,row11c2,row11c3,row11c4
row12c1,row12c2,row12c3,row12c4
row13c1,row13c2,row13c3,row13c4
row14c1,row14c2,row14c3,row14c4
row15c1,row15c2,row15c3
row16c1,row16c2,row16c3,row16c4
row17c1,row17c2,row17c3,row17c4
row18c1,row18c2,row18c3,row18c4
row19c1,row19c2,row19c3,row19c4
`
	headers := []string{"col1", "col2", "col3", "col4"}
	buf := bytes.NewReader([]byte(rawData))
	return buf, headers, len(rawData)
}

// Headerless, 2 short, 1 long rows
func dataSet412() (ReaderAtSeeker, []string, int) {
	rawData := `row01c1,row01c2,row01c3,row01c4
row02c1,row02c2,row02c3,row02c4
row03c1,row03c2,row03c3,row03c4
row04c1,row04c2,row04c3,row04c4
row05c1,row05c2,row05c3
row06c1,row06c2,row06c3,row06c4
row07c1,row07c2,row07c3,row07c4
row08c1,row08c2,row08c3,row08c4
row09c1,row09c2,row09c3,row09c4,THIS IS,TOO MUCH
row11c1,row11c2,row11c3,row11c4
row12c1,row12c2,row12c3,row12c4
row13c1,row13c2,row13c3,row13c4
row14c1,row14c2,row14c3,row14c4
row15c1,row15c2,row15c3
row16c1,row16c2,row16c3,row16c4
row17c1,row17c2,row17c3,row17c4
row18c1,row18c2,row18c3,row18c4
row19c1,row19c2,row19c3,row19c4
`
	headers := []string{"col1", "col2", "col3", "col4"}
	buf := bytes.NewReader([]byte(rawData))
	return buf, headers, len(rawData)
}

// Good headerless, no bad rows
func dataSet03() (ReaderAtSeeker, []string, int) {
	rawData := `row01c1,row01c2,row01c3,row01c4
row02c1,row02c2,row02c3,row02c4
row03c1,row03c2,row03c3,row03c4
row04c1,,row04c3,row04c4
row05c1,,,row05c4
row06c1,row06c2,row06c3,row06c4
row07c1,row07c2,row07c3,row07c4
row08c1,row08c2,row08c3,row08c4
row09c1,row09c2,row09c3,row09c4
row11c1,row11c2,row11c3,row11c4
row12c1,row12c2,row12c3,row12c4
row13c1,row13c2,row13c3,row13c4
row14c1,row14c2,row14c3,row14c4
row15c1,row15c2,row15c3,row15c4
row16c1,row16c2,row16c3,row16c4
row17c1,row17c2,row17c3,row17c4
row18c1,row18c2,row18c3,row18c4
row19c1,row19c2,row19c3,row19c4
`
	headers := []string{}
	buf := bytes.NewReader([]byte(rawData))
	return buf, headers, len(rawData)
}

// headerless, 2 bad rows
func dataSet04() (ReaderAtSeeker, []string, int) {
	rawData := `row01c1,row01c2,row01c3,row01c4
row02c1,row02c2,row02c3,row02c4
row03c1,row03c2,row03c3,row03c4
row04c1,row04c2,row04c3,row04c4
row05c1,,row05c3
row06c1,row06c2,row06c3,row06c4
row07c1,,,
row08c1,row08c2,row08c3,row08c4
row09c1,row09c2,row09c3,row09c4
row11c1,row11c2,row11c3,row11c4
row12c1,row12c2,row12c3,row12c4
row13c1,row13c2,row13c3,row13c4
row14c1,row14c2,row14c3,row14c4
row15c1,row15c2,
row16c1,row16c2,row16c3,row16c4
row17c1,row17c2,row17c3,row17c4
row18c1,row18c2,row18c3,row18c4
row19c1,row19c2,row19c3,row19c4
`
	headers := []string{}
	buf := bytes.NewReader([]byte(rawData))
	return buf, headers, len(rawData)
}

// FW, no bad rows
func dataSetFW01() (ReaderAtSeeker, []string, int) {
	rawData := `first_name               last_name                other1                   city                     mbr_id    other2                   other3                   
John                     Doe                      abcdef                   Montreal                 0123456789qwerty                   asdfg                    
Robert                   Smith                    abcdef                   New York                 0a2b3c4d5eqwerty                   asdfg                    
john                     Albert                   abcdef                   Huston                   1111111111qwerty                   asdfg                    
Peter                    Sunny                    abcdef                   Madrid                   2222222222qwerty                   asdfg                    
     Michel                 Dufresne                  some thing else            Montreal             2222334    some other thing            some more....      `
	headers := []string{"first_name",
		"last_name",
		"other1",
		"city",
		"mbr_id",
		"other2",
		"other3"}
	buf := bytes.NewReader([]byte(rawData))
	// FW encoding
	return buf, headers, len(rawData)
}

// FW, 2 short rows
func dataSetFW02() (ReaderAtSeeker, []string, int) {
	rawData := `first_name               last_name                other1                   city                     mbr_id    other2                   other3                   
John                     Doe                      abcdef                   Montreal                 0123456789qwerty                   asdfg                    
Robert                   Smith                    abcdef                   New York                 0a2b3c4d5eqwerty                   asdfg                    
john                     Albert                   abcdef                   Huston                   1111111111qwerty                   asdfg                    
Peter                    Sunny                    abcdef                   Madrid                   2222222222qwerty             
     Michel                 Dufresne                  some thing else            Montreal             2222334    some other thing`
	headers := []string{"first_name",
		"last_name",
		"other1",
		"city",
		"mbr_id",
		"other2",
		"other3"}
	buf := bytes.NewReader([]byte(rawData))
	// FW encoding
	return buf, headers, len(rawData)
}

// FW, 2 short, 1 long rows
func dataSetFW03() (ReaderAtSeeker, []string, int) {
	rawData := `first_name               last_name                other1                   city                     mbr_id    other2                   other3                   
John                     Doe                      abcdef                   Montreal                 0123456789qwerty                   asdfg                    
Peter                    Sunny                    abcdef                   Madrid                   2222222222qwerty             
Robert                   Smith                    abcdef                   New York                 0a2b3c4d5eqwerty                   asdfg                    THIS IS TOO LONG
john                     Albert                   abcdef                   Huston                   1111111111qwerty                   asdfg                    
     Michel                 Dufresne                  some thing else            Montreal             2222334    some other thing`
	headers := []string{"first_name",
		"last_name",
		"other1",
		"city",
		"mbr_id",
		"other2",
		"other3"}
	buf := bytes.NewReader([]byte(rawData))
	// FW encoding
	return buf, headers, len(rawData)
}
