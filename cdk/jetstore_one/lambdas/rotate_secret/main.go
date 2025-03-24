package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Env variable:
// JETS_DSN_SECRET
// AWS_API_SECRET
// AWS_JETS_ADMIN_PWD_SECRET
// JETS_ENCRYPTION_KEY_SECRET
// JETS_REGION
// JETS_ADMIN_EMAIL

// Lambda function to rotate secret
// Based on https://github.com/aws-samples/aws-secrets-manager-rotation-lambdas/blob/master/SecretsManagerRotationTemplate/lambda_function.py

var (
	jetsDnsSecret           string = os.Getenv("JETS_DSN_SECRET")
	awsApiSecret            string = os.Getenv("AWS_API_SECRET")
	awsJetsAdminPwdSecret   string = os.Getenv("AWS_JETS_ADMIN_PWD_SECRET")
	jetsEncryptionKeySecret string = os.Getenv("JETS_ENCRYPTION_KEY_SECRET")
	jetsAdminEmail          string = os.Getenv("JETS_ADMIN_EMAIL")
	secretBeingRotated      string
)

// Input Event as json:
//
//	{
//		"Step" : "request.type",
//		"SecretId" : "string",
//		"ClientRequestToken" : "string",
//		"RotationToken" : "string"
//	}

type RotateSecretEvent struct {
	Step               string `json:"Step"`
	SecretId           string `json:"SecretId"`
	ClientRequestToken string `json:"ClientRequestToken"`
	RotationToken      string `json:"RotationToken"`
}

func handler(event RotateSecretEvent) error {
	if len(jetsDnsSecret) == 0 {
		log.Panic("Missing env var JETS_DSN_SECRET")
	}
	if len(awsApiSecret) == 0 {
		log.Panic("Missing env var AWS_API_SECRET")
	}
	if len(awsJetsAdminPwdSecret) == 0 {
		log.Panic("Missing env var AWS_JETS_ADMIN_PWD_SECRET")
	}
	if len(jetsEncryptionKeySecret) == 0 {
		log.Panic("Missing env var JETS_ENCRYPTION_KEY_SECRET")
	}
	if len(jetsAdminEmail) == 0 {
		log.Panic("Missing env var JETS_ADMIN_EMAIL")
	}

	log.Printf("RotateSecret Lambda Called for secret %s at step %s for version %s",
		event.SecretId, event.Step, event.ClientRequestToken)

	// Get the Secret Manager client
	smClient, err := awsi.NewSecretManagerClient()
	if err != nil {
		return err
	}

	// Keep track of the secret being rotated
	switch {
	case strings.Contains(event.SecretId, jetsDnsSecret):
		secretBeingRotated = jetsDnsSecret

	case strings.Contains(event.SecretId, awsApiSecret):
		secretBeingRotated = awsApiSecret

	case strings.Contains(event.SecretId, awsJetsAdminPwdSecret):
		secretBeingRotated = awsJetsAdminPwdSecret

	case strings.Contains(event.SecretId, jetsEncryptionKeySecret):
		secretBeingRotated = jetsEncryptionKeySecret

	default:
		return fmt.Errorf("error: unknown secret %s to rotate", event.SecretId)
	}

	// Make sure the version is staged correctly
	secretInfo, err := smClient.DescribeSecret(event.SecretId)
	if err != nil {
		return fmt.Errorf("while describing secret %s: %v", event.SecretId, err)
	}
	if secretInfo.RotationEnabled == nil || !*secretInfo.RotationEnabled {
		return fmt.Errorf("error: secret %s is not enabled for rotation", event.SecretId)
	}
	// Get the version with the label of event.ClientRequestToken
	stageLabels := secretInfo.VersionIdsToStages[event.ClientRequestToken]
	if len(stageLabels) == 0 {
		return fmt.Errorf("error: secret %s at version %s does not have stage labels", event.SecretId, event.ClientRequestToken)
	}

	// Check if secret version is already AWSCURRENT
	hasPending := false
	for _, label := range secretInfo.VersionIdsToStages[event.ClientRequestToken] {
		if label == "AWSCURRENT" {
			log.Printf("Secret version %s already set as AWSCURRENT for secret %s.", event.ClientRequestToken, event.SecretId)
			return nil
		}
		if label == "AWSPENDING" {
			hasPending = true
		}
	}
	// Make sure the secret version in the AWSPENDING
	if !hasPending {
		return fmt.Errorf("secret version %s not set as AWSPENDING for rotation of secret %s", event.ClientRequestToken,
			event.SecretId)
	}

	switch event.Step {
	case "create_secret", "createSecret":
		return CreateSecret(smClient, &event)

	case "set_secret", "setSecret":
		return SetSecret(smClient, &event)

	case "test_secret", "testSecret":
		err = TestSecret(smClient, &event)
		if err != nil {
			log.Printf("Failed to test secret %s: %v\n", event.SecretId, err)
			if secretBeingRotated == jetsDnsSecret {
				log.Println("Reverting database passord to previous value")
				previousValue, err2 := smClient.GetSecretValue(jetsDnsSecret, "AWSPREVIOUS")
				if err2 != nil {
					err2 = fmt.Errorf("failed to get previous value of secret %s: %v", jetsDnsSecret, err2)
					log.Println(err2)
					return err2
				}
				dbpool, err2 := OpenDbConn(previousValue)
				if err2 != nil {
					return err2
				}
				// revert the postgres user password in database
				var m map[string]any
				err2 = json.Unmarshal([]byte(previousValue), &m)
				if err2 != nil {
					return fmt.Errorf("while unmarshalling previous value of secret %s: %v", event.SecretId, err2)
				}
				_, err2 = dbpool.Exec(context.Background(),
					fmt.Sprintf("ALTER USER %v PASSWORD '%v';", m["username"], m["password"]))
				if err2 != nil {
					return fmt.Errorf("error: failed to revert the user password in database: %v", err2)
				}
			}
		}
		return err

	case "finish_secret", "finishSecret":
		// Get the AWSCURRENT version of the secret
		for version, labels := range secretInfo.VersionIdsToStages {
			for _, label := range labels {
				if label == "AWSCURRENT" {
					log.Printf("Move AWSPENDING version to AWSCURRENT for secret %s.", event.SecretId)
					err = smClient.UpdateSecretVersionStage(event.SecretId, "AWSCURRENT", event.ClientRequestToken, version)
					if err != nil {
						return fmt.Errorf("while moving the AWSPENDING version as AWSCURRENT for secret %s: %v", event.SecretId, err)
					}
					// Update database with last update time
					return RecordSecretRotation()
				}
			}
		}
		return fmt.Errorf("error: AWSCURRENT version not found for secret %s", event.SecretId)

	default:
		return fmt.Errorf("error: unknown step %s while rotating secret %s", event.Step, event.SecretId)
	}
}

