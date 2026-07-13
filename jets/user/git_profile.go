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
	"math"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	if JetsEncriptionKey != "" {
		if err := verifyKeyStrength(JetsEncriptionKey); err != nil {
			log.Println("WARNING: JetsEncriptionKey did not pass strength check:", err)
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

// minKeyEntropyBits is the minimum acceptable Shannon entropy (in bits per byte)
// for the AES encryption key. A 32-byte key generated from a cryptographically
// secure source will comfortably exceed this threshold, while weak or predictable
// keys (e.g. repeated characters or dictionary phrases) fall below it. This guards
// against Insufficient Entropy (CWE-331) in the configured key material.
const minKeyEntropyBits = 3.0

// shannonEntropy returns the Shannon entropy of data expressed in bits per byte.
func shannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]float64
	for _, b := range data {
		counts[b]++
	}
	length := float64(len(data))
	var entropy float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := c / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// verifyKeyStrength validates that the encryption key has the required length and
// sufficient entropy to be used safely as an AES-256 key. It returns an error when
// the key is not 32 bytes long or when its entropy is below minKeyEntropyBits.
func verifyKeyStrength(keyString string) error {
	if len(keyString) != 32 {
		return fmt.Errorf("error: encryption key must be 32 bytes long, got %d", len(keyString))
	}
	if entropy := shannonEntropy([]byte(keyString)); entropy < minKeyEntropyBits {
		return fmt.Errorf(
			"error: encryption key has insufficient entropy (%.2f bits/byte, minimum %.2f); "+
				"use a key generated from a cryptographically secure source", entropy, minKeyEntropyBits)
	}
	return nil
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

	if stringToEncrypt == "" {
		err = fmt.Errorf("ERROR: encrypt called with empty stringToEncrypt")
		log.Println(err)
		return "", err
	}
	if err = verifyKeyStrength(keyString); err != nil {
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

	if encryptedString == "" {
		err = fmt.Errorf("ERROR: decrypt called with empty encryptedString")
		log.Println(err)
		return "", err
	}
	// Mitigating vulnerability: Insufficient Entropy (CWE ID 331) by verifying key strength before decryption. 
	// This ensures that the key used for decryption has sufficient randomness and is not easily guessable, 
	// which is crucial for maintaining the security of the encrypted data.
	if err = verifyKeyStrength(keyString); err != nil {
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
