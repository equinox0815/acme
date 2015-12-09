package main

import(
	"fmt"
	"gopkg.in/hlandau/service.v2/passwd"
)

var usernamesToTry = []string{"daemon", "nobody"}

func determineAppropriateUsername() (string, error) {
	for _, u := range usernamesToTry {
		_, err := passwd.ParseUID(u)
		if err == nil {
			return u, nil
		}
	}

	return "", fmt.Errorf("cannot find appropriate username")
}