func CreateSecret(smClient *awsi.SecretManagerClient, event *RotateSecretEvent) error {
	// Make sure the current secret exists
	currentValue, err := smClient.GetCurrentSecretValue(event.SecretId)
	if err != nil {
		err = fmt.Errorf("while getting current value of secret %s: %v", event.SecretId, err)
		log.Println(err)
		return err
	}
	// Now try to get the secret version, if that fails, put a new secret
	var pendingValue string
	_, err = smClient.GetSecretValue(event.SecretId, "AWSPENDING")
	if err != nil {
		log.Printf("Creating a pending value for secret %s", event.SecretId)
		switch secretBeingRotated {
		case jetsDnsSecret:
			var m map[string]any
			err = json.Unmarshal([]byte(currentValue), &m)
			if err != nil {
				return fmt.Errorf("while unmarshalling current value of secret %s: %v", event.SecretId, err)
			}
			// Get a new password
			m["password"], err = smClient.GetRandomPassword(" %+~`#$&*()|[]{}:;<>?!'/@\"\\", 30)
			if err != nil {
				return fmt.Errorf("while generating new password for secret %s: %v", event.SecretId, err)
			}
			b, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("while marshalling value of secret %s: %v", event.SecretId, err)
			}
			pendingValue = string(b)

		case awsApiSecret:
			pendingValue, err = smClient.GetRandomPassword(" ", 15)
			if err != nil {
				return fmt.Errorf("while GetRandomPassword for secret %s: %v", event.SecretId, err)
			}

		case awsJetsAdminPwdSecret:
			pendingValue, err = smClient.GetRandomPassword(" ", 15)
			if err != nil {
				return fmt.Errorf("while GetRandomPassword for secret %s: %v", event.SecretId, err)
			}

		case jetsEncryptionKeySecret:
			pendingValue, err = smClient.GetRandomPassword(" !\"#$%&'()*+,./:;<=>?@[\\]^_`{|}~", 32)
			if err != nil {
				return fmt.Errorf("while GetRandomPassword for secret %s: %v", event.SecretId, err)
			}

		default:
			return fmt.Errorf("error: unknown secret %s to rotate", event.SecretId)
		}

		// Put the secret in aws secret manager
		err = smClient.PutSecretValue(event.SecretId, pendingValue, "AWSPENDING", event.ClientRequestToken)
		if err != nil {
			return fmt.Errorf("while calling PutSecretValue for secret %s: %v", event.SecretId, err)
		}
	}
	return nil
}

