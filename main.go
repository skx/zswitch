package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/skx/ziglib"
)

// Connect to MQTT, retrying a bunch of times if we see failure.
func connectMQTT(config ziglib.Config) (*ziglib.Client, error) {
	delay := 2 * time.Second
	count := 1
	max := 10
	for count < max {
		log.Printf("Connecting to MQTT broker attempt %d/%d...", count, max)

		c, err := ziglib.Connect(config)
		if err == nil {
			return c, nil
		}

		log.Printf("MQTT connection failed: %v", err)
		log.Printf("Retrying in %s...", delay)

		time.Sleep(delay)

		count++
	}
	return nil, fmt.Errorf("failed to connect to MQ after %d attempts", count)
}

func main() {
	// MQ Host
	host := "cube.my.flat"
	if os.Getenv("MQ_HOST") != "" {
		host = os.Getenv("MQ_HOST")
	}

	// Define command-line flags.
	dump := flag.Bool("dump", false, "Dump all known devices")
	debug := flag.Bool("debug", false, "Show all log output")
	mq_host := flag.String("mq-host", host, "The MQ host address")
	mq_port := flag.Int("mq-port", 1883, "The MQ host port.")
	mq_user := flag.String("mq-user", "", "The username to authenticate to the queue with")
	mq_pass := flag.String("mq-pass", "", "The password to authenticate to the queue with")

	flag.Parse()

	// Default debugging level, increased via "-debug"
	level := slog.LevelWarn
	if *debug {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Where we connect
	mqUri := fmt.Sprintf("tcp://%s:%d/", *mq_host, *mq_port)

	// Note: no ClientID is set, so the library generates a unique one.
	// A fixed ID would cause concurrent invocations (e.g. from cron) to
	// disconnect each other at the broker, losing state changes.
	client, err := connectMQTT(ziglib.Config{
		Broker:   mqUri,
		Username: *mq_user,
		Password: *mq_pass,
	})
	if err != nil {
		log.Fatalf("MQTT connection failed: %v", err)
		return
	}
	defer client.Close()

	if *dump {

		// Get the devices
		devs := client.Devices()

		// Sort them.
		sort.Slice(devs, func(i, j int) bool {
			return devs[i].FriendlyName < devs[j].FriendlyName
		})

		for _, dev := range devs {
			if dev.FriendlyName == "Coordinator" {
				continue
			}
			fmt.Printf("%s %s %s %s\n", dev.IEEEAddress, dev.State, dev.FriendlyName, dev.Description)
		}
		return
	}

	// Ensure we have two arguments, before we use them.
	if flag.NArg() != 2 {
		fmt.Printf("Usage %s NAME STATE\n", os.Args[0])
		return
	}

	// OK we're making a change
	name := flag.Arg(0)
	state := flag.Arg(1)

	// Get the devices
	devs := client.Devices()

	// Sort them.
	sort.Slice(devs, func(i, j int) bool {
		return devs[i].FriendlyName < devs[j].FriendlyName
	})

	// For each device
	for _, dev := range devs {

		// If the name is the name
		// Or the name contains the name.
		// Or the description contains the name.
		if dev.FriendlyName == name || strings.Contains(dev.FriendlyName, name) || strings.Contains(dev.Description, name) {
			slog.Info("changing state of devices",
				"device", dev.FriendlyName,
				"address", dev.IEEEAddress,
				"state", state)
			err := client.SetState(dev.FriendlyName, state)
			if err != nil {
				log.Fatalf("failed to change state of %q: %v", dev.FriendlyName, err)
			}
		}
	}
}
