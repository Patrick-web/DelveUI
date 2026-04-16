package config

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadEnvFile parses a dotenv file and returns a map. Missing file returns (nil, nil).
func LoadEnvFile(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return godotenv.Read(path)
}
