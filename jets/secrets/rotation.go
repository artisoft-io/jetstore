package secrets

import (
	"log"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Component to perform rotation of JetStore secrets

var apiSecret string = os.Getenv("JETS_API_SECRET")
var adminSecret string = os.Getenv("JETS_ADMIN_SECRET")
var encryptionKeySecret string = os.Getenv("JETS_ENCRYPTION_KEY_SECRET")
var dsnSecret string = os.Getenv("JETS_DSN_SECRET")

func PerformSecretsRotation(dbpool *pgxpool.Pool) (err error) {
	log.Println("Rotate JetStore Secrets Initiated")
	defer func(){
		if err != nil {
			log.Printf("Rotate JetStore Secrets Completed with Error(s): %v\n", err)
		} else {
			log.Println("Rotate JetStore Secrets Completed")
		}
	}()
	// Database password rotation
	
	return rotateDatabasePassword()
}

func rotateDatabasePassword() error {
	
	return nil
}