package main

import "testing"

func TestCensorchirp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no censor", "I had something interesting for breakfast", "I had something interesting for breakfast"},
		{"censored", "I hear Mastodon is better than Chirpy. sharbert I need to migrate", "I hear Mastodon is better than Chirpy. **** I need to migrate"},
		{"censored", "I really need a kerfuffle to go to bed sooner, Fornax !", "I really need a **** to go to bed sooner, **** !"},
	}

	profane := []string{"kerfuffle", "sharbert", "fornax"}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := censorChirp(test.input, profane)
			if test.expected != expected {
				t.Errorf("got %s, expected %s", expected, test.expected)
			}
		})
	}
}
