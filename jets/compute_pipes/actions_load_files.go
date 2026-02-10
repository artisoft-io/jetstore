package compute_pipes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Load multipart files to JetStore, file to load are provided by channel fileNameCh
var (
	ErrKillSwitch     = errors.New("ErrKillSwitch")
	ComputePipesStart = time.Now()
)

type ReaderAtSeeker interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

func (cpCtx *ComputePipesContext) LoadFiles(ctx context.Context, dbpool *pgxpool.Pool) (err error) {

	// Create a channel to use as main input source for the pipeline
	computePipesInputCh := make(chan []any, 5)
	var computePipesMergeChs []chan []any
	var inputSchemaCh chan ParquetSchemaInfo

	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			fmt.Fprintf(&buf, "LoadFiles: recovered error: %v\n", r)
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
			close(cpCtx.ChResults.LoadFromS3FilesResultCh)
		}
		if inputSchemaCh != nil {
			close(inputSchemaCh)
			inputSchemaCh = nil
		}
		if err != nil {
			log.Printf("LoadFiles: terminating with err %v\n", err)
			cpCtx.DoneAll(err)
		}
	}()

	inputChannelConfig := &cpCtx.CpConfig.PipesConfig[0].InputChannel
	
	inputFormat := inputChannelConfig.Format
	if strings.HasPrefix(inputFormat, "parquet") && cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		// Save the parquet schema
		inputSchemaCh = make(chan ParquetSchemaInfo, 1)
	}

	var waitForDone *sync.WaitGroup
	l := len(inputChannelConfig.MergeChannels)

	// Prepare the S3DeviceManager
	err = cpCtx.NewS3DeviceManager()
	if err != nil {
		log.Printf("NewS3DeviceManager returned with err: %v\n", err)
		cpCtx.DoneAll(err)
		// skip loading files
		goto done
	}

	// Check if have merge channels
	if l > 0 {
		computePipesMergeChs = make([]chan []any, 0, l)
		waitForDone = new(sync.WaitGroup)
		for i := range l {
			channelConfig := inputChannelConfig.MergeChannels[i]
			mergeCh := make(chan []any, 5)
			computePipesMergeChs = append(computePipesMergeChs, mergeCh)
			// Start a goroutine to load the merge input files
			waitForDone.Go(func ()  {
				err := cpCtx.loadMergeInput(mergeCh, &channelConfig, cpCtx.FileNamesCh[i+1])
				if err != nil {
					log.Printf("loadMergeInput goroutine terminated with err: %v\n", err)
					cpCtx.DoneAll(err)
				}
			})
		}	
	}

	// Start the Compute Pipes async
	go cpCtx.StartComputePipes(dbpool, inputSchemaCh, computePipesInputCh, computePipesMergeChs)

	err = cpCtx.loadMainInput(computePipesInputCh, inputChannelConfig, inputSchemaCh)
	if err != nil {
		log.Printf("loadMainInput returned with err: %v\n", err)
		cpCtx.DoneAll(err)
		err = nil
	}

	if waitForDone != nil {
		// Wait for all merge input loaders to be done
		waitForDone.Wait()
	}
	done:
	close(cpCtx.ChResults.LoadFromS3FilesResultCh)
	return
}

func checkIncorrectDelimiter(singleColumnCount, inputRowCount int64, delimiter rune) (err error) {
	if float64(singleColumnCount) > 0.9*float64(inputRowCount) {
		// Got a single column while expecting multiple, must have invalid delimiter
		err = fmt.Errorf("error: got single column row while expecting file with multiple columns, is the delimiter '%s' the correct one?", string(delimiter))
		log.Println(err.Error())
	}
	return
}

func LastIndexByte(s []byte, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
