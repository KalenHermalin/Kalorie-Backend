package env

import (
	"errors"
	"fmt"
	"os"
)

func GetString(key string) (string, error) {
	val, ok := os.LookupEnv(key)
	if !ok {
		fmt.Printf("Could not find key: %s\n", key)
		return "", errors.New("Could not find key: " + key)
	}
	return val, nil
}
