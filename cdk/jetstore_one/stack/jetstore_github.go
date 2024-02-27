package stack

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

func getBitbucketIps() *[]*string {
	results := jsii.Strings(
		"104.192.136.0/21",
		"185.166.140.0/22",
		"18.205.93.0/25",
		"18.234.32.128/25",
		"13.52.5.0/25",
	)
	return results
}

func NewGitAccessSecurityGroup(stack awscdk.Stack, vpc awsec2.Vpc) awsec2.SecurityGroup {
	securityGroup := awsec2.NewSecurityGroup(stack, jsii.String("GitAccessSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc: vpc,
		Description: jsii.String("Allow network access to Git SCM"),
		AllowAllOutbound: jsii.Bool(false),
	})
	// Add access to GIT SCM
	gitScm := os.Getenv("JETS_GIT_ACCESS")
	if strings.Contains(gitScm, "github") {
		fmt.Println("Providing access to github for git integration")
		for _,cdr := range (*getGithubIps()) {
			securityGroup.AddEgressRule(awsec2.Peer_Ipv4(cdr), awsec2.Port_Tcp(jsii.Number(443)), 
				jsii.String("allow https access to github repository"), jsii.Bool(false))
		}	
	}
	if strings.Contains(gitScm, "bitbucket") {
		fmt.Println("Providing access to bitbucket for git integration")
		for _,cdr := range (*getBitbucketIps()) {
			securityGroup.AddEgressRule(awsec2.Peer_Ipv4(cdr), awsec2.Port_Tcp(jsii.Number(443)), 
				jsii.String("allow https access to bitbucket repository"), jsii.Bool(false))
		}	
	}
	return securityGroup
}