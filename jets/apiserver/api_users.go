package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/user"
	// "github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

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
	server.AuditLogger.Info("user login", zap.String("user", jetsUser.Email),zap.String("time", time.Now().Format(time.RFC3339)))

	jetsUser.Prepare()
	err = jetsUser.Validate("login")
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, FormatError(err.Error()))
		return
	}
	// provided password
	password := jetsUser.Password
	// get user details including pwd for verification from db
	jetsUser, err = user.GetUserByEmail(server.dbpool, jetsUser.Email)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, FormatError(err.Error()))
		return
	}
	if jetsUser.IsActive != 1 {
		ERROR(w, http.StatusUnprocessableEntity, errors.New("User is not active, please contact your Administrator"))
		return
	}
	err = user.VerifyPassword(jetsUser.Password, password)
	jetsUser.Password = ""
	if err != nil && err == bcrypt.ErrMismatchedHashAndPassword {
		log.Println("ERROR",err)
		ERROR(w, http.StatusUnprocessableEntity, errors.New("Invalid User or Password"))
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
		"name": jetsUser.Name,
		"user_email": jetsUser.Email,
		"is_admin": jetsUser.IsAdmin(),
		"is_active": jetsUser.IsActive,
		"dev_mode": jetsUser.DevMode,
		"capabilities": jetsUser.GetCapabilities(),
		"token": jetsUser.Token,
		"gitProfile": jetsUser.UserGitProfile,
	}
	JSON(w, http.StatusOK, data)
}

func IsDuplicateUserError(err string) bool {
	return strings.Contains(err, "duplicate key")
}

func FormatError(err string) error {
	log.Println("ERROR:",err)
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
