package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

func (server *Server) Home(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, "Welcome To This Awesome API")
}


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

	var err error
	user := User{}

	// err = server.DB.Debug().Model(User{}).Where("email = ?", email).Take(&user).Error
	// if err != nil {
	// 	return "", err
	// }
	//* Get user password from storage
	// --
	err = VerifyPassword(user.Password, password)
	if err != nil && err == bcrypt.ErrMismatchedHashAndPassword {
		return "", err
	}
	return CreateToken(user.ID)
}

func FormatError(err string) error {

	if strings.Contains(err, "nickname") {
		return errors.New("Nickname Already Taken")
	}

	if strings.Contains(err, "email") {
		return errors.New("Email Already Taken")
	}

	if strings.Contains(err, "title") {
		return errors.New("Title Already Taken")
	}
	if strings.Contains(err, "hashedPassword") {
		return errors.New("Incorrect Password")
	}
	return errors.New("Incorrect Details")
}

// User Management Functions

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
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	userCreated, err := user.SaveUser(server)

	if err != nil {

		formattedError := FormatError(err.Error())

		ERROR(w, http.StatusInternalServerError, formattedError)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("%s%s/%d", r.Host, r.RequestURI, userCreated.ID))
	JSON(w, http.StatusCreated, userCreated)
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

func (server *Server) GetUser(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	uid, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		ERROR(w, http.StatusBadRequest, err)
		return
	}
	user := User{ID: uint32(uid)}
	//* TODO FindUserByID
	// userGotten, err := user.FindUserByID(server.DB, uint32(uid))
	// if err != nil {
	// 	ERROR(w, http.StatusBadRequest, err)
	// 	return
	// }
	userGotten := user
	JSON(w, http.StatusOK, userGotten)
}

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
	JSON(w, http.StatusOK, updatedUser)
}

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
