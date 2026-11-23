package stack

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	jsii "github.com/aws/jsii-runtime-go"
)

// Functions to create the SecurityGroup giving access to GitHub

// githubMetaURL publishes GitHub's IP ranges; the "git" key holds the CIDRs for git over https.
const githubMetaURL = "https://api.github.com/meta"

// maxGithubCidrs caps the egress rules taken from GitHub's list: the security group's default
// quota is 60 rules, and GitHub publishes more IPv4 git CIDRs than that leaves room for.
// The list reaches 51, which is the intended number.
const maxGithubCidrs = 51

// getGithubIps returns the IPv4 CIDRs GitHub publishes for git over https. The request is
// unauthenticated unless GITHUB_TOKEN is set, and an unauthenticated one is limited to 60 an
// hour per IP address, which a shared CI runner can exhaust; GitHub then answers 403 with a
// message and no "git" key. The synth stops with that answer rather than a type assertion.
func getGithubIps() *[]*string {
	ipList, err := fetchGithubGitCidrs(http.DefaultClient, githubMetaURL, os.Getenv("GITHUB_TOKEN"), 3, 5*time.Second)
	if err != nil {
		panic(fmt.Errorf("while getting github ips for git integration: %v", err))
	}
	fmt.Println("Done getting github ips for git integration")
	return jsii.Strings(ipList...)
}

// fetchGithubGitCidrs gets url and returns its IPv4 git CIDRs. A transport error or a 5xx is
// tried again, up to attempts times, delay apart; any other status other than 200 is an error
// naming the status, GitHub's message and, when the rate limit is spent, when it resets.
func fetchGithubGitCidrs(client *http.Client, url, token string, attempts int, delay time.Duration) ([]string, error) {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			log.Printf("github meta: attempt %d of %d after: %v", attempt, attempts, lastErr)
			time.Sleep(delay)
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("GET %s: %v", url, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading %s: %v", url, err)
			continue
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("GET %s: %s: %s", url, resp.Status, githubMessage(body))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("GET %s: %s: %s", url, resp.Status, githubMessage(body))
			if resp.Header.Get("X-RateLimit-Remaining") == "0" {
				if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
					msg += fmt.Sprintf("; the rate limit resets at %s", time.Unix(reset, 0).UTC().Format(time.RFC3339))
				}
				if token == "" {
					msg += "; run again after that, or set GITHUB_TOKEN for an authenticated request"
				}
			}
			return nil, errors.New(msg)
		}
		return parseGithubGitCidrs(body)
	}
	return nil, lastErr
}

// parseGithubGitCidrs reads the "git" key of a meta response and keeps its IPv4 CIDRs, at most
// maxGithubCidrs of them.
func parseGithubGitCidrs(body []byte) ([]string, error) {
	var meta struct {
		Git []string `json:"git"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("decoding %s: %v", githubMetaURL, err)
	}
	if len(meta.Git) == 0 {
		return nil, fmt.Errorf("%s has no \"git\" CIDRs: %s", githubMetaURL, githubMessage(body))
	}
	ipList := make([]string, 0, maxGithubCidrs)
	for _, ip := range meta.Git {
		if strings.Contains(ip, ":") {
			continue // ip v6
		}
		ipList = append(ipList, ip)
		if len(ipList) >= maxGithubCidrs {
			log.Printf("More than %d GitHub IPs retrieved, stopping early", maxGithubCidrs-1)
			break
		}
	}
	if len(ipList) == 0 {
		return nil, fmt.Errorf("%s lists no IPv4 \"git\" CIDR", githubMetaURL)
	}
	return ipList, nil
}

// githubMessage is the "message" of a GitHub error body, or the start of the body.
func githubMessage(body []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		return e.Message
	}
	if len(body) > 200 {
		return string(body[:200]) + "..."
	}
	return string(body)
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

func NewGitAccessSecurityGroup(stack awscdk.Stack, vpc awsec2.IVpc) awsec2.SecurityGroup {
	securityGroup := awsec2.NewSecurityGroup(stack, jsii.String("GitAccessSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		Description:      jsii.String("Allow network access to Git SCM"),
		AllowAllOutbound: jsii.Bool(false),
	})
	// Add access to GIT SCM
	gitScm := os.Getenv("JETS_GIT_ACCESS")
	if strings.Contains(gitScm, "github") {
		fmt.Println("Providing access to github for git integration")
		for _, cidr := range *getGithubIps() {
			securityGroup.AddEgressRule(awsec2.Peer_Ipv4(cidr), awsec2.Port_Tcp(jsii.Number(443)),
				jsii.String("allow https access to github repository"), jsii.Bool(false))
		}
	}
	if strings.Contains(gitScm, "bitbucket") {
		fmt.Println("Providing access to bitbucket for git integration")
		for _, cidr := range *getBitbucketIps() {
			securityGroup.AddEgressRule(awsec2.Peer_Ipv4(cidr), awsec2.Port_Tcp(jsii.Number(443)),
				jsii.String("allow https access to bitbucket repository"), jsii.Bool(false))
		}
	}
	return securityGroup
}
