package user

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/badoux/checkmail"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var AdminEmail string

type User struct {
	Name           string    `json:"name"`
	Email          string    `json:"user_email"`
	Password       string    `json:"password"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Token          string    `json:"token"`
	DevMode        string    `json:"dev_mode"`
	roles          map[string]bool
	capabilities   map[string]bool
	IsActive       int        `json:"is_active"`
	UserGitProfile GitProfile `json:"gitProfile"`
}

// JetStore Capabilities:
//  - jetstore_read: Read access from ui
// 	- client_config: Add, modify client configuration
//	- workspace_ide: Access workspace IDE screens and functions, including query tool and git functions
//	- run_pipelines: Load files and run pipelines
//  - user_profile:  Update user profile
// NOTE: role_capability table is initialized in jets_init_db.sql

func NewUser(email string) *User {
	u := User{Email: email}
	u.roles = make(map[string]bool)
	u.capabilities = make(map[string]bool)
	return &u
}

func Hash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func (u *User) IsAdmin() bool {
	return u.Email == AdminEmail
}

func (u *User) GetCapabilities() []string {
	keys := make([]string, 0, len(u.capabilities))
	for k := range u.capabilities {
		keys = append(keys, k)
	}
	return keys
}

func (u *User) HasCapability(capability string) bool {
	if capability == "" {
		return false
	}
	if u.IsAdmin() {
		return true
	}
	return u.capabilities[capability]
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
			return errors.New("required Password")
		}
		if u.Email == "" {
			return errors.New("required Email")
		}
		if !u.IsAdmin() {
			if err := checkmail.ValidateFormat(u.Email); err != nil {
				return errors.New("invalid Email")
			}
		}
		return nil

	default:
		if u.IsAdmin() {
			return errors.New("login Reserved for Administrator")
		}
		if u.Name == "" {
			return errors.New("required Name")
		}
		if u.Password == "" {
			return errors.New("required Password")
		}
		//* check that password pass test
		hasDigit, _ := regexp.MatchString("[0-9]", u.Password)
		hasUpper, _ := regexp.MatchString("[A-Z]", u.Password)
		hasLower, _ := regexp.MatchString("[a-z]", u.Password)
		hasSpecial, _ := regexp.MatchString(`[!@#$%^&*()_+-=\[\]{}|']`, u.Password)
		if !hasDigit || !hasUpper || !hasLower || !hasSpecial || len(u.Password) < 14 {
			return errors.New("invalid Password")
		}
		if u.Email == "" {
			return errors.New("required Email")
		}
		if err := checkmail.ValidateFormat(u.Email); err != nil {
			return errors.New("invalid Email")
		}
		return nil
	}
}

func (u *User) InsertUser(dbpool *pgxpool.Pool) error {
	// hash the password
	err := u.BeforeSave()
	if err != nil {
		log.Println("while hashing user's password before save in db:", err)
		return errors.New("unknown error while saving user")
	}
	// insert in db
	stmt := `INSERT INTO jetsapi.users (name, user_email, password) VALUES ($1, $2, $3)`
	_, err = dbpool.Exec(context.Background(), stmt, u.Name, u.Email, u.Password)
	if err != nil {
		log.Println("while inserting in db:", err)
		return errors.New("unknown error while saving user")
	}
	return nil
}

func GetUserByEmail(dbpool *pgxpool.Pool, email string) (*User, error) {
	// select from db
	u := NewUser(email)
	encryptedRoles := make([]string, 0)
	stmt := `SELECT name, password, encrypted_roles, is_active, git_name, git_email, git_handle 
					 FROM jetsapi.users 
					 WHERE user_email = $1`
	err := dbpool.QueryRow(context.Background(), stmt, u.Email).
		Scan(&u.Name, &u.Password, &encryptedRoles, &u.IsActive,
			&u.UserGitProfile.Name,
			&u.UserGitProfile.Email,
			&u.UserGitProfile.GitHandle)
	if err != nil {
		log.Println("while select user by user_email from db:", err)
		return nil, errors.New("invalid user or password")
	}
	if u.IsAdmin() {
		u.capabilities["client_config"] = true
		u.capabilities["workspace_ide"] = true
		u.capabilities["run_pipelines"] = true
		return u, nil
	}
	// Decrypt user's role and map it to capabilities
	// @**@ profile read Decrypt user's role and map it to capabilities

	for _, role := range encryptedRoles {
		u.roles[role] = true
	}
	if len(u.roles) > 0 {
		for i := range encryptedRoles {
			encryptedRoles[i] = fmt.Sprintf("'%s'", encryptedRoles[i])
		}
		stmt = fmt.Sprintf("SELECT capability	FROM jetsapi.role_capability WHERE role IN (%s)",
			strings.Join(encryptedRoles, ","))
		rows, err := dbpool.Query(context.Background(), stmt)
		if err != nil {
			log.Println("while select user by user_email from db:", err)
			return nil, errors.New("error retreiving roles")
		}
		defer rows.Close()
		var capability string
		for rows.Next() {
			if err := rows.Scan(&capability); err != nil {
				log.Println("while scanning capability from db:", err)
				return nil, errors.New("error retreiving capabilities")
			}
			u.capabilities[capability] = true
		}
	}

	return u, nil
}

func GetUserByToken(dbpool *pgxpool.Pool, token string) (*User, error) {
	// Get user info
	userEmail, err := ExtractTokenID(token)
	if err != nil {
		log.Println("while ExtractTokenID", err.Error())
		err = errors.New("error: unauthorized, cannot extract user email from token")
		return nil, err
	}
	user, err := GetUserByEmail(dbpool, userEmail)
	if err != nil {
		log.Println("while GetUserByEmail", err.Error())
		err = errors.New("error: unauthorized, cannot get user info")
		return nil, err
	}
	return user, nil
}
