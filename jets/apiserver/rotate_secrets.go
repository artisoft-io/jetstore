package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/secrets"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Rotate secrets and sync the current ui container

func RotateSecrets(dbpool *pgxpool.Pool) error {
	err := secrets.PerformSecretsRotation(dbpool)
	if err != nil {
		err = fmt.Errorf("rotate secrets: while rotating secrets(1): %v", err)
		log.Println(err)
		return err
	}

	// Refresh the api secret to sign the jwt token
	if *awsApiSecret != "" {
		*apiSecret, err = awsi.GetSecretValue(*awsApiSecret)
		if err != nil {
			err = fmt.Errorf("rotate secrets: while rotating secrets(2): %v", err)
			log.Println(err)
			return err
		}
		user.ApiSecret = *apiSecret
	} else {
		return fmt.Errorf("error: secrets rotated but AWS_API_SECRET not available to reload")
	}

	// Refresh the db connection
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		dsn, err := awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			err = fmt.Errorf("rotate secrets: while getting dsn from aws secret: %v", err)
			log.Println(err)
			return err
		}
		// re-open db connection
		server.dbpool.Close()
		server.dbpool, err = pgxpool.Connect(context.Background(), dsn)
		if err != nil {
			err = fmt.Errorf("rotate secrets: while opening db connection: %v", err)
			log.Println(err)
			return err
		}
	} else {
		return fmt.Errorf("error: secrets rotated but JETS_DSN_SECRET not available to reload")
	}

	// Update encrypt key, used to encrypt git token and user passwords
	ecSecret := os.Getenv("JETS_ENCRYPTION_KEY_SECRET")
	if ecSecret != "" {
		user.JetsEncriptionKey, err = awsi.GetSecretValue(ecSecret)
		if err != nil {
			err = fmt.Errorf("rotate secrets: while getting JETS_ENCRYPTION_KEY_SECRET from aws secret: %v", err)
			log.Println(err)
			return err
		}		
	} else {
		err = fmt.Errorf("error: secrets rotated but JETS_ENCRYPTION_KEY_SECRET not available to reload")
		log.Println(err)
		return err
	}

	// Update admin password in db
	if *awsAdminPwdSecret != "" {
		adminPassword, err := awsi.GetSecretValue(*awsAdminPwdSecret)
		if err != nil {
			err = fmt.Errorf("rotate secrets: while getting apiSecret from aws secret manager: %v", err)
			log.Println(err)
			return err
		}
		//****** hash the password
		hashedPassword, err := user.Hash(adminPassword)
		if err != nil {
			err = fmt.Errorf("rotate secrets: while hashing admin password: %v", err)
			log.Println(err)
			return err
		}
		stmt := "UPDATE jetsapi.users SET password = $1 WHERE user_email = $2"
		_, err = server.dbpool.Exec(context.Background(), stmt, string(hashedPassword), *adminEmail)
		if err != nil {
			err = fmt.Errorf("rotate secrets: while updating admin password in db: %v", err)
			log.Println(err)
			return err
		}
	}
	return nil
}