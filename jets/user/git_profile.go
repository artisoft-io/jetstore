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

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)
type GitProfile struct {
	Name      string `json:"git_name"`
	Email     string `json:"git_email"`
	GitHandle string `json:"git_handle"`
	GitToken  string `json:"git_token"`
}

var jetsEncriptionKey string
func init() {
	jetsEncriptionKey = os.Getenv("JETS_ENCRYPTION_KEY")
	if jetsEncriptionKey == "" {
		log.Println("Could not load value for JETS_ENCRYPTION_KEY")
	}
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
	gitProfile.GitToken = DecryptGitToken(encryptedGitToken)
	return gitProfile, nil
}

// func EncryptWithEmail(dataToEncrypt, email string) string {
// 	encryptedEmail := encrypt(email, jetsEncriptionKey)
// 	if len(encryptedEmail) < 32 {
// 		return ""
// 	}
// 	return encrypt(dataToEncrypt, encryptedEmail[:32])
// }

func EncryptGitToken(gitToken string) string {
	return encrypt(gitToken, jetsEncriptionKey)
}

// func DecryptWithEmail(encryptedData, email string) string {
// 	encryptedEmail := encrypt(email, jetsEncriptionKey)
// 	if len(encryptedEmail) < 32 {
// 		return ""
// 	}
// 	return decrypt(encryptedData, encryptedEmail[:32])
// }

func DecryptGitToken(encryptedGitToken string) string {
	return decrypt(encryptedGitToken, jetsEncriptionKey)
}


// From: https://www.melvinvivas.com/how-to-encrypt-and-decrypt-data-using-aes
func encrypt(stringToEncrypt string, keyString string) (encryptedString string) {

	if stringToEncrypt == "" || len(keyString) != 32 {
		log.Println("ERROR: decrypt called with empty stringToEncrypt or len(key) != 32")
		return ""
	}

	//Since the key is in string, we need to convert decode it to bytes
	key := []byte(keyString)
	plaintext := []byte(stringToEncrypt)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	//Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	//https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	//Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	//Encrypt the data using aesGCM.Seal
	//Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return fmt.Sprintf("%x", ciphertext)
}

func decrypt(encryptedString string, keyString string) (decryptedString string) {

	if encryptedString == "" || len(keyString) != 32 {
		log.Println("ERROR: decrypt called with empty encryptedString or len(key) != 32")
		return ""
	}
	
	key := []byte(keyString)
	enc, _ := hex.DecodeString(encryptedString)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	//Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}

	return string(plaintext)
}
