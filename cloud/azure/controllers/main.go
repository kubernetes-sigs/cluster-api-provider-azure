package controllers

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func PrepareEnvironment() error {
	//Parse in environment variables if necessary
	if os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
		err := godotenv.Load()
		if err == nil && os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
			return fmt.Errorf("couldn't find environment variable for the Azure subscription: %v", err)
		}
		if err != nil {
			return fmt.Errorf("failed to load environment variables: %v", err)
		}
	}
	return nil
}
