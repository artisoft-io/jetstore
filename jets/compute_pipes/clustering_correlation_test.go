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

	mean, variance := clusterCorrelation.MeanAndVariance()
	if mean > 0 {
		t.Errorf("Expecting correlated columns, got %v", mean)
	}
	fmt.Printf("Got correlation of: (%v, %v)\n", mean, variance)
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

	mean, variance := clusterCorrelation.MeanAndVariance()
	if mean > 0 {
		t.Errorf("Expecting correlated columns, got %v", mean)
	}
	fmt.Printf("Got correlation of: (%v, %v)\n", mean, variance)
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

	mean, variance := clusterCorrelation.MeanAndVariance()
	if mean > 0 {
		t.Errorf("Expecting correlated columns, got %v", mean)
	}
	fmt.Printf("Got correlation of: (%v, %v)\n", mean, variance)
}

func TestClusterCorrelation04(t *testing.T) {

	clusterCorrelation := NewClusterCorrelation("id", "first_name", 10)
	// Add observation that is correlated around cardinality of 1
	clusterCorrelation.AddObservation(1)
	clusterCorrelation.AddObservation(2)
	clusterCorrelation.AddObservation(2)

	mean, variance := clusterCorrelation.MeanAndVariance()
	if mean < 0 {
		t.Errorf("Expecting correlated columns, got %v", mean)
	}
	fmt.Printf("Got correlation of: (%v, %v)\n", mean, variance)
}
