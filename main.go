package main

import (
	"log"
	"time"

	ai "github.com/AbatyaLEX/LocalMotivatorWin/ai"
	ticker "github.com/AbatyaLEX/LocalMotivatorWin/ticker"
	notification "github.com/AbatyaLEX/LocalMotivatorWin/windows"
)

const (
	configPath = "config.json"
	notTitle   = "Local Motivator"
)

func main() {
	config, err := ai.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	aiClient, err := ai.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}

	if err := notification.Start(); err != nil {
		log.Fatal(err)
	}
	defer notification.Close()

	period := time.Duration(
		config.NotificationIntervalMinute,
	) * time.Minute

	action := func() {
		message, err := aiClient.CreateRequest()
		if err != nil {
			log.Printf("failed to generate motivation: %v", err)
			return
		}

		if err := notification.ShowNotification(
			notTitle,
			message,
		); err != nil {
			log.Printf("failed to show notification: %v", err)
			return
		}

		log.Printf("motivation delivered: %s", message)
	}

	ticker.Start(period, action)
}
