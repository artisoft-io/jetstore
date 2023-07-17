package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	iam "github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type BootstrapAWSStackProps struct {
	awscdk.StackProps
}

func NewBootstrapAWSStack(scope constructs.Construct, id string, props *BootstrapAWSStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// The code that defines your stack goes here
	/**
	* Create an Identity provider for GitHub inside your AWS Account. This
	* allows GitHub to present itself to AWS IAM and assume a role.
	*/
	provider := iam.NewOpenIdConnectProvider(stack, jsii.String("JSProvider"), &iam.OpenIdConnectProviderProps{
		Url: jsii.String("https://token.actions.githubusercontent.com"),
		ClientIds: jsii.Strings("sts.amazonaws.com"),
		// Thumbprints no longer needed but no harm leaving them
		Thumbprints: jsii.Strings("6938fd4d98bab03faadb97b34396831e3780aea1", "1c58a3a8518e8759bf075b76b750d4f2df264fcd"),
	})

	/**
	 * Create a principal for the OpenID; which can allow it to assume
	 * deployment roles.
	 */
	GitHubPrincipal := iam.NewOpenIdConnectPrincipal(provider, &map[string]interface{}{
		"StringLike": map[string]interface{}{
			"token.actions.githubusercontent.com:sub": 
			fmt.Sprintf("repo:%s:*", os.Getenv("GH_REPO_NAME"))},
		"StringEquals": map[string]interface{}{
			"token.actions.githubusercontent.com:aud": "sts.amazonaws.com"},
	})

	/**
		* Create a deployment role that has short lived credentials. The only
		* principal that can assume this role is the GitHub Open ID provider.
		*
		* This role is granted authority to assume aws cdk roles; which are created
		* by the aws cdk v2.
	*/
	role := os.Getenv("AWS_ROLE")
	if role == "" {
		role = "github-ci-role"
	}
	iam.NewRole(stack, jsii.String("CDKDeployRole"), &iam.RoleProps{
		AssumedBy: GitHubPrincipal,
		Description: jsii.String("Role assumed by GitHubPrincipal for deploying from CI using aws cdk"),
		RoleName: jsii.String(role),
		MaxSessionDuration: awscdk.Duration_Hours(jsii.Number(1)),
		InlinePolicies: &map[string]iam.PolicyDocument{
			"CdkDeploymentPolicy": iam.NewPolicyDocument(&iam.PolicyDocumentProps{
				AssignSids: jsii.Bool(true),
				Statements: &[]iam.PolicyStatement{
					iam.NewPolicyStatement(&iam.PolicyStatementProps{
						Effect: iam.Effect_ALLOW,
						Actions: jsii.Strings("sts:AssumeRole"),
						Resources: jsii.Strings(fmt.Sprintf("arn:aws:iam::%s:role/cdk-*",	os.Getenv("AWS_ACCOUNT"))),
					}),
					iam.NewPolicyStatement(&iam.PolicyStatementProps{
						Effect: iam.Effect_ALLOW,
						Actions: jsii.Strings(
							"ecr:GetAuthorizationToken",
						),
						Resources: jsii.Strings("*"),
					}),
					iam.NewPolicyStatement(&iam.PolicyStatementProps{
						Effect: iam.Effect_ALLOW,
						Actions: jsii.Strings(
							"ecr:BatchGetImage",
							"ecr:BatchCheckLayerAvailability",
							"ecr:CompleteLayerUpload",
							"ecr:GetDownloadUrlForLayer",
							"ecr:InitiateLayerUpload",
							"ecr:PutImage",
							"ecr:UploadLayerPart",
						),
						Resources: jsii.Strings(fmt.Sprintf("arn:aws:ecr:*:%s:repository/*",	os.Getenv("AWS_ACCOUNT"))),
					}),
					iam.NewPolicyStatement(&iam.PolicyStatementProps{
						Effect: iam.Effect_ALLOW,
						Actions: jsii.Strings(
							"cloudformation:DescribeStacks",
						),
						Resources: jsii.Strings(fmt.Sprintf("arn:aws:cloudformation:%s:%s:stack/CDKToolkit/*", os.Getenv("AWS_REGION"), os.Getenv("AWS_ACCOUNT"))),
					}),
				},
			}),
		},
	})
	
	return stack
}

