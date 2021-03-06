package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/badoux/checkmail"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Token     string    `json:"token"`
}

func Hash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func (u *User) BeforeSave() error {
	hashedPassword, err := Hash(u.Password)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

func (u *User) Prepare() {
	u.Name = html.EscapeString(strings.TrimSpace(u.Name))
	u.Email = html.EscapeString(strings.TrimSpace(u.Email))
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
}

func (u *User) Validate(action string) error {
	switch strings.ToLower(action) {
	case "login":
		if u.Password == "" {
			return errors.New("Required Password")
		}
		if u.Email == "" {
			return errors.New("Required Email")
		}
		if err := checkmail.ValidateFormat(u.Email); err != nil {
			return errors.New("Invalid Email")
		}
		return nil

	default:
		if u.Name == "" {
			return errors.New("Required Name")
		}
		if u.Password == "" {
			return errors.New("Required Password")
		}
		//* check that password pass test
		hasDigit, _ := regexp.MatchString("[0-9]", u.Password)
		hasUpper, _ := regexp.MatchString("[A-Z]", u.Password)
		hasLower, _ := regexp.MatchString("[a-z]", u.Password)
		if !hasDigit || !hasUpper || !hasLower {
			return errors.New("Invalid Password")
		}
		if u.Email == "" {
			return errors.New("Required Email")
		}
		if err := checkmail.ValidateFormat(u.Email); err != nil {
			return errors.New("Invalid Email")
		}
		return nil
	}
}

func (u *User) InsertUser(dbpool *pgxpool.Pool) error {
	// hash the password
	err := u.BeforeSave()
	if err != nil {
		fmt.Println("while hashing user's password before save in db:", err)
		return fmt.Errorf("while hashing user's password before save in db: %v", err)
	}
	// insert in db
	stmt := `INSERT INTO jetsapi.users (name, email, password) VALUES ($1, $2, $3)`
	_, err = dbpool.Exec(context.Background(), stmt, u.Name, u.Email, u.Password)
	if err != nil {
		fmt.Println("while inserting in db:", err)
		return fmt.Errorf("while inserting in db: %v", err)
	}
	return nil
}

func (u *User) GetUserByEmail(dbpool *pgxpool.Pool) error {
	// select from db
	stmt := `SELECT name, password FROM jetsapi.users WHERE email = $1`
	err := dbpool.QueryRow(context.Background(), stmt, u.Email).Scan(&u.Name, &u.Password)
	if err != nil {
		fmt.Println("while select user by email from db:", err)
		return fmt.Errorf("while select user by email from db: %v", err)
	}
	return nil
}
