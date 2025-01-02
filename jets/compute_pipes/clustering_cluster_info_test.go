package compute_pipes

import (
	"fmt"
	"testing"
)

func TestClusterInfo01(t *testing.T) {
	// 2 clusters without tags should be able to merge
	cluster := NewClusterInfo(nil, nil)
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil)}
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
	// Only one cluster without tags should not be able to merge with cluster
	// having tags
	cluster.clusterTags["tag"] = true
	if canMerge(cluster, 0, clusters) {
		t.Errorf("NOT expecting to be able to merge")
	}
	// 2 clusters with tags should be able to merge
	clusters[0].clusterTags["tag"] = true
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
}

func TestClusterInfo02(t *testing.T) {
	// 2 clusters with tags, with another one without tags should be able to merge
	cluster := NewClusterInfo(nil, nil)
	cluster.clusterTags["tag"] = true
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil), NewClusterInfo(nil, nil)}
	clusters[0].clusterTags["tag"] = true
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
}

func TestClusterInfo03(t *testing.T) {
	// 2 clusters 1 with tags and 1 without, and another one without tags should be able to merge
	cluster := NewClusterInfo(nil, nil)
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil), NewClusterInfo(nil, nil)}
	clusters[0].clusterTags["tag"] = true
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
}

func TestClusterInfo04(t *testing.T) {
	// 2 clusters 1 with tags and 1 without, and another one with tags should not be able to merge
	cluster := NewClusterInfo(nil, nil)
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil), NewClusterInfo(nil, nil)}
	clusters[0].clusterTags["tag"] = true
	clusters[1].clusterTags["tag"] = true
	if canMerge(cluster, 0, clusters) {
		t.Errorf("NOT expecting to be able to merge")
	}
}

func TestMakingCluster01(t *testing.T) {

  columnsCorrelation := make([]*ColumnCorrelation, 0)
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "PROVIDER_CITY", 186, 118, 4858))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "PROVIDER_ADDRESS_LINE_1", 186, 119, 4858))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "MEMBER_LAST_NAME", 286, 233, 4739))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "SUBSCRIBER_SSN", 492, 257, 4402))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "MEMBER_FIRST_NAME", 492, 258, 4409))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "MEMBER_LAST_NAME", 492, 260, 4409))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "PROVIDER_CITY", 286, 340, 4736))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "PROVIDER_TIN", 879, 258, 3505))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "PROVIDER_NPI_NUMBER", 879, 258, 3505))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "PROVIDER_TIN", 930, 263, 3562))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "MEMBER_FIRST_NAME", 286, 387, 4739))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "member_id", 286, 390, 4739))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "PROVIDER_CITY", 492, 371, 4406))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "PHARMACY_ID_NUMBER", 286, 418, 4739))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "PROVIDER_ADDRESS_LINE_1", 286, 419, 4736))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "SUBSCRIBER_SSN", 186, 448, 4835))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "MEMBER_LAST_NAME", 186, 476, 4858))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "PHARMACY_ID_NUMBER", 492, 433, 4409))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "PROVIDER_ADDRESS_LINE_1", 492, 433, 4406))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "PROVIDER_NPI_NUMBER", 656, 475, 4025))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "PROVIDER_CITY", 656, 494, 4023))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "MEMBER_FIRST_NAME", 186, 619, 4858))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "member_id", 186, 628, 4858))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "PROVIDER_CITY", 879, 459, 3498))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "SUBSCRIBER_SSN", 930, 466, 3550))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "PROVIDER_CITY", 930, 467, 3555))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "SUBSCRIBER_SSN", 879, 459, 3493))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "MEMBER_LAST_NAME", 930, 474, 3562))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "MEMBER_LAST_NAME", 879, 467, 3505))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "MEMBER_FIRST_NAME", 930, 528, 3562))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "MEMBER_FIRST_NAME", 879, 520, 3505))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "member_id", 930, 531, 3562))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "member_id", 879, 523, 3505))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "SUBSCRIBER_SSN", 656, 619, 4005))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "MEMBER_LAST_NAME", 656, 638, 4025))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "PHARMACY_ID_NUMBER", 930, 572, 3562))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_NPI_NUMBER", "PROVIDER_ADDRESS_LINE_1", 930, 571, 3555))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "PHARMACY_ID_NUMBER", 879, 563, 3505))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PRESCRIBER_ID", "PROVIDER_ADDRESS_LINE_1", 879, 562, 3498))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "PHARMACY_ID_NUMBER", 656, 687, 4025))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "PROVIDER_ADDRESS_LINE_1", 656, 688, 4023))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "MEMBER_FIRST_NAME", 656, 700, 4025))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PROVIDER_TIN", "member_id", 656, 712, 4025))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "PROVIDER_TIN", 492, 889, 4409))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "PROVIDER_TIN", 286, 990, 4739))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("member_id", "PROVIDER_NPI_NUMBER", 492, 950, 4409))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("SUBSCRIBER_SSN", "PROVIDER_NPI_NUMBER", 286, 1080, 4739))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "PROVIDER_TIN", 186, 1121, 4858))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("PHARMACY_ID_NUMBER", "PROVIDER_NPI_NUMBER", 186, 1262, 4858))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "PROVIDER_CITY", 1404, 764, 2538))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "PHARMACY_ID_NUMBER", 1404, 952, 2539))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "PROVIDER_ADDRESS_LINE_1", 1404, 953, 2538))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "SUBSCRIBER_SSN", 1404, 963, 2514))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "MEMBER_LAST_NAME", 1404, 982, 2539))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "MEMBER_FIRST_NAME", 1404, 1004, 2539))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "member_id", 1404, 1015, 2539))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "PROVIDER_TIN", 1404, 1084, 2539))
  columnsCorrelation = append(columnsCorrelation, NewColumnCorrelation("NDC", "PROVIDER_NPI_NUMBER", 1404, 1138, 2539))

  columnClassificationMap := make(map[string]string)
  columnClassificationMap["CLAIM_NUMBER"] =	"id"
  columnClassificationMap["SUBSCRIBER_SSN"] =	"ssn"
  columnClassificationMap["member_id"] =	"ssn"
  columnClassificationMap["MEMBER_FIRST_NAME"] =	"first_name"
  columnClassificationMap["MEMBER_LAST_NAME"] =	"last_name"
  columnClassificationMap["NDC"] =	"id"
  columnClassificationMap["PRESCRIBER_ID"] =	"id"
  columnClassificationMap["PHARMACY_ID_NUMBER"] =	"npi"
  columnClassificationMap["PROVIDER_TIN"] =	"ssn"
  columnClassificationMap["PROVIDER_NPI_NUMBER"] =	"npi"
  columnClassificationMap["PROVIDER_ADDRESS_LINE_1"] =	"street_1"
  columnClassificationMap["PROVIDER_ADDRESS_LINE_2"] =	"street_2"
  columnClassificationMap["PROVIDER_CITY"] =	"city"

  config := &ClusteringSpec{
    TransitiveDataClassification: []string{"id","ssn","npi","dob","first_name","last_name","street_1"},
    ClusterDataSubclassification: []string{"npi"},
  }
  // Test making the clusters
  clusters := MakeClusters(columnsCorrelation, columnClassificationMap, config)
	fmt.Println("Clustering Complete, the clusters are:")
	for _, cluster := range clusters {
		fmt.Println(cluster)
	}
  if len(clusters) == 0 {
    t.Errorf("That's it")
  }
}