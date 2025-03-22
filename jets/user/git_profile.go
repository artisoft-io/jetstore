package user

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type GitProfile struct {
	Name      string `json:"git_name"`
	Email     string `json:"git_email"`
	GitHandle string `json:"git_handle"`
	GitToken  string `json:"git_token"`
}

var JetsEncriptionKey string
var DevMode bool

func init() {
	var err error
	JetsEncriptionKey = os.Getenv("JETS_ENCRYPTION_KEY")
	if JetsEncriptionKey == "" {
		JetsEncriptionKey, err = getEncryptionKeyFromSecret()
		if err != nil {
			log.Println("user.init():", err)
		}
	}
	AdminEmail = os.Getenv("JETS_ADMIN_EMAIL")
	_, DevMode = os.LookupEnv("JETSTORE_DEV_MODE")

	// Get secret to sign jwt tokens
	awsApiSecret := os.Getenv("AWS_API_SECRET")
	apiSecret := os.Getenv("API_SECRET")
	if apiSecret == "" && awsApiSecret != "" {
		apiSecret, err = awsi.GetCurrentSecretValue(awsApiSecret)
		if err != nil {
			log.Printf("user.init(): could not get secret value for AWS_API_SECRET: %v\n", err)
		}
	}
	ApiSecret = apiSecret
	TokenExpiration = 60
}

func getEncryptionKeyFromSecret() (key string, err error) {
	secret := os.Getenv("JETS_ENCRYPTION_KEY_SECRET")
	if secret != "" {
		key, err = awsi.GetCurrentSecretValue(secret)
		if err != nil {
			err = fmt.Errorf("while getting JETS_ENCRYPTION_KEY_SECRET from aws secret: %v", err)
		}
	} else {
		err = fmt.Errorf("error: could not load value for JETS_ENCRYPTION_KEY or JETS_ENCRYPTION_KEY_SECRET")
	}
	return
}

func GetGitProfile(dbpool *pgxpool.Pool, userEmail string) (GitProfile, error) {
	var gitProfile GitProfile
	stmt := fmt.Sprintf(
		"SELECT git_name, git_email, git_handle, git_token FROM jetsapi.users WHERE user_email = '%s'",
		userEmail)
	err := dbpool.QueryRow(context.Background(), stmt).Scan(
		&gitProfile.Name, &gitProfile.Email, &gitProfile.GitHandle, &gitProfile.GitToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Println("Error user not found")
			return gitProfile, errors.New("error: user not found")
		}
		log.Println("Error while reading user's git profile from users table:", err)
		return gitProfile, err
	}
	// Decrypt the git token
	encryptedGitToken := gitProfile.GitToken
	gitProfile.GitToken, err = DecryptGitToken(encryptedGitToken)
	return gitProfile, err
}

func EncryptGitToken(gitToken string) (string, error) {
	return encrypt(gitToken, JetsEncriptionKey)
}

func DecryptGitToken(encryptedGitToken string) (string, error) {
	token, err := decrypt(encryptedGitToken, JetsEncriptionKey)
	if err != nil && strings.Contains(err.Error(), "cipher: message authentication failed") {
		// Check if encryption key has changed
		currentValue := JetsEncriptionKey
		JetsEncriptionKey, err = getEncryptionKeyFromSecret()
		if err != nil {
			// unable to get the new key, secret may not be set
			log.Println("WARNING: unable to refresh encryption key from secret (in DecryptGitToken):", err)
			return "", err
		}
		if currentValue != JetsEncriptionKey {
			log.Println("Encryption key have been rotated, identified in DecryptGitToken")
			return decrypt(encryptedGitToken, JetsEncriptionKey)
		}
	}
	return token, err
}

func EncryptValue(value, encryptionKey string) (string, error) {
	return encrypt(value, encryptionKey)
}

func DecryptValue(value, encryptionKey string) (string, error) {
	return decrypt(value, encryptionKey)
}

// From: https://www.melvinvivas.com/how-to-encrypt-and-decrypt-data-using-aes
func encrypt(stringToEncrypt string, keyString string) (encryptedString string, err error) {

	if stringToEncrypt == "" || len(keyString) != 32 {
		err = fmt.Errorf("ERROR: decrypt called with empty stringToEncrypt or len(key) != 32")
		log.Println(err)
		return "", err
	}

	//Since the key is in string, we need to convert decode it to bytes
	key := []byte(keyString)
	plaintext := []byte(stringToEncrypt)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	//Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	//https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	//Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	//Encrypt the data using aesGCM.Seal
	//Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return fmt.Sprintf("%x", ciphertext), nil
}

func decrypt(encryptedString string, keyString string) (decryptedString string, err error) {

	if encryptedString == "" || len(keyString) != 32 {
		err = fmt.Errorf("ERROR: decrypt called with empty encryptedString or len(key) != 32")
		log.Println(err)
		return "", err
	}

	key := []byte(keyString)
	enc, _ := hex.DecodeString(encryptedString)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	//Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
