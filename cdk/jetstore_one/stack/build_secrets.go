package stack

// Build JetStore One Secrets

import (
	awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
	awssm "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
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