package compute_pipes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The sharding start step reads an authored .pc.json from the workspace. A document
// without cluster_config used to unmarshal cleanly and then panic on the first
// dereference of ClusterConfig, in prepareCpipesEnv. It must be refused with an
// error naming the file instead, as UnmarshalComputePipesConfig refuses it on the
// coordinator's path.
func TestLoadStartConfigRequiresClusterConfig(t *testing.T) {
	dir := t.TempDir()
	name := "pipes_config/no_cluster.pc.json"
	configFile := filepath.Join(dir, "no_cluster.pc.json")
	err := os.WriteFile(configFile, []byte(`{"reducing_pipes_config": []}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	var cpConfig ComputePipesConfig
	err = loadStartConfig(configFile, name, &cpConfig)
	if err == nil {
		// Show what the start step did next with this document.
		var panicked any
		func() {
			defer func() { panicked = recover() }()
			_, _ = prepareCpipesEnv(&StartComputePipesArgs{}, &CpipesStartup{
				CpConfig:                      cpConfig,
				MainInputSchemaProviderConfig: &SchemaProviderSpec{},
			})
		}()
		t.Fatalf("start step accepted a config without cluster_config; prepareCpipesEnv then panicked: %v", panicked)
	}
	if !strings.Contains(err.Error(), "cluster_config is required in "+name) {
		t.Errorf("error does not name the missing block and the file: %v", err)
	}
}

func TestLoadStartConfigAcceptsClusterConfig(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "with_cluster.pc.json")
	err := os.WriteFile(configFile, []byte(`{"cluster_config": {"max_nbr_partitions": 1}, "reducing_pipes_config": []}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	var cpConfig ComputePipesConfig
	if err = loadStartConfig(configFile, "with_cluster.pc.json", &cpConfig); err != nil {
		t.Fatalf("loadStartConfig refused a config carrying cluster_config: %v", err)
	}
	if cpConfig.ClusterConfig == nil || cpConfig.ClusterConfig.MaxNbrPartitions != 1 {
		t.Fatalf("cluster_config not loaded: %+v", cpConfig.ClusterConfig)
	}
	// The dereference that panicked without the block now runs.
	if _, err = prepareCpipesEnv(&StartComputePipesArgs{}, &CpipesStartup{
		CpConfig:                      cpConfig,
		MainInputSchemaProviderConfig: &SchemaProviderSpec{},
	}); err != nil {
		t.Fatalf("prepareCpipesEnv: %v", err)
	}
}
