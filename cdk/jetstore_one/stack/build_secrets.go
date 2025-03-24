package stack

// Build JetStore One Secrets

import (
	"fmt"
	"os"
	"strconv"

	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	awssm "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-sdk-go-v2/aws"
	constructs "github.com/aws/constructs-go/constructs/v10"
	jsii "github.com/aws/jsii-runtime-go"
)

func (jsComp *JetStoreStackComponents) BuildSecrets(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// Created here since it's needed for all containers
	jsComp.ApiSecret = awssm.NewSecret(stack, props.MkId("apiSecret"), &awssm.SecretProps{
		Description: jsii.String("API secret used for jwt token encryption"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength:          jsii.Number(15),
			IncludeSpace:            jsii.Bool(false),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})

	jsComp.AdminPwdSecret = awssm.NewSecret(stack, props.MkId("adminPwdSecret"), &awssm.SecretProps{
		Description: jsii.String("JetStore UI admin password"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength:          jsii.Number(15),
			IncludeSpace:            jsii.Bool(false),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})

	jsComp.EncryptionKeySecret = awssm.NewSecret(stack, props.MkId("encryptionKeySecret"), &awssm.SecretProps{
		Description: jsii.String("JetStore Encryption Key"),
		GenerateSecretString: &awssm.SecretStringGenerator{
			PasswordLength:          jsii.Number(32),
			IncludeSpace:            jsii.Bool(false),
			ExcludePunctuation:      jsii.Bool(true),
			RequireEachIncludedType: jsii.Bool(true),
		},
	})
}

func (jsComp *JetStoreStackComponents) AddSecretRotationSchedules(scope constructs.Construct, stack awscdk.Stack, props *JetstoreOneStackProps) {

	// Schedule for rotating secrets
	var rotationDays float64
	if len(os.Getenv("JETS_SECRETS_ROTATION_DAYS")) > 0 {
		var err error
		rotationDays, err = strconv.ParseFloat(os.Getenv("JETS_SECRETS_ROTATION_DAYS"), 64)
		if err != nil {
			fmt.Println("while parsing JETS_SECRETS_ROTATION_DAYS:", err)
			rotationDays = 30
		}
	} else {
		rotationDays = 30
	}
	fmt.Println("Using secret rotation schedule of", rotationDays, "days (from env JETS_SECRETS_ROTATION_DAYS)")

	// RDS Secret Rotation Schedule
	jsComp.RdsSecret.AddRotationSchedule(props.MkId("rdsSecretRotation"), &awssm.RotationScheduleOptions{
		AutomaticallyAfter:        awscdk.Duration_Days(aws.Float64(rotationDays)),
		RotateImmediatelyOnUpdate: aws.Bool(false),
		RotationLambda:            jsComp.SecretRotationLambda,
	})

	// API Secret Rotation Schedule
	jsComp.ApiSecret.AddRotationSchedule(props.MkId("apiSecretRotation"), &awssm.RotationScheduleOptions{
		AutomaticallyAfter:        awscdk.Duration_Days(aws.Float64(rotationDays)),
		RotateImmediatelyOnUpdate: aws.Bool(false),
		RotationLambda:            jsComp.SecretRotationLambda,
	})

	// Admin Pwd Secret Rotation Schedule
	jsComp.AdminPwdSecret.AddRotationSchedule(props.MkId("adminPwdSecretRotation"), &awssm.RotationScheduleOptions{
		AutomaticallyAfter:        awscdk.Duration_Days(aws.Float64(rotationDays)),
		RotateImmediatelyOnUpdate: aws.Bool(false),
		RotationLambda:            jsComp.SecretRotationLambda,
	})

	// Encryption Key Secret Rotation Schedule
	jsComp.EncryptionKeySecret.AddRotationSchedule(props.MkId("EncryptKeySecretRotation"), &awssm.RotationScheduleOptions{
		AutomaticallyAfter:        awscdk.Duration_Days(aws.Float64(rotationDays)),
		RotateImmediatelyOnUpdate: aws.Bool(false),
		RotationLambda:            jsComp.SecretRotationLambda,
	})
}
