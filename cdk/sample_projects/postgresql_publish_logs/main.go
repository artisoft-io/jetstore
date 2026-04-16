package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type AuroraPostgreSQLStackProps struct {
	awscdk.StackProps
}

type AuroraPostgreSQLStack struct {
	awscdk.Stack
}

func NewAuroraPostgreSQLStack(scope constructs.Construct, id string, props *AuroraPostgreSQLStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// Create VPC for the Aurora cluster
	vpc := awsec2.NewVpc(stack, jsii.String("AuroraVPC"), &awsec2.VpcProps{
		MaxAzs:      jsii.Number(2),
		NatGateways: jsii.Number(1),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
				CidrMask:   jsii.Number(24),
			},
			{
				Name:       jsii.String("private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
				CidrMask:   jsii.Number(24),
			},
			{
				Name:       jsii.String("isolated"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
				CidrMask:   jsii.Number(24),
			},
		},
	})

	// Create DB subnet group
	subnetGroup := awsrds.NewSubnetGroup(stack, jsii.String("AuroraSubnetGroup"), &awsrds.SubnetGroupProps{
		Description: jsii.String("Subnet group for Aurora PostgreSQL cluster"),
		Vpc:         vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
		},
	})

	// Create security group for Aurora cluster
	securityGroup := awsec2.NewSecurityGroup(stack, jsii.String("AuroraSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		Description:      jsii.String("Security group for Aurora PostgreSQL cluster"),
		AllowAllOutbound: jsii.Bool(false),
	})

	// Allow PostgreSQL connections from within VPC
	securityGroup.AddIngressRule(
		awsec2.Peer_Ipv4(vpc.VpcCidrBlock()),
		awsec2.Port_Tcp(jsii.Number(5432)),
		jsii.String("Allow PostgreSQL connections from VPC"),
		jsii.Bool(false),
	)

	// Create Aurora PostgreSQL cluster with CloudWatch logs enabled
	cluster := awsrds.NewDatabaseCluster(stack, jsii.String("AuroraPostgreSQLCluster"), &awsrds.DatabaseClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
			Version: awsrds.AuroraPostgresEngineVersion_VER_16_1(),
		}),
		Credentials: awsrds.Credentials_FromGeneratedSecret(jsii.String("postgres"), &awsrds.CredentialsBaseOptions{
			SecretName: jsii.String("aurora-postgresql-credentials"),
		}),
		InstanceProps: &awsrds.InstanceProps{
			InstanceType: awsec2.InstanceType_Of(awsec2.InstanceClass_BURSTABLE4_GRAVITON, awsec2.InstanceSize_MEDIUM),
			Vpc:          vpc,
			SecurityGroups: &[]awsec2.ISecurityGroup{
				securityGroup,
			},
		},
		Instances:   jsii.Number(2), // Writer + 1 Reader
		SubnetGroup: subnetGroup,

		// Enable CloudWatch Logs exports
		CloudwatchLogsExports: &[]*string{
			jsii.String("postgresql"),
		},

		// Optional: Configure CloudWatch Logs retention
		CloudwatchLogsRetention: awslogs.RetentionDays_THREE_MONTHS,

		// Database configuration
		DefaultDatabaseName: jsii.String("myapp"),

		// Backup configuration
		Backup: &awsrds.BackupProps{
			Retention:       awscdk.Duration_Days(jsii.Number(7)),
			PreferredWindow: jsii.String("03:00-04:00"),
		},

		// Maintenance window
		PreferredMaintenanceWindow: jsii.String("sun:04:00-sun:05:00"),

		// Enable deletion protection for production
		DeletionProtection: jsii.Bool(false), // Set to true for production

		// Storage encryption
		StorageEncrypted: jsii.Bool(true),

		// Monitoring
		MonitoringInterval: awscdk.Duration_Seconds(jsii.Number(60)),

		// Create parameter group for additional PostgreSQL logging configuration
		ParameterGroup: awsrds.NewParameterGroup(stack, jsii.String("AuroraPostgreSQLParameterGroup"), &awsrds.ParameterGroupProps{
			Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
				Version: awsrds.AuroraPostgresEngineVersion_VER_16_1(),
			}),
			Description: jsii.String("Parameter group for Aurora PostgreSQL with enhanced logging"),
			Parameters: &map[string]*string{
				"log_statement":               jsii.String("all"),
				"log_min_duration_statement":  jsii.String("1000"), // Log queries taking more than 1 second
				"log_connections":             jsii.String("1"),
				"log_disconnections":          jsii.String("1"),
				"log_lock_waits":              jsii.String("1"),
				"log_temp_files":              jsii.String("0"),
				"log_autovacuum_min_duration": jsii.String("0"),
			},
		}),
	})

	// Output the cluster endpoint
	awscdk.NewCfnOutput(stack, jsii.String("ClusterEndpoint"), &awscdk.CfnOutputProps{
		Value:       cluster.ClusterEndpoint().Hostname(),
		Description: jsii.String("Aurora PostgreSQL Cluster Endpoint"),
	})

	// Output the reader endpoint
	awscdk.NewCfnOutput(stack, jsii.String("ReaderEndpoint"), &awscdk.CfnOutputProps{
		Value:       cluster.ClusterReadEndpoint().Hostname(),
		Description: jsii.String("Aurora PostgreSQL Reader Endpoint"),
	})

	// Output the CloudWatch Log Group name
	awscdk.NewCfnOutput(stack, jsii.String("LogGroupName"), &awscdk.CfnOutputProps{
		Value:       jsii.String("/aws/rds/cluster/" + *cluster.ClusterIdentifier() + "/postgresql"),
		Description: jsii.String("CloudWatch Log Group for PostgreSQL logs"),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewAuroraPostgreSQLStack(app, "AuroraPostgreSQLStack", &AuroraPostgreSQLStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String("123456789012"), // Replace with your account ID
		Region:  jsii.String("us-east-1"),    // Replace with your preferred region
	}
}