func SetSecret(smClient *awsi.SecretManagerClient, event *RotateSecretEvent) error {
	// update postgres password if rotating db pwd,
	// update the jetstore admin password in db if rotating jets admin pwd
	// Decrypt and re-encrypt the git tokens in database if rotating encryption key

	// Get the pending value of the secret
	pendingValue, err := smClient.GetSecretValue(event.SecretId, "AWSPENDING")
	if err != nil {
		return fmt.Errorf("while getting pending value of secret %s: %v", event.SecretId, err)
	}
	if strings.Contains(event.SecretId, jetsDnsSecret) {
		// Check if the password is already set in db to the pending version
		dbpool, err := OpenDbConn(pendingValue)
		if err == nil {
			log.Printf("set_secret: AWSPENDING secret is already set as password in PostgreSQL DB for secret %s", event.SecretId)
			dbpool.Close()
			return nil
		}
	}

	// Open db connection using the current value of the db credentials
	dbpool, err := OpenCurrentDbConn()
	if err != nil {
		return err
	}
	defer dbpool.Close()

	switch {
	case strings.Contains(event.SecretId, jetsDnsSecret):
		// Update postgres user password in database
		// get user name and password from the pending version of the secret
		var m map[string]any
		err = json.Unmarshal([]byte(pendingValue), &m)
		if err != nil {
			return fmt.Errorf("while unmarshalling pending value of secret %s: %v", event.SecretId, err)
		}

		// update the postgres user password in database
		_, err = dbpool.Exec(context.Background(),
			fmt.Sprintf("ALTER USER %v PASSWORD '%v';", m["username"], m["password"]))
		if err != nil {
			return fmt.Errorf("error: failed to update the user password in database: %v", err)
		}

	case strings.Contains(event.SecretId, awsJetsAdminPwdSecret):
		// Update jets admin password in database
		// hash the password
		hashedPassword, err := user.Hash(pendingValue)
		if err != nil {
			err = fmt.Errorf("while hashing admin password (secret %s): %v", event.SecretId, err)
			log.Println(err)
			return err
		}
		stmt := "UPDATE jetsapi.users SET password = $1 WHERE user_email = $2"
		_, err = dbpool.Exec(context.Background(), stmt, string(hashedPassword), jetsAdminEmail)
		if err != nil {
			err = fmt.Errorf("while updating admin password in db (secret %s): %v", event.SecretId, err)
			log.Println(err)
			return err
		}

	case strings.Contains(event.SecretId, jetsEncryptionKeySecret):
		currentValue, err := smClient.GetCurrentSecretValue(event.SecretId)
		if err != nil {
			return fmt.Errorf("while getting current value of secret %s: %v", event.SecretId, err)
		}
		return updateEncryptedToken(dbpool, pendingValue, currentValue)
	}
	return nil
}