// Expected Env Variables
// ----------------------
// AWS_ACCOUNT (required)
// AWS_REGION (required)
// AWS_ROLE (default github-ci-role)
// GH_ORG_NAME (required)
// GH_REPO_NAME (required)
// JETS_TAG_NAME_OWNER (optional, stack-level tag name for owner)
// JETS_TAG_VALUE_OWNER (optional, stack-level tag value for owner)
// JETS_TAG_NAME_PROD (optional, stack-level tag name for prod indicator)
// JETS_TAG_VALUE_PROD (optional, stack-level tag value for indicating it's a production env)
// JETS_TAG_NAME_PHI (optional, resource-level tag name for indicating if resource contains PHI data, value true/false)
// JETS_TAG_NAME_PII (optional, resource-level tag name for indicating if resource contains PII data, value true/false)
// JETS_TAG_NAME_DESCRIPTION (optional, resource-level tag name for description of the resource)

func main() {
	defer jsii.Close()
	fmt.Println("Got following env var")
	fmt.Println("env AWS_ACCOUNT:", os.Getenv("AWS_ACCOUNT"))
	fmt.Println("env AWS_REGION:", os.Getenv("AWS_REGION"))
	fmt.Println("env AWS_ROLE:", os.Getenv("AWS_ROLE"))
	fmt.Println("env GH_ORG_NAME:", os.Getenv("GH_ORG_NAME"))
	fmt.Println("env GH_REPO_NAME:", os.Getenv("GH_REPO_NAME"))
	fmt.Println("env JETS_TAG_NAME_OWNER:", os.Getenv("JETS_TAG_NAME_OWNER"))
	fmt.Println("env JETS_TAG_VALUE_OWNER:", os.Getenv("JETS_TAG_VALUE_OWNER"))
	fmt.Println("env JETS_TAG_NAME_PROD:", os.Getenv("JETS_TAG_NAME_PROD"))
	fmt.Println("env JETS_TAG_VALUE_PROD:", os.Getenv("JETS_TAG_VALUE_PROD"))
	fmt.Println("env JETS_TAG_NAME_PHI:", os.Getenv("JETS_TAG_NAME_PHI"))
	fmt.Println("env JETS_TAG_NAME_PII:", os.Getenv("JETS_TAG_NAME_PII"))
	fmt.Println("env JETS_TAG_NAME_DESCRIPTION:", os.Getenv("JETS_TAG_NAME_DESCRIPTION"))

	// Verify that we have all the required env variables
	hasErr := false
	var errMsg []string
	if os.Getenv("AWS_ACCOUNT") == "" || os.Getenv("AWS_REGION") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'AWS_ACCOUNT' and 'AWS_REGION' are required.")		
	}
	if os.Getenv("GH_ORG_NAME") == "" || os.Getenv("GH_REPO_NAME") == "" {
		hasErr = true
		errMsg = append(errMsg, "Env variables 'GH_ORG_NAME' and 'GH_REPO_NAME' are required.")		
	}
	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		os.Exit(1)
	}

	app := awscdk.NewApp(nil)

	// Set stack-level tags
	stackDescription := jsii.String(
		"Create an Identity provider for GitHub inside the AWS Account." +
		" This allows GitHub to present itself to AWS IAM and assume a role to deploy JetStore Platform stack")

	if os.Getenv("JETS_TAG_NAME_OWNER") != "" && os.Getenv("JETS_TAG_VALUE_OWNER") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_OWNER")), jsii.String(os.Getenv("JETS_TAG_VALUE_OWNER")), nil)
	}
	if os.Getenv("JETS_TAG_NAME_PROD") != "" && os.Getenv("JETS_TAG_VALUE_PROD") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_PROD")), jsii.String(os.Getenv("JETS_TAG_VALUE_PROD")), nil)
	}
	if os.Getenv("JETS_TAG_NAME_PHI") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_PHI")), jsii.String("false"), nil)
	}
	if os.Getenv("JETS_TAG_NAME_PII") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_PII")), jsii.String("false"), nil)
	}
	if os.Getenv("JETS_TAG_NAME_DESCRIPTION") != "" {
		awscdk.Tags_Of(app).Add(jsii.String(os.Getenv("JETS_TAG_NAME_DESCRIPTION")), stackDescription, nil)
	}
	NewBootstrapAWSStack(app, "BootstrapAWSStack", &BootstrapAWSStackProps{
		awscdk.StackProps{
			Env: env(),
			Description: stackDescription,
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	// return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("AWS_ACCOUNT")),
		Region:  jsii.String(os.Getenv("AWS_REGION")),
	 }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
