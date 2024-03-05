package compute_pipes

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Compute Pipes main entry point

type ComputePipesResult struct {
	CopyRowCount int64
	Err          error
}

// Function to write transformed row to database
func StartComputePipes(dbpool *pgxpool.Pool, headersDKInfo *schema.HeadersAndDomainKeysInfo, done chan struct{},
	computePipesInputCh <-chan []interface{}, computePipesResultCh chan<- ComputePipesResult, computePipesJson *string,
	envSettings map[string]interface{}) {

	var cpErr error	
	if computePipesJson == nil || len(*computePipesJson) == 0 {
		// Loader in classic mode, no compute pipes defined
		tableIdentifier, err := SplitTableName(headersDKInfo.TableName)
		if err != nil {
			cpErr = fmt.Errorf("while splitting table name: %s", err)
			goto gotError
		}
		wt := WriteTableSource{
			source: computePipesInputCh,
			tableIdentifier: tableIdentifier,	// using default staging table
			columns: headersDKInfo.Headers, // using default staging table
		}
		wt.writeTable(dbpool, done, computePipesResultCh)

	} else {
		fmt.Println("**& Compute Pipes identified")

		// unmarshall the compute graph definition
		var cpConfig ComputePipesConfig
		err := json.Unmarshal([]byte(*computePipesJson), &cpConfig)
		if err != nil {
			cpErr = fmt.Errorf("while unmarshaling compute pipes json: %s", err)
			goto gotError
		}

		// Prepare the channel registry
		fmt.Println("**& Compute Pipes prepare channel registry")
		channelRegistry := &ChannelRegistry{
			computePipesInputCh: computePipesInputCh,
			inputColumns: headersDKInfo.HeadersPosMap,
			inputChannelSpec: &ChannelSpec{
				Name: "input_row",
				Columns: headersDKInfo.Headers,
			},
			computeChannels: make(map[string]*Channel),
			closedChannels: make(map[string]bool),
		}
		for i := range cpConfig.Channels {
			cm := make(map[string]int)
			for j, c := range cpConfig.Channels[i].Columns {
				cm[c] = j
			}
			channelRegistry.computeChannels[cpConfig.Channels[i].Name] = &Channel{
				channel: make(chan []interface{}),
				columns: cm,
				config: &cpConfig.Channels[i],
			}
		}
		fmt.Println("**& Compute Pipes channel registry ready")
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
			err = prepareOutoutTable(dbpool, tableIdentifier, &cpConfig.OutputTables[i])
			if err != nil {
				cpErr = fmt.Errorf("while preparing output table: %s", err)
				goto gotError	
			}
			outChannel := channelRegistry.computeChannels[cpConfig.OutputTables[i].Key]
			if outChannel == nil {
				cpErr = fmt.Errorf("error: invalid Compute Pipes configuration: Output table %s does not have a channel configuration", 
					cpConfig.OutputTables[i].Name)
				goto gotError	
			}
			wt := WriteTableSource{
				source: outChannel.channel,
				tableIdentifier: tableIdentifier,
				columns: outChannel.config.Columns,
			}
			go wt.writeTable(dbpool, done, computePipesResultCh)
		}
		fmt.Println("**& Compute Pipes output tables ready")

		ctx := BuilderContext{
			cpConfig: &cpConfig,
			channelRegistry: channelRegistry,
			env: envSettings,
		}
		err = ctx.buildComputeGraph()
		if err != nil {
			cpErr = fmt.Errorf("while building the compute graph: %s", err)
			goto gotError	
		}

	}
	// All good!
	return

	gotError:
	log.Println(cpErr)
	computePipesResultCh <- ComputePipesResult{Err: cpErr}
	close(done)
}
