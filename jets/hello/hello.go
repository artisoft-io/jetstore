package main

// import "fmt"

import (
	"fmt"
	"github.com/artisoft-io/jetstore/jets/bridge"
)

func main() {
	fmt.Println("Hello Jets!")
	
	jr_name := "jetrule_rete_test.db"
	lk_name := ""
	fmt.Println("Loading with LoadJetRules...")
	js, err := bridge.LoadJetRules(jr_name, lk_name)
	if err != nil {
		fmt.Println("We got a Problem:", err)
	}
	fmt.Println("LOADED:", jr_name)
	rs, err := bridge.NewReteSession(js, "ms_factory_test1.jr")
	if err != nil {
		fmt.Println("We got a Problem starting ReteSession:", err)
	}
	fmt.Println("ReteSession Started")

	iClaim, err := bridge.NewResource(rs, "iclaim")
	if err != nil {
		fmt.Println("Error NewResource:", err)
	}
	rdf_type, err := bridge.NewResource(rs, "rdf:type")
	if err != nil {
		fmt.Println("Error NewResource:", err)
	}
	Claim, err := bridge.NewResource(rs, "hc:Claim")
	if err != nil {
		fmt.Println("Error NewResource:", err)
	}

	name, err := iClaim.GetName()
	if err != nil {
		fmt.Println("Error GetName:", err)
	}
	if name == "iclaim" {
		fmt.Println("As expected name",name, "is same as 'iclaim'")
	}

	ret, err := rs.Insert(iClaim, rdf_type, Claim)
	if err != nil {
		fmt.Println("Error Insert:", err)
	}
	if ret > 0 {
		fmt.Println("Triple Inserted as expected!")
	}

	err = rs.ExecuteRules()
	if err != nil {
		fmt.Println("Error ExecuteRules:", err)
	}

	fmt.Println("The ReteSession Contains, after ExecuteRules:")
	itor, err := rs.FindAll()
	if err != nil {
		fmt.Println("Error FindAll:", err)
	}
	for !itor.IsEnd() {
		fmt.Println("    (", itor.GetSubject().AsText(), ", ", itor.GetPredicate().AsText(), ", ", itor.GetObject().AsText(), ")")
		itor.Next()
	}
	fmt.Println("Done, releasing iterator")
	bridge.ReleaseIterator(itor)

	BaseClaim, err := bridge.NewResource(rs, "hc:BaseClaim")
	if err != nil {
		fmt.Println("Error NewResource:", err)
	}
	fmt.Println("Chech that we inferred the triple (", iClaim.AsText(), ", ", rdf_type.AsText(), ", ", BaseClaim.AsText())
	isc, err := rs.Contains(iClaim, rdf_type, BaseClaim)
	if err != nil {
		fmt.Println("Error Contains:", err)
	}
	if isc > 0 {
		fmt.Println("YES it's there as expected!!")
	}
	fmt.Println("Done for now! Releasing ReteSession")
	err = bridge.ReleaseReteSession(rs)
	if err != nil {
		fmt.Println("Error ReleaseReteSession:", err)
	}

	fmt.Println("Releasing MetaStore")
	err = bridge.ReleaseJetRules(js)
	if err != nil {
		fmt.Println("We got a Problem while releasing jetrules:", err)
	}

	fmt.Println("That's All Folks!")
}