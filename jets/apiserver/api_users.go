package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/user"
	// "github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// errSignInRefused is the one answer to a sign-in that does not succeed: an
// unknown login, a wrong password, and an account the administrator has not
// activated (or has deactivated) all read the same, so the answer does not say
// whether the login exists. The reason is logged, not sent. Until 2026-11-11
// an inactive account was told to contact the administrator before its
// password was checked, which confirmed the login to anyone who typed it, and
// an unknown login got the database lookup's own, differently worded, error.
var errSignInRefused = errors.New("Invalid user or password, or the user has not been activated by an administrator")

// unknownUserHash is checked against the password a sign-in for an unknown
// login sends, so that answer costs the same bcrypt work as a wrong password
// and its timing does not tell the two apart.
var unknownUserHash = sync.OnceValue(func() []byte {
	hash, err := user.Hash("jetstore unknown login " + time.Now().String())
	if err != nil {
		log.Println("ERROR while hashing the unknown-login password:", err)
	}
	return hash
})

// Login ------------------------------------------------------------
func (server *Server) Login(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, FormatError(err.Error()))
		return
	}
	jetsUser := user.NewUser("")
	err = json.Unmarshal(body, &jetsUser)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, FormatError(err.Error()))
		return
	}
	server.AuditLogger.Info("user login", zap.String("user", jetsUser.Email), zap.String("time", time.Now().Format(time.RFC3339)))

	jetsUser.Prepare()
	err = jetsUser.Validate("login")
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, FormatError(err.Error()))
		return
	}
	// provided password
	password := jetsUser.Password
	// get user details including pwd for verification from db
	email := jetsUser.Email
	jetsUser, err = user.GetUserByEmail(server.dbpool, email)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "password authentication failed"):
			// re-open db connection
			err = server.SecretsRotated()
			if err != nil {
				err = fmt.Errorf("while re-opening db connection after secret rotation: %v", err)
				ERROR(w, http.StatusUnprocessableEntity, FormatError(err.Error()))
				return
			}
			// try again
			jetsUser, err = user.GetUserByEmail(server.dbpool, email)
		}
	}
	if err != nil {
		log.Printf("sign-in refused for %q: %v", email, err)
		_ = bcrypt.CompareHashAndPassword(unknownUserHash(), []byte(password))
		ERROR(w, http.StatusUnprocessableEntity, errSignInRefused)
		return
	}
	// The password first, then the account: an inactive account's answer must
	// not be reachable without its password, and any error from the check
	// refuses, not only a mismatch (a stored value that is not a bcrypt hash
	// returned another error and signed the user in).
	err = user.VerifyPassword(jetsUser.Password, password)
	jetsUser.Password = ""
	if err != nil {
		log.Printf("sign-in refused for %q: password: %v", email, err)
		ERROR(w, http.StatusUnprocessableEntity, errSignInRefused)
		return
	}
	if jetsUser.IsActive != 1 {
		log.Printf("sign-in refused for %q: the account is not active", email)
		ERROR(w, http.StatusUnprocessableEntity, errSignInRefused)
		return
	}
	jetsUser.Token, err = user.CreateToken(jetsUser.Email)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, FormatError(err.Error()))
		return
	}
	if globalDevMode {
		jetsUser.DevMode = "true"
	}
	data := map[string]interface{}{
		"name":             jetsUser.Name,
		"user_email":       jetsUser.Email,
		"is_admin":         jetsUser.IsAdmin(),
		"is_active":        jetsUser.IsActive,
		"dev_mode":         jetsUser.DevMode,
		"capabilities":     jetsUser.GetCapabilities(),
		"token":            jetsUser.Token,
		"gitProfile":       jetsUser.UserGitProfile,
		"jetstore_version": os.Getenv("JETS_VERSION"),
		// The image's `<branch>-<sha>`, stamped by the build beside JETS_VERSION
		// (dockerfiles/Dockerfile.ui_service). The UI footer shows it (D05).
		"jetstore_git_sha": os.Getenv("JETS_GIT_SHA"),
		// The deployment's own Pipeline Status buttons, from
		// JETS_CUSTOM_BUTTONS_CONFIG_JSON, parsed once and never failing the
		// sign-in (custom_buttons.go). Always a list, empty when unconfigured (D04).
		"custom_buttons": customButtons(),
	}
	JSON(w, http.StatusOK, data)
}

func IsDuplicateUserError(err string) bool {
	return strings.Contains(err, "duplicate key")
}

func FormatError(err string) error {
	log.Println("ERROR:", err)
	// if strings.Contains(err, "name") {
	// 	return errors.New("Name Already Taken")
	// }

	// if strings.Contains(err, "email") {
	// 	return errors.New("Email Already Taken")
	// }
	// if strings.Contains(err, "hashedPassword") {
	// 	return errors.New("Incorrect Password")
	// }
	//* Leave the error as is for now
	return errors.New(err)
}

// User Management Functions

