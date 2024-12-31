package compute_pipes

import (
	"fmt"
	"testing"
)

func TestClusterCorrelation01(t *testing.T) {

	clusterCorrelation := NewClusterCorrelation("id", "first_name", 3)
	// Add observation that is correlated around cardinality of 1
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(3)
	clusterCorrelation.AddObservation(3)
	clusterCorrelation.AddObservation(4)

	cramerV, _ := clusterCorrelation.CramerV()
	if cramerV < 0 {
		t.Errorf("Expecting correlated columns, got %v", cramerV)
	}
	fmt.Printf("Got correlation of: %v\n", cramerV)
}

func TestClusterCorrelation02(t *testing.T) {

	clusterCorrelation := NewClusterCorrelation("id", "first_name", 3)
	// Add observation that is correlated around cardinality of 1
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(3)
	clusterCorrelation.AddObservation(3)
	clusterCorrelation.AddObservation(3)
	clusterCorrelation.AddObservation(4)
	clusterCorrelation.AddObservation(4)
	clusterCorrelation.AddObservation(4)

	cramerV, _ := clusterCorrelation.CramerV()
	if cramerV < 0 {
		t.Errorf("Expecting correlated columns, got %v", cramerV)
	}
	fmt.Printf("Got correlation of: %v\n", cramerV)
}

func TestClusterCorrelation03(t *testing.T) {

	clusterCorrelation := NewClusterCorrelation("id", "first_name", 3)
	// Add observation that is correlated around cardinality of 1
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(3)
	clusterCorrelation.AddObservation(3)
	clusterCorrelation.AddObservation(4)
	clusterCorrelation.AddObservation(4)
	clusterCorrelation.AddObservation(4)
	clusterCorrelation.AddObservation(4)
	clusterCorrelation.AddObservation(4)
	clusterCorrelation.AddObservation(4)

	cramerV, _ := clusterCorrelation.CramerV()
	if cramerV < 0 {
		t.Errorf("Expecting correlated columns, got %v", cramerV)
	}
	fmt.Printf("Got correlation of: %v\n", cramerV)
}

func TestClusterCorrelation04(t *testing.T) {

	clusterCorrelation := NewClusterCorrelation("id", "first_name", 10)
	// Add observation that is correlated around cardinality of 1
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(2)

	cramerV, _ := clusterCorrelation.CramerV()
	if cramerV > 0 {
		t.Errorf("Expecting correlated columns, got %v", cramerV)
	}
	fmt.Printf("Got correlation of: %v\n", cramerV)
}
