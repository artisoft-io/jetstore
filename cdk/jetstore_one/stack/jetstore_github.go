package stack

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	jsii "github.com/aws/jsii-runtime-go"
)

// Functions to create the SecurityGroup giving access to GitHub

func getGithubIps() *[]*string {
	// Issue an HTTP GET request
	resp, err := http.Get("https://api.github.com/meta")
	if err != nil {
		panic(fmt.Errorf("while http GET to https://api.github.com/meta:%v", err))
	}
	defer resp.Body.Close()

	// Parse the response body as json
	d := json.NewDecoder(resp.Body)
	ipList := make([]string, 0)
	for {
		bodyData := make(map[string]interface{})
		err := d.Decode(&bodyData)
		if err == io.EOF {
			 break
		}
		if err != nil {
			log.Fatalf("Error decoding body of https://api.github.com/meta: %v", err)
		}
		// do stuff with "jsonObject"...
		for _,ipi := range bodyData["git"].([]interface{}) {
		ip := ipi.(string)
		// check for ip v6
		if !strings.Contains(ip, ":") {
			ipList = append(ipList, ip)
		}			
		}
	}	
	fmt.Println("Done getting github ip for git integration")
	results := jsii.Strings(ipList...)
	return results
}

func NewGithubAccessSecurityGroup(stack awscdk.Stack, vpc awsec2.Vpc) awsec2.SecurityGroup {
	securityGroup := awsec2.NewSecurityGroup(stack, jsii.String("GithubAccessSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc: vpc,
		Description: jsii.String("Allow network access to GitHub"),
		AllowAllOutbound: jsii.Bool(false),
	})
	// Add access to Github
	cdrs := getGithubIps()
	for _,cdr := range *cdrs {
		securityGroup.AddEgressRule(awsec2.Peer_Ipv4(cdr), awsec2.Port_Tcp(jsii.Number(443)), 
			jsii.String("allow https access to github repository"), jsii.Bool(false))
	}	
	return securityGroup
}