package stack

// Build JetStore Once ECS Tasks

import (
	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awswafv2"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

// functions to build the Web ACL
// Attach WAF to the ELB

func (jsComp *JetStoreStackComponents) BuildWAFV2(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {
	// 1. Create a Web ACL (WAFv2)
	jsComp.WebAcl = awswafv2.NewCfnWebACL(stack, jsii.String("MyWebACL"), &awswafv2.CfnWebACLProps{
		DefaultAction: &awswafv2.CfnWebACL_DefaultActionProperty{
			Allow: &awswafv2.CfnWebACL_AllowActionProperty{},
		},
		Scope: jsii.String("REGIONAL"),
		VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
			SampledRequestsEnabled:   jsii.Bool(true),
			CloudWatchMetricsEnabled: jsii.Bool(true),
			MetricName:               jsii.String("WebACLMetric"),
		},
		Rules: []awswafv2.CfnWebACL_RuleProperty{
			{
        Name:     jsii.String("BlockBadBots"),
				Priority: jsii.Number(0),
				Statement: &awswafv2.CfnWebACL_StatementProperty{
					ManagedRuleGroupStatement: &awswafv2.CfnWebACL_ManagedRuleGroupStatementProperty{
						Name:       jsii.String("AWSManagedRulesCommonRuleSet"),
						VendorName: jsii.String("AWS"),
					},
				},
				VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
					SampledRequestsEnabled:   jsii.Bool(true),
					CloudWatchMetricsEnabled: jsii.Bool(true),
					MetricName:               jsii.String("CommonRuleSet"),
				},
			},
		},
	})

	// 2. Associate the Web ACL with the ALB
  awswafv2.NewCfnWebACLAssociation(stack, jsii.String("WebACLAssociation"), &awswafv2.CfnWebACLAssociationProps{
    ResourceArn: jsComp.UiLoadBalancer.LoadBalancerArn(),
    WebAclArn:   jsComp.WebAcl.AttrArn(),
  })
}
