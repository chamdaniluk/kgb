package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword meng-hash password dengan bcrypt (TECH-STACK §2).
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword membandingkan password dengan hash bcrypt.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
