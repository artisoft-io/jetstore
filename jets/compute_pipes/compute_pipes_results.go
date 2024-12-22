package compute_pipes

import (
	"context"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ComputePipesResult struct {
	// Table name can be jets_partition name
	// PartCount is nbr of file part in jets_partition
	TableName    string
	CopyRowCount int64
	PartsCount   int64
	Err          error
}
type LoadFromS3FilesResult struct {
	LoadRowCount int64
	BadRowCount  int64
	Err          error
}
type JetrulesWorkerResult struct {
	ReteSessionCount int64
	ErrorsCount      int64
	Err              error
}
type ClusteringResult struct {
	Err          error
}

// ChannelResults holds the channel reporting back results.
// LoadFromS3FilesResultCh: results from loading files (row count)
// Copy2DbResultCh: results of records written to JetStore DB (row count)
// WritePartitionsResultCh: report on rows output to s3 (row count)
// S3PutObjectResultCh: reports on nbr of files put to s3 (file count)
// JetrulesWorkerResultCh: reports on nbr of rete session and errors
// ClusteringResultCh: reports on nbr of clusters identified and errors
type ChannelResults struct {
	LoadFromS3FilesResultCh chan LoadFromS3FilesResult
	Copy2DbResultCh         chan chan ComputePipesResult
	WritePartitionsResultCh chan chan ComputePipesResult
	S3PutObjectResultCh     chan ComputePipesResult
	JetrulesWorkerResultCh  chan chan JetrulesWorkerResult
	ClusteringResultCh      chan chan ClusteringResult
}

type SaveResultsContext struct {
	dbpool        *pgxpool.Pool
	JetsPartition string
	NodeId        int
	SessionId     string
}

func NewSaveResultsContext(dbpool *pgxpool.Pool) *SaveResultsContext {
	return &SaveResultsContext{dbpool: dbpool}
}

func (ctx *SaveResultsContext) Save(category string, result *ComputePipesResult) {
	if result == nil {
		return
	}
	stmt := `INSERT INTO jetsapi.cpipes_results (
		session_id, jets_partition, node_id, category, name, row_count, parts_count, err) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	jetsPartition := ctx.JetsPartition
	nodeId := ctx.NodeId
	sessionId := ctx.SessionId
	var errMsg string
	if result.Err != nil {
		errMsg = result.Err.Error()
	}
	_, err := ctx.dbpool.Exec(context.Background(), stmt, sessionId, jetsPartition, nodeId, category,
		result.TableName, result.CopyRowCount, result.PartsCount, errMsg)
	if err != nil {
		log.Printf("error inserting in jetsapi.cpipes_results table: %v", err)
		return
	}
}
