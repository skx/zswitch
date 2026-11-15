package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/skx/ziglib"
)

func main() {
	// Define command-line flags.
	dump := flag.Bool("dump", false, "Dump all known devices")
	debug := flag.Bool("debug", false, "Show all log output")
	mq_host := flag.String("mq-host", "cube.my.flat", "The MQ host address")
	mq_port := flag.Int("mq-port", 1883, "The MQ host port.")
	flag.Parse()

	// Default debugging level, increased via "-debug"
	level := slog.LevelWarn
	if *debug {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Where we connect
	mqUri := fmt.Sprintf("tcp://%s:%d/", *mq_host, *mq_port)

	client, err := ziglib.Connect(ziglib.Config{
		Broker:   mqUri,
		ClientID: "zigbee-switch",
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
				log.Fatalf("failed to change state", "device", name)
			}
		}
	}
}
