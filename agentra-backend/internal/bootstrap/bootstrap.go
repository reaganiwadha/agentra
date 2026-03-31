package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

var (
	mu         sync.Mutex
	token      string
	forcedMode bool
)

func generateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ValidateAndClear(t string) bool {
	mu.Lock()
	defer mu.Unlock()
	if token != "" && t == token {
		token = ""
		forcedMode = false
		return true
	}
	return false
}

func Validate(t string) bool {
	mu.Lock()
	defer mu.Unlock()
	return token != "" && t == token
}

func ForceResetToken() string {
	mu.Lock()
	defer mu.Unlock()
	token = generateToken()
	forcedMode = true
	return token
}

func IsForcedMode() bool {
	mu.Lock()
	defer mu.Unlock()
	return forcedMode && token != ""
}

func Check(db *sqlx.DB, log *logrus.Logger) error {
	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM users WHERE role = 'admin'"); err != nil {
		return err
	}
	if count == 0 {
		mu.Lock()
		token = generateToken()
		t := token
		mu.Unlock()

		fmt.Println("=== Agentra First Setup ===")
		fmt.Printf("Admin setup token: %s\n", t)
		fmt.Println("Visit POST /setup to create first admin")
	}
	return nil
}
