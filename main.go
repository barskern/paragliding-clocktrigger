package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/subosito/gotenv"
	"net/http"
	"os"
	"os/signal"
	"time"
)

type slackMsg struct {
	Text string `json:"text"`
}

var (
	paraglidingURL string
	slackURL       string
	interval       time.Duration
)

func main() {
	var err error
	var ok bool
	// Load env variables from .env
	gotenv.Load()

	// Get paragliding url from env
	paraglidingURL, ok = os.LookupEnv("PARAGLIDING_URL")
	if !ok {
		log.Fatal("unable to get environment variable PARAGLIDING_URL")
	}

	// Get slack url from env
	slackURL, ok = os.LookupEnv("SLACK_WEBHOOK_URL")
	if !ok {
		log.Fatal("unable to get environment variable SLACK_WEBHOOK_URL")
	}
	// Get optional interval from env
	intervalStr, ok := os.LookupEnv("CLOCK_INTERVAL")
	if ok {
		interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			log.WithField("error", err).Fatal("invalid interval passed to CLOCK_INTERVAL")
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

	log.WithField("url", paraglidingURL).Info("getting initial count of ids")
	ids, err := getIDsFrom(paraglidingURL)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"url":   paraglidingURL,
		}).Fatal("unable to get initial ids")
	}

	count := len(ids)
	log.WithField("count", count).Info("initial count")
	for {
		select {
		case <-sigint:
			log.Info("recieved interrupt, shutting down")
			return
		case <-ticker.C:
			log.Info("running ticker iteration")
			count, err = updateCount(count)

			if err != nil {
				log.WithField("error", err).Error("iteration failed")
				continue
			}
		}
	}
}

func updateCount(count int) (newCount int, err error) {
	ids, err := getIDsFrom(paraglidingURL)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"url":   paraglidingURL,
		}).Error("unable to get ids")
		return
	}
	newCount = len(ids)
	log.WithField("count", newCount).Info("new count")

	if newCount > count {
		var plural string
		if len(ids)-count > 1 {
			plural = "s"
		} else {
			plural = ""
		}
		msg := slackMsg{
			fmt.Sprintf(
				"The following new track%s have been added to '%s': %v",
				plural,
				paraglidingURL,
				ids[count:],
			),
		}

		b := new(bytes.Buffer)
		json.NewEncoder(b).Encode(msg)
		log.WithField("msg", msg).Info("sending updated information to hook")

		_, err := http.Post(slackURL, "application/json", b)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"url":   slackURL,
			}).Error("unable to post request to url")
		}
	}
	return
}

func getIDsFrom(url string) (ids []uint, err error) {
	res, err := http.Get(url)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"url":   url,
		}).Error("unable to request ids from url")
	}

	err = json.NewDecoder(res.Body).Decode(&ids)
	defer res.Body.Close()
	if err != nil {
		log.WithField("error", err).Error("unable to decode json response")
	}
	return
}