// CreateUser ------------------------------------------------------
func (server *Server) CreateUser(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
	}
	jetsUser := user.NewUser("")
	err = json.Unmarshal(body, &jetsUser)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	jetsUser.Prepare()
	err = jetsUser.Validate("")
	if err != nil {
		ERROR(w, http.StatusNotAcceptable, err)
		return
	}
	// Perform the insert
	err = jetsUser.InsertUser(server.dbpool)
	if err != nil {
		errstr := err.Error()
		if IsDuplicateUserError(errstr) {
			ERROR(w, http.StatusConflict, errors.New("User Already Exists"))
			return
		}
		formattedError := FormatError(errstr)
		ERROR(w, http.StatusInternalServerError, formattedError)
		return
	}
	jetsUser.Password = ""
	jetsUser.Token, err = user.CreateToken(jetsUser.Email)
	if err != nil {
		formattedError := FormatError(err.Error())
		ERROR(w, http.StatusUnprocessableEntity, formattedError)
		return
	}
	JSON(w, http.StatusOK, jetsUser)
}

// func (server *Server) GetUsers(w http.ResponseWriter, r *http.Request) {
// 	//* TODO FindAllUsers
// 	// jetsUser := User{}
// 	// users, err := jetsUser.FindAllUsers(server.DB)
// 	// if err != nil {
// 	// 	ERROR(w, http.StatusInternalServerError, err)
// 	// 	return
// 	// }
//* FIXME
// 	users := []user.User{}
// 	JSON(w, http.StatusOK, users)
// }

// // GetUser ------------------------------------------------------
// func (server *Server) GetUser(w http.ResponseWriter, r *http.Request) {

// 	vars := mux.Vars(r)
//* FIXME
// 	jetsUser := user.User{Email: vars["id"]}
// 	err := jetsUser.GetUserByEmail(server.dbpool)
// 	if err != nil {
// 		log.Println("error while get user by ID:",err)
// 		ERROR(w, http.StatusUnprocessableEntity, errors.New("User ID not found"))
// 		return
// 	}
// 	jetsUser.Password = ""
// 	JSON(w, http.StatusOK, jetsUser)
// }

// // GetUserDetails ------------------------------------------------------
// func (server *Server) GetUserDetails(w http.ResponseWriter, r *http.Request) {
// 	tokenID, err := user.ExtractTokenID(user.ExtractToken(r))
// 	if err != nil {
// 		log.Println("error while extracting user email from jwt token:",err)
// 		ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
// 		return
// 	}
//* FIXME
// 	jetsUser := user.User{Email: tokenID}
// 	err = jetsUser.GetUserByEmail(server.dbpool)
// 	if err != nil {
// 		log.Println("error while get user by email:",err)
// 		ERROR(w, http.StatusUnprocessableEntity, errors.New("User ID not found"))
// 		return
// 	}
// 	jetsUser.Password = ""
// 	JSON(w, http.StatusOK, jetsUser)
// }

// // UpdateUser ------------------------------------------------------
// func (server *Server) UpdateUser(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	body, err := io.ReadAll(r.Body)
// 	if err != nil {
// 		ERROR(w, http.StatusUnprocessableEntity, err)
// 		return
// 	}
//* FIXME
// 	jetsUser := user.User{}
// 	err = json.Unmarshal(body, &jetsUser)
// 	if err != nil {
// 		ERROR(w, http.StatusUnprocessableEntity, err)
// 		return
// 	}
// 	server.AuditLogger.Info("update user", zap.String("user", jetsUser.Email),zap.String("time", time.Now().Format(time.RFC3339)))
// 	tokenID, err := user.ExtractTokenID(user.ExtractToken(r))
// 	if err != nil {
// 		ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
// 		return
// 	}
// 	if tokenID != vars["id"] {
// 		ERROR(w, http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized)))
// 		return
// 	}
// 	jetsUser.Prepare()
// 	err = jetsUser.Validate("update")
// 	if err != nil {
// 		ERROR(w, http.StatusUnprocessableEntity, err)
// 		return
// 	}
// 	//* TODO UpdateUser
// 	// updatedUser, err := jetsUser.UpdateAUser(server.DB, uint32(uid))
// 	// if err != nil {
// 	// 	formattedError := FormatError(err.Error())
// 	// 	ERROR(w, http.StatusInternalServerError, formattedError)
// 	// 	return
// 	// }
// 	updatedUser := jetsUser
// 	updatedUser.Password = ""
// 	JSON(w, http.StatusOK, updatedUser)
// }

// // DeleteUser ------------------------------------------------------
// func (server *Server) DeleteUser(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	tokenID, err := user.ExtractTokenID(user.ExtractToken(r))
// 	if err != nil {
// 		ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
// 		return
// 	}
// 	if tokenID != "" && tokenID != vars["id"] {
// 		ERROR(w, http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized)))
// 		return
// 	}
// 	//* TODO
// 	// _, err = user.DeleteAUser(server.DB, uint32(uid))
// 	// if err != nil {
// 	// 	ERROR(w, http.StatusInternalServerError, err)
// 	// 	return
// 	// }
// 	w.Header().Set("Entity", vars["id"])
// 	JSON(w, http.StatusNoContent, "")
// }
