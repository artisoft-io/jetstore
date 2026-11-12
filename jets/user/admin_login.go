package user

import "os"

// AdminLoginEnvVar names the built-in administrator's login. It need not be an
// email address.
const AdminLoginEnvVar = "JETS_ADMIN_EMAIL"

// DefaultAdminLogin is the built-in administrator's login when AdminLoginEnvVar
// is unset or empty.
const DefaultAdminLogin = "admin"

// AdminLoginFromEnv returns the built-in administrator's login: AdminLoginEnvVar,
// or DefaultAdminLogin when it is unset or empty.
//
// Every process that names the administrator reads it here. The apiserver creates
// the administrator under this login, and the secret-rotation Lambda writes the
// rotated administrator password to the user with this login, so the two must
// agree. They used to disagree: the apiserver's -adminEmail flag defaulted to
// "admin" and never read the variable, while the Lambda read the variable alone,
// so a deployment setting it to anything but "admin" rotated the password of a
// user the apiserver had never created.
func AdminLoginFromEnv() string {
	if v := os.Getenv(AdminLoginEnvVar); v != "" {
		return v
	}
	return DefaultAdminLogin
}
