package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJwt(t *testing.T) {
	type jwtInput struct {
		id          uuid.UUID
		tokenSecret string
		expiresIn   time.Duration
	}

	cases := []struct {
		index        int
		description  string
		input        jwtInput
		process      func(jwtInput, string) (bool, error)
		expectedPass bool
	}{
		{
			index:       0,
			description: "token should expired",
			input:       jwtInput{uuid.New(), "secret", 1 * time.Second},
			process: func(ji jwtInput, secretKey string) (bool, error) {
				newToken, err := MakeJWT(ji.id, ji.tokenSecret, ji.expiresIn)
				if err != nil {
					return false, err
				}
				time.Sleep(2 * ji.expiresIn)
				id, err := ValidateJWT(newToken, secretKey)
				if err != nil {
					return false, err
				}
				return id == ji.id, nil
			},
			expectedPass: false,
		},
		{
			index:       1,
			description: "token should pass",
			input:       jwtInput{uuid.New(), "secret", 1 * time.Second},
			process: func(ji jwtInput, s string) (bool, error) {
				newToken, err := MakeJWT(ji.id, ji.tokenSecret, ji.expiresIn)
				if err != nil {
					return false, err
				}
				id, err := ValidateJWT(newToken, s)
				if err != nil {
					return false, err
				}
				return id == ji.id, nil
			},
			expectedPass: true,
		},
		{
			index:       2,
			description: "mismatch secret",
			input:       jwtInput{uuid.New(), "nosecret", 1 * time.Second},
			process: func(ji jwtInput, s string) (bool, error) {
				newToken, err := MakeJWT(ji.id, ji.tokenSecret, ji.expiresIn)
				if err != nil {
					return false, err
				}
				id, err := ValidateJWT(newToken, s)
				if err != nil {
					return false, err
				}
				return id == ji.id, err
			},
			expectedPass: false,
		},
	}

	for _, c := range cases {
		fmt.Printf("\nTest case number: %v\nTest description: %s\n", c.index, c.description)
		isPassed, err := c.process(c.input, "secret")
		if isPassed != c.expectedPass {
			t.Errorf("test index: %v\nisPassed: %v\nexpectedPass: %v\nerr: %v\n", c.index, isPassed, c.expectedPass, err)
		}
	}
}
