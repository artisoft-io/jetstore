package main

import (
  "github.com/aws/aws-cdk-go/awscdk/v2"
  "github.com/aws/aws-cdk-go/awscdk/v2/awswafv2"
  "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
  "github.com/aws/constructs-go/constructs/v10"
  "github.com/aws/jsii-runtime-go"
)

type WafAlbStackProps struct {
  awscdk.StackProps
}

func NewWafAlbStack(scope constructs.Construct, id string, props *WafAlbStackProps) awscdk.Stack {
  stack := awscdk.NewStack(scope, &id, &props.StackProps)

  // 1. Create a Web ACL (WAFv2)
  webAcl := awswafv2.NewCfnWebACL(stack, jsii.String("MyWebACL"), &awswafv2.CfnWebACLProps{
    DefaultAction: &awswafv2.CfnWebACL_DefaultActionProperty{
      Allow: &awswafv2.CfnWebACL_AllowActionProperty{},
    },
    Scope:        jsii.String("REGIONAL"),
    VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
      SampledRequestsEnabled: jsii.Bool(true),
      CloudWatchMetricsEnabled: jsii.Bool(true),
      MetricName: jsii.String("WebACLMetric"),
    },
    Name: jsii.String("MyWebACL"),
    Rules: []awswafv2.CfnWebACL_RuleProperty{
      {
        Name:     jsii.String("BlockBadBots"),
        Priority: jsii.Number(0),
        Action: &awswafv2.CfnWebACL_RuleActionProperty{
          Block: &awswafv2.CfnWebACL_BlockActionProperty{},
        },
        Statement: &awswafv2.CfnWebACL_StatementProperty{
          ManagedRuleGroupStatement: &awswafv2.CfnWebACL_ManagedRuleGroupStatementProperty{
            Name: jsii.String("AWSManagedRulesCommonRuleSet"),
            VendorName: jsii.String("AWS"),
          },
        },
        VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
          SampledRequestsEnabled: jsii.Bool(true),
          CloudWatchMetricsEnabled: jsii.Bool(true),
          MetricName: jsii.String("CommonRuleSet"),
        },
      },
    },
  })

  // 2. Create or import an existing ALB (for simplicity, assume new)
  alb := awselasticloadbalancingv2.NewApplicationLoadBalancer(stack, jsii.String("MyALB"), &awselasticloadbalancingv2.ApplicationLoadBalancerProps{
    Vpc:      nil      /* your VPC here */,
    InternetFacing: jsii.Bool(true),
  })

  // 3. Associate the Web ACL with the ALB
  awswafv2.NewCfnWebACLAssociation(stack, jsii.String("WebACLAssociation"), &awswafv2.CfnWebACLAssociationProps{
    ResourceArn: alb.LoadBalancerArn(),
    WebAclArn:   webAcl.AttrArn(),
  })

  return stack
}

func main() {
  app := awscdk.NewApp(nil)

  NewWafAlbStack(app, "WafAlbStack", &WafAlbStackProps{
    StackProps: awscdk.StackProps{
      Env: env(),
    },
  })

  app.Synth(nil)
}

// provide your AWS env details
func env() *awscdk.Environment {
  return &awscdk.Environment{
    Account: jsii.String("123456789012"),
    Region:  jsii.String("us-west-2"),
  }
}