func updateEncryptedToken(dbpool *pgxpool.Pool, encryptKey, decryptKey string) error {
	// Decrypt and re-encrypt the git tokens in database
	// Get the current encrypted values
	gitUserTokens, err := getGitTokens(dbpool)
	if err != nil {
		return err
	}
	if len(gitUserTokens) > 0 {
		err = updateGitTokens(dbpool, encryptKey, decryptKey, gitUserTokens)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func getGitTokens(dbpool *pgxpool.Pool) (gitUserTokens map[string]string, err error) {
	gitUserTokens = make(map[string]string)
	rows, err := dbpool.Query(context.Background(),
		"SELECT git_token, user_email FROM jetsapi.users")
	if err != nil {
		err = fmt.Errorf("while querying git_token from users table: %v", err)
		log.Println(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		// scan the row
		var token, user sql.NullString
		if err = rows.Scan(&token, &user); err != nil {
			err = fmt.Errorf("while scanning users table: %v", err)
			log.Println(err)
			return
		}
		if token.Valid && len(token.String) > 0 && user.Valid {
			gitUserTokens[user.String] = token.String
		}
	}
	return
}

func updateGitTokens(dbpool *pgxpool.Pool, encryptKey, decryptKey string, gitUserTokens map[string]string) error {
	stmt := "UPDATE jetsapi.users SET git_token = $1 WHERE user_email = $2"
	for userEmail, token := range gitUserTokens {
		tok, err := user.DecryptValue(token, decryptKey)
		if err != nil {
			log.Printf("while decrypting user git token: %v, skipping user %s", err, userEmail)
			continue
		}
		token, err = user.EncryptValue(tok, encryptKey)
		if err != nil {
			return fmt.Errorf("while encrypting user git token: %v", err)
		}
		// put it back in the database
		_, err = dbpool.Exec(context.Background(), stmt, token, userEmail)
		if err != nil {
			return fmt.Errorf("while updating git_token in db: %v", err)
		}
	}
	return nil
}

func TestSecret(smClient *awsi.SecretManagerClient, event *RotateSecretEvent) error {
	// test connectivity to db when rotating db secret, that's all we have to do here
	if !strings.Contains(event.SecretId, jetsDnsSecret) {
		return nil
	}

	// connect to db with the pending version
	pendingValue, err := smClient.GetSecretValue(event.SecretId, "AWSPENDING")
	if err != nil {
		return fmt.Errorf("while getting pending value of secret %s: %v", event.SecretId, err)
	}
	dbpool, err := OpenDbConn(pendingValue)
	if err != nil {
		err = fmt.Errorf("error: failed to connect to database using pending secret %v: %v", event.SecretId, err)
		log.Println(err)
		return err
	}
	defer dbpool.Close()

	// Test the database connection to ensure that permission are correct
	_, err = dbpool.Exec(context.Background(), "SELECT NOW();")
	if err != nil {
		return fmt.Errorf("error: failed to test the database connection: %v", err)
	}
	return nil
}

// Put a record in secret_rotation table to record the secret rotation
func RecordSecretRotation() error {
	dbpool, err := OpenCurrentDbConn()
	if err != nil {
		return err
	}
	defer dbpool.Close()

	// Check if no value exists in db
	_, err = dbpool.Exec(context.Background(),
		"INSERT INTO jetsapi.secret_rotation (secret, last_update) VALUES ($1, DEFAULT)", secretBeingRotated)
	if err != nil {
		// Already got an entry, update it
		_, err = dbpool.Exec(context.Background(),
			"UPDATE jetsapi.secret_rotation SET last_update = DEFAULT WHERE secret = $1", secretBeingRotated)
		if err != nil {
			return fmt.Errorf("while updating last_update in secret_rotation table: %v", err)
		}
	}
	return nil
}

// Open a DB connection using the current value of credentials
func OpenCurrentDbConn() (*pgxpool.Pool, error) {
	// Get a db connection
	dsn, err := awsi.GetDsnFromSecret(jetsDnsSecret, false, 1)
	if err != nil {
		err = fmt.Errorf("while getting dsn from secret: %v", err)
		log.Println(err)
		return nil, err
	}
	dbpool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		err = fmt.Errorf("error: failed to connect to database using current dsn: %v", err)
		log.Println(err)
		return nil, err
	}
	return dbpool, nil
}

// Open a DB connection using the provided dsn json
func OpenDbConn(dnsJson string) (*pgxpool.Pool, error) {
	dns, err := awsi.GetDsnFromJson(dnsJson, false, 1)
	if err != nil {
		return nil, fmt.Errorf("while getting dns from json: %v", err)
	}
	return pgxpool.Connect(context.Background(), dns)
}

func main() {
	lambda.Start(handler)
}
