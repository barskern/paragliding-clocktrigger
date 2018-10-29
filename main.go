package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"time"
)

type slackMsg struct {
	Text string `json:"text"`
}

func main() {
	// Get paragliding url from env
	paraglidingURL, ok := os.LookupEnv("PARAGLIDING_URL")
	if !ok {
		log.Fatal("unable to get environment variable PARAGLIDING_URL")
	}
	// Get slack url from env
	slackURL, ok := os.LookupEnv("SLACK_CLOCK_URL")
	if !ok {
		log.Fatal("unable to get environment variable SLACK_CLOCK_URL")
	}
	// Get optional interval from env
	intervalStr, ok := os.LookupEnv("CLOCK_INTERVAL")
	var interval time.Duration
	var err error
	if ok {
		interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			log.WithField("error", err).Fatalf("invalid interval passed to CLOCK_INTERVAL")
		}
	}
	if interval == 0 {
		log.Warn("falling back to default interval")
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	log.WithFields(log.Fields{
		"interval":        interval,
		"slack_url":       slackURL,
		"paragliding_url": paraglidingURL,
	}).Info("initializing clock trigger")

	prevCount := 0
	b := new(bytes.Buffer)
	for {
		select {
		case <-sigint:
			log.Info("recieved interrupt, shutting down")
			return
		case <-ticker.C:
			log.Info("running ticker iteration")

			res, err := http.Get(paraglidingURL + "/paragliding/api/track")
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"url":   paraglidingURL + "/paragliding/api/track",
				}).Error("unable to request ids from url")
				continue
			}

			var ids []uint
			err = json.NewDecoder(res.Body).Decode(&ids)
			res.Body.Close()
			if err != nil {
				log.WithField("error", err).Error("unable to decode json response")
				continue
			}
			if len(ids) > prevCount {
				// Decrement prevCount to preform a correct index because index
				// starts at 0
				if prevCount > 0 {
					prevCount--
				}
				msg := slackMsg{
					fmt.Sprintf(
						"The following new tracks have been added to '%s': %v",
						paraglidingURL,
						ids[prevCount:],
					),
				}

				// Update prevCount to current count
				prevCount = len(ids)

				json.NewEncoder(b).Encode(msg)
				log.WithField("msg", msg).Info("sending updated information to hook")

				res, err = http.Post(slackURL, "application/json", b)
				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
						"url":   slackURL,
					}).Error("unable to post request to url")
				}
			}

		}
	}
}
