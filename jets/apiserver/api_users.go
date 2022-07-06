package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// Login ------------------------------------------------------------
func (server *Server) Login(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	user := User{}
	err = json.Unmarshal(body, &user)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}

	user.Prepare()
	err = user.Validate("login")
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	token, err := server.SignIn(user.Email, user.Password)
	if err != nil {
		formattedError := FormatError(err.Error())
		ERROR(w, http.StatusUnprocessableEntity, formattedError)
		return
	}
	JSON(w, http.StatusOK, token)
}

func (server *Server) SignIn(email, password string) (string, error) {

	user := User{Email: email}
	err := user.GetUserByEmail(server.dbpool)
	if err != nil {
		return "", err
	}
	err = VerifyPassword(user.Password, password)
	if err != nil && err == bcrypt.ErrMismatchedHashAndPassword {
		return "", err
	}
	return CreateToken(user.ID)
}

func IsDuplicateUserError(err string) bool {
	return strings.Contains(err, "users_email_key")
}

func FormatError(err string) error {

	if strings.Contains(err, "name") {
		return errors.New("Name Already Taken")
	}

	if strings.Contains(err, "email") {
		return errors.New("Email Already Taken")
	}
	if strings.Contains(err, "hashedPassword") {
		return errors.New("Incorrect Password")
	}
	return errors.New("Unknown Error")
}

// User Management Functions

// CreateUser ------------------------------------------------------
func (server *Server) CreateUser(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
	}
	user := User{}
	err = json.Unmarshal(body, &user)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	user.Prepare()
	err = user.Validate("")
	if err != nil {
		ERROR(w, http.StatusNotAcceptable, err)
		return
	}
	// Perform the insert
	userPwd := user.Password
	err = user.InsertUser(server.dbpool)
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
	token, err := server.SignIn(user.Email, userPwd)
	if err != nil {
		formattedError := FormatError(err.Error())
		ERROR(w, http.StatusUnprocessableEntity, formattedError)
		return
	}
	JSON(w, http.StatusOK, token)
}

func (server *Server) GetUsers(w http.ResponseWriter, r *http.Request) {

	//* TODO FindAllUsers
	// user := User{}
	// users, err := user.FindAllUsers(server.DB)
	// if err != nil {
	// 	ERROR(w, http.StatusInternalServerError, err)
	// 	return
	// }
	users := []User{}
	JSON(w, http.StatusOK, users)
}

// GetUser ------------------------------------------------------
func (server *Server) GetUser(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	uid, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		ERROR(w, http.StatusBadRequest, err)
		return
	}
	user := User{ID: uint32(uid)}
	err = user.GetUserByID(server.dbpool)
	if err != nil {
		log.Println("error while get user by ID:",err)
		ERROR(w, http.StatusUnprocessableEntity, errors.New("User ID not found"))
		return
	}
	user.Password = ""
	JSON(w, http.StatusOK, user)
}

// GetUserDetails ------------------------------------------------------
func (server *Server) GetUserDetails(w http.ResponseWriter, r *http.Request) {

	tokenID, err := ExtractTokenID(r)
	if err != nil {
		log.Println("error while extracting user ID from jwt token:",err)
		ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}
	user := User{ID: tokenID}
	err = user.GetUserByID(server.dbpool)
	if err != nil {
		log.Println("error while get user by ID:",err)
		ERROR(w, http.StatusUnprocessableEntity, errors.New("User ID not found"))
		return
	}
	user.Password = ""
	JSON(w, http.StatusOK, user)
}

// UpdateUser ------------------------------------------------------
func (server *Server) UpdateUser(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	uid, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		ERROR(w, http.StatusBadRequest, err)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	user := User{}
	err = json.Unmarshal(body, &user)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	tokenID, err := ExtractTokenID(r)
	if err != nil {
		ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}
	if tokenID != uint32(uid) {
		ERROR(w, http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized)))
		return
	}
	user.Prepare()
	err = user.Validate("update")
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	//* TODO UpdateUser
	// updatedUser, err := user.UpdateAUser(server.DB, uint32(uid))
	// if err != nil {
	// 	formattedError := FormatError(err.Error())
	// 	ERROR(w, http.StatusInternalServerError, formattedError)
	// 	return
	// }
	updatedUser := user
	updatedUser.Password = ""
	JSON(w, http.StatusOK, updatedUser)
}

// DeleteUser ------------------------------------------------------
func (server *Server) DeleteUser(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	// user := User{}

	uid, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		ERROR(w, http.StatusBadRequest, err)
		return
	}
	tokenID, err := ExtractTokenID(r)
	if err != nil {
		ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}
	if tokenID != 0 && tokenID != uint32(uid) {
		ERROR(w, http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized)))
		return
	}
	//* TODO
	// _, err = user.DeleteAUser(server.DB, uint32(uid))
	// if err != nil {
	// 	ERROR(w, http.StatusInternalServerError, err)
	// 	return
	// }
	w.Header().Set("Entity", fmt.Sprintf("%d", uid))
	JSON(w, http.StatusNoContent, "")
}
