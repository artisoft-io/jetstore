package compute_pipes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes main entry point

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

type ChannelResults struct {
	LoadFromS3FilesResultCh chan LoadFromS3FilesResult
	Copy2DbResultCh         chan chan ComputePipesResult
	WritePartitionsResultCh chan chan chan ComputePipesResult
	MapOnClusterResultCh    chan chan chan ComputePipesResult
}

// Function to write transformed row to database
func StartComputePipes(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, done chan struct{}, errCh chan error,
	computePipesInputCh <-chan []interface{}, chResults *ChannelResults,
	computePipesJson *string, envSettings map[string]interface{},
	fileKeyComponents map[string]interface{}) {
	var peersAddr []string

	var cpErr error
	if computePipesJson == nil || len(*computePipesJson) == 0 {
		// Loader in classic mode, no compute pipes defined
		tableIdentifier, err := SplitTableName(headersDKInfo.TableName)
		if err != nil {
			cpErr = fmt.Errorf("while splitting table name: %s", err)
			goto gotError
		}
		wt := WriteTableSource{
			source:          computePipesInputCh,
			tableIdentifier: tableIdentifier,       // using default staging table
			columns:         headersDKInfo.Headers, // using default staging table
		}
		table := make(chan ComputePipesResult, 1)
		chResults.Copy2DbResultCh <- table
		wt.writeTable(dbpool, done, table)

	} else {
		fmt.Println("Compute Pipes identified")

		// unmarshall the compute graph definition
		var cpConfig ComputePipesConfig
		err := json.Unmarshal([]byte(*computePipesJson), &cpConfig)
		if err != nil {
			cpErr = fmt.Errorf("while unmarshaling compute pipes json: %s", err)
			goto gotError
		}

		// Add to envSettings based on compute pipe config
		if cpConfig.Context != nil {
			for _, contextSpec := range *cpConfig.Context {
				switch contextSpec.Type {
				case "file_key_component":
					envSettings[contextSpec.Key] = fileKeyComponents[contextSpec.Expr]
				default:
					cpErr = fmt.Errorf("error: unknown ContextSpec Type: %v", contextSpec.Type)
					goto gotError
				}
			}
		}

		// Prepare the channel registry
		channelRegistry := &ChannelRegistry{
			computePipesInputCh: computePipesInputCh,
			inputColumns:        headersDKInfo.HeadersPosMap,
			inputChannelSpec: &ChannelSpec{
				Name:    "input_row",
				Columns: headersDKInfo.Headers,
			},
			computeChannels:     make(map[string]*Channel),
			outputTableChannels: make([]string, 0),
			closedChannels:      make(map[string]bool),
		}
		for i := range cpConfig.Channels {
			cm := make(map[string]int)
			for j, c := range cpConfig.Channels[i].Columns {
				cm[c] = j
			}
			channelRegistry.computeChannels[cpConfig.Channels[i].Name] = &Channel{
				channel: make(chan []interface{}),
				columns: cm,
				config:  &cpConfig.Channels[i],
			}
		}
		fmt.Println("Compute Pipes channel registry ready")
		// for i := range cpConfig.Channels {
		// 	fmt.Println("**& Channel", cpConfig.Channels[i].Name, "Columns map", channelRegistry.computeChannels[cpConfig.Channels[i].Name].columns)
		// }

		// Prepare the output tables
		for i := range cpConfig.OutputTables {
			tableIdentifier, err := SplitTableName(cpConfig.OutputTables[i].Name)
			if err != nil {
				cpErr = fmt.Errorf("while splitting table name: %s", err)
				goto gotError
			}
			// fmt.Println("**& Preparing Output Table", tableIdentifier)
			err = prepareOutoutTable(dbpool, tableIdentifier, &cpConfig.OutputTables[i])
			if err != nil {
				cpErr = fmt.Errorf("while preparing output table: %s", err)
				goto gotError
			}
			outChannel := channelRegistry.computeChannels[cpConfig.OutputTables[i].Key]
			channelRegistry.outputTableChannels = append(channelRegistry.outputTableChannels, cpConfig.OutputTables[i].Key)
			if outChannel == nil {
				cpErr = fmt.Errorf("error: invalid Compute Pipes configuration: Output table %s does not have a channel configuration",
					cpConfig.OutputTables[i].Name)
				goto gotError
			}
			// fmt.Println("**& Channel for Output Table", tableIdentifier, "is:", outChannel.config.Name)
			wt := WriteTableSource{
				source:          outChannel.channel,
				tableIdentifier: tableIdentifier,
				columns:         outChannel.config.Columns,
			}
			table := make(chan ComputePipesResult, 1)
			chResults.Copy2DbResultCh <- table
			go wt.writeTable(dbpool, done, table)
		}
		fmt.Println("Compute Pipes output tables ready")

		// Setup the s3Uploader
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("JETS_REGION")))
		if err != nil {
			cpErr = fmt.Errorf("while loading aws configuration (in StartComputePipes): %v", err)
			goto gotError
		}
		// Create a s3 client
		s3Client := s3.NewFromConfig(cfg)
		// Create the uploader with the client and custom options
		s3Uploader := manager.NewUploader(s3Client)

		var nodeAddr string
		if cpConfig.ClusterConfig != nil {
			// Get the node address and register it with database
			nodePort := strings.Split(envSettings["$CPIPES_SERVER_ADDR"].(string), ":")[1]
			if envSettings["$JETSTORE_DEV_MODE"].(bool) {
				nodeAddr = fmt.Sprintf("127.0.0.1:%s", nodePort)
			} else {
				nodeIp, err := awsi.GetPrivateIp()
				if err != nil {
					cpErr = fmt.Errorf("while getting node's IP (in StartComputePipes): %v", err)
					goto gotError
				}
				nodeAddr = fmt.Sprintf("%s:%s", nodeIp, nodePort)
			}
			// Register node to database
			sessionId := envSettings["$SESSIONID"].(string)
			stmt := fmt.Sprintf(
				"INSERT INTO jetsapi.cpipes_cluster_node_registry (session_id, node_address, shard_id) VALUES ('%s','%s',%d);",
				sessionId, nodeAddr, envSettings["$SHARD_ID"].(int))
			log.Println(stmt)
			_, err = dbpool.Exec(context.Background(), stmt)
			if err != nil {
				cpErr = fmt.Errorf("while inserting node's addressin db (in StartComputePipes): %v", err)
				goto gotError
			}
			log.Printf("Node's address %s registered into database", nodeAddr)
			// Get the peers addresses from database
			registrationTimeout := cpConfig.ClusterConfig.PeerRegistrationTimeout
			if registrationTimeout == 0 {
				registrationTimeout = 60
			}
			nbrNodes := envSettings["$NBR_SHARDS"].(int)
			stmt = "SELECT node_address FROM jetsapi.cpipes_cluster_node_registry WHERE session_id = $1 ORDER BY shard_id ASC"
			start := time.Now()
			for {
				peersAddr = make([]string, 0)
				rows, err := dbpool.Query(context.Background(), stmt, sessionId)
				if err != nil {
					cpErr = fmt.Errorf("while querying peer's address from db (in StartComputePipes): %v", err)
					goto gotError
				}
				for rows.Next() {
					var addr string
					if err := rows.Scan(&addr); err != nil {
						rows.Close()
						cpErr = fmt.Errorf("while scanning node's address from db (in StartComputePipes): %v", err)
						goto gotError
					}
					peersAddr = append(peersAddr, addr)
				}
				rows.Close()
				if len(peersAddr) == nbrNodes {
					break
				}
				log.Printf("Got %d out of %d peer's addresses, will try again", len(peersAddr), nbrNodes)
				if time.Since(start) > time.Duration(registrationTimeout)*time.Second {
					log.Printf("Error, timeout occured while trying to get peer's addresses")
					cpErr = fmt.Errorf("error: timeout while getting peers addresses (in StartComputePipes): %v", err)
					goto gotError
				}
				time.Sleep(1 * time.Second)
			}
		}

		ctx := &BuilderContext{
			dbpool:          dbpool,
			cpConfig:        &cpConfig,
			channelRegistry: channelRegistry,
			selfAddress:     nodeAddr,
			peersAddress:    peersAddr,
			done:            done,
			errCh:           errCh,
			chResults:       chResults,
			env:             envSettings,
			s3Uploader:      s3Uploader,
		}
		err = ctx.buildComputeGraph()
		if err != nil {
			cpErr = fmt.Errorf("while building the compute graph: %s", err)
			goto gotError
		}

	}
	// All done!
	close(chResults.Copy2DbResultCh)
	close(chResults.WritePartitionsResultCh)
	close(chResults.MapOnClusterResultCh)
	return

gotError:
	log.Println(cpErr)
	fmt.Println("**! gotError in StartComputePipes")
	errCh <- cpErr
	close(done)
	close(chResults.Copy2DbResultCh)
	close(chResults.WritePartitionsResultCh)
	close(chResults.MapOnClusterResultCh)
}
