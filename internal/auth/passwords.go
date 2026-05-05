package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

func VerifyPassword(encodedHash string, password string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("unsupported password hash format")
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	for _, param := range strings.Split(parts[3], ",") {
		keyValue := strings.SplitN(param, "=", 2)
		if len(keyValue) != 2 {
			return false, fmt.Errorf("invalid password hash params")
		}
		value, err := strconv.Atoi(keyValue[1])
		if err != nil {
			return false, err
		}
		switch keyValue[0] {
		case "m":
			memory = uint32(value)
		case "t":
			iterations = uint32(value)
		case "p":
			parallelism = uint8(value)
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
