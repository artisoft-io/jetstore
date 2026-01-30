package compute_pipes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"strings"
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

	// Create a channel to use as a buffer between the file loader and the copy to db
	// This gives the opportunity to use Compute Pipes to transform the data before writing to the db
	computePipesInputCh := make(chan []any, 5)
	var computePipesMergeChs []<-chan []any
	var inputSchemaCh chan ParquetSchemaInfo

	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			fmt.Fprintf(&buf, "LoadFiles: recovered error: %v\n", r)
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
		}
		if inputSchemaCh != nil {
			close(inputSchemaCh)
			inputSchemaCh = nil
		}
		close(computePipesInputCh)
		close(cpCtx.ChResults.LoadFromS3FilesResultCh)
		if err != nil {
			log.Printf("LoadFile: terminating with err %v, closing done channel\n", err)
			cpCtx.ErrCh <- err
			// Avoid closing a closed channel
			select {
			case <-cpCtx.Done:
				log.Println("LoadFile: done channel already closed")
			default:
				close(cpCtx.Done)
			}
		}
	}()

	inputChannelConfig := &cpCtx.CpConfig.PipesConfig[0].InputChannel
	
	inputFormat := inputChannelConfig.Format
	if strings.HasPrefix(inputFormat, "parquet") && cpCtx.CpConfig.CommonRuntimeArgs.CpipesMode == "sharding" {
		// Save the parquet schema
		inputSchemaCh = make(chan ParquetSchemaInfo, 1)
	}

	// Prepare the S3DeviceManager
	err = cpCtx.NewS3DeviceManager()
	if err != nil {
		return
	}

	// Check if have merge channels
	l := len(inputChannelConfig.MergeChannels)
	if l > 0 {
		computePipesMergeChs = make([]<-chan []any, 0, l)
		for i := range l {
			channelConfig := inputChannelConfig.MergeChannels[i]
			mergeCh := make(chan []any, 5)
			computePipesMergeChs = append(computePipesMergeChs, mergeCh)
			go cpCtx.loadMergeInput(mergeCh, &channelConfig, cpCtx.MergeFileNamesCh[i])
		}	
	}

	// Start the Compute Pipes async
	go cpCtx.StartComputePipes(dbpool, inputSchemaCh, computePipesInputCh, computePipesMergeChs)

	return cpCtx.loadMainInput(computePipesInputCh, inputChannelConfig, inputSchemaCh)
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
