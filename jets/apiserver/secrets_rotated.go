package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Sync the current ui container secrets with secret manager

func (server *Server) SecretsRotated() error {
	var err error
	log.Println("Secrets have rotated, refreshed cached values in ui container")

	// Refresh the api secret to sign the jwt token
	if *awsApiSecret != "" {
		*apiSecret, err = awsi.GetCurrentSecretValue(*awsApiSecret)
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
		user.JetsEncriptionKey, err = awsi.GetCurrentSecretValue(ecSecret)
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
	return nil
}
