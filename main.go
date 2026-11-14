package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	mqttBroker = "tcp://cube.my.flat:1883"
	clientID   = "go-home-automation"
	baseTopic  = "zigbee2mqtt"

	maxAttempts  = 10
	stateTimeout = 5 * time.Second
)

// Device contains details about a discovered device.
type Device struct {
	FriendlyName string `json:"friendly_name"`
	IEEEAddress  string `json:"ieee_address"`
	Description  string `json:"description"`
}

func main() {

	// Define command-line flags
	dump := flag.Bool("dump", false, "Dump all known devices")
	debug := flag.Bool("debug", false, "Show all log ouptut")
	flag.Parse()

	// If -debug is in-play change the log-level
	if *debug {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})))
	} else {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})))
	}

	// Change the default MQTT host, if required.
	if str := os.Getenv("MQ_HOST"); str != "" {
		mqttBroker = fmt.Sprintf("tcp://%s:1883", str)
	}

	var err error

	// Dumping devices?
	if *dump {
		err = dumpDevices()
		if err != nil {
			slog.Error("failed to dump devices", "error", err)
			os.Exit(1)
		}
		return
	}

	// Otherwise just do the on/off thing.
	if flag.NArg() != 2 {
		fmt.Printf("Usage: %s [flags] NAME STATE\n", os.Args[0])
		os.Exit(1)
	}

	err = run(flag.Arg(0), flag.Arg(1))

	if err != nil {
		slog.Error("Program failed", "error", err)
		os.Exit(1)
	}
}

// connect returns a handle to the MQTT queue.
func connect() (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(mqttBroker).
		SetClientID(fmt.Sprintf("%s-%d", clientID, os.Getpid())).
		SetConnectTimeout(10 * time.Second).
		SetAutoReconnect(true).
		SetOrderMatters(false)

	client := mqtt.NewClient(opts)

	slog.Info("Connecting to MQTT broker", "broker", mqttBroker)

	token := client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return nil, fmt.Errorf("MQTT connection timed out")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("MQTT connection failed: %w", err)
	}

	slog.Info("Connected to MQTT broker")

	return client, nil
}

// getDevices retrieves all known devices from Zigbee2MQTT.
func getDevices(client mqtt.Client) ([]Device, error) {
	devicesTopic := baseTopic + "/bridge/devices"
	devicesChan := make(chan []Device, 1)

	token := client.Subscribe(devicesTopic, 1, func(
		client mqtt.Client,
		msg mqtt.Message,
	) {
		var devices []Device

		if err := json.Unmarshal(msg.Payload(), &devices); err != nil {
			slog.Error("Failed to decode device list", "error", err)
			return
		}

		select {
		case devicesChan <- devices:
		default:
		}
	})

	if !token.WaitTimeout(10 * time.Second) {
		return nil, fmt.Errorf("device list subscription timed out")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("device list subscription failed: %w", err)
	}

	slog.Info("Waiting for Zigbee2MQTT device list", "topic", devicesTopic)

	select {
	case devices := <-devicesChan:
		slog.Info("Received device list", "device_count", len(devices))
		return devices, nil

	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf(
			"timed out waiting for Zigbee2MQTT device list",
		)
	}
}

// dumpDevices retrieves and displays all known devices.
func dumpDevices() error {
	client, err := connect()
	if err != nil {
		return err
	}
	defer client.Disconnect(250)

	devices, err := getDevices(client)
	if err != nil {
		return err
	}

	for _, device := range devices {

		// Skip the coordinator
		if device.FriendlyName == "Coordinator" {
			continue
		}
		// show the details.
		fmt.Printf("%s %s %s\n", device.FriendlyName, device.IEEEAddress, device.Description)
	}

	return nil
}

// run finds all matching devices and sets their state.
func run(friendlyName, desiredState string) error {
	desiredState = strings.ToUpper(desiredState)

	if desiredState != "ON" && desiredState != "OFF" {
		fmt.Printf("Invalid desired state.\nState must be ON or OFF not %s\n", desiredState)
		return nil
	}

	client, err := connect()
	if err != nil {
		return err
	}
	defer client.Disconnect(250)

	// Look up all matching devices.
	devices, err := getDevices(client)
	if err != nil {
		return err
	}

	var matches []Device

	for _, device := range devices {
		// Friendly name matches?
		if strings.EqualFold(device.FriendlyName, friendlyName) {
			matches = append(matches, device)
			continue
		}

		// Description matches?
		if strings.Contains(
			strings.ToUpper(device.Description),
			strings.ToUpper(friendlyName),
		) {
			matches = append(matches, device)
		}
	}

	if len(matches) == 0 {
		return fmt.Errorf(
			"no devices matching '%s' found in Zigbee2MQTT",
			friendlyName,
		)
	}

	slog.Info("Found matching devices", "count", len(matches))

	// Set state on all matching devices.
	var failures []string

	for _, device := range matches {
		slog.Info(
			"Found device",
			"friendly_name", device.FriendlyName,
			"description", device.Description,
			"ieee_address", device.IEEEAddress,
		)

		if err := setDeviceState(client, device, desiredState); err != nil {
			slog.Error(
				"Failed to set device state",
				"friendly_name", device.FriendlyName,
				"ieee_address", device.IEEEAddress,
				"desired_state", desiredState,
				"error", err,
			)

			failures = append(failures, device.FriendlyName)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf(
			"failed to set %d of %d matching devices: %s",
			len(failures),
			len(matches),
			strings.Join(failures, ", "),
		)
	}

	slog.Info(
		"Successfully processed all matching devices",
		"device_count", len(matches),
		"desired_state", desiredState,
	)

	return nil
}

// setDeviceState changes the state of a single device.
func setDeviceState(
	client mqtt.Client,
	device Device,
	desiredState string,
) error {
	friendlyName := device.FriendlyName

	logger := slog.With(
		"friendly_name", friendlyName,
		"ieee_address", device.IEEEAddress,
	)

	stateTopic := fmt.Sprintf("%s/%s", baseTopic, friendlyName)
	commandTopic := stateTopic + "/set"
	getTopic := stateTopic + "/get"

	stateChan := make(chan string, 20)

	// Subscribe to device state.
	token := client.Subscribe(stateTopic, 1, func(
		client mqtt.Client,
		msg mqtt.Message,
	) {
		var data struct {
			State string `json:"state"`
		}

		if err := json.Unmarshal(msg.Payload(), &data); err != nil {
			logger.Error("Failed to decode device state", "error", err)
			return
		}

		state := strings.ToUpper(data.State)
		if state == "" {
			return
		}

		select {
		case stateChan <- state:
		default:
			logger.Warn("State channel full; dropping update")
		}
	})

	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("MQTT state subscription timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT state subscription failed: %w", err)
	}

	logger.Info("Subscribed to device state", "topic", stateTopic)

	// Request current device state.
	getPayload, err := json.Marshal(map[string]string{
		"state": "",
	})
	if err != nil {
		return fmt.Errorf("failed to encode state request: %w", err)
	}

	logger.Info("Requesting current device state")

	token = client.Publish(getTopic, 1, false, getPayload)

	if !token.WaitTimeout(10 * time.Second) {
		logger.Warn("State request timed out; continuing")
	} else if err = token.Error(); err != nil {
		logger.Error("State request failed", "error", err)
	}

	// Check initial state.
	currentState, received := waitForState(
		stateChan,
		desiredState,
		stateTimeout,
	)

	if received {
		logger.Info("Received current device state", "state", currentState)

		if currentState == desiredState {
			logger.Info(
				"Device already in desired state",
				"state", desiredState,
			)
			return nil
		}
	} else {
		logger.Warn("No initial state received; proceeding with command")
	}

	// Prepare command payload.
	payload, err := json.Marshal(map[string]string{
		"state": desiredState,
	})
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Attempt to change device state.
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Info(
			"Setting device state",
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"desired_state", desiredState,
		)

		token = client.Publish(commandTopic, 1, false, payload)

		if !token.WaitTimeout(10 * time.Second) {
			logger.Warn("Command publish timed out", "attempt", attempt)
		} else if err := token.Error(); err != nil {
			logger.Error(
				"Command publish failed",
				"attempt", attempt,
				"error", err,
			)
		} else {
			logger.Info("Command published successfully", "attempt", attempt)
		}

		// Wait for the desired state, ignoring unrelated updates.
		currentState, received = waitForState(
			stateChan,
			desiredState,
			stateTimeout,
		)

		if received {
			logger.Info("Device reported state", "state", currentState)

			if currentState == desiredState {
				logger.Info(
					"Successfully changed device state",
					"state", desiredState,
				)
				return nil
			}
		} else {
			logger.Warn("No matching state update received", "attempt", attempt)
		}

		if attempt == maxAttempts {
			break
		}

		// Random delay between 1 and 5 seconds.
		delay := time.Duration(rng.Intn(5)+1) * time.Second

		logger.Info("Retrying device command", "delay", delay.String())
		time.Sleep(delay)
	}

	return fmt.Errorf(
		"failed to set %s to %s after %d attempts",
		friendlyName,
		desiredState,
		maxAttempts,
	)
}

// waitForState waits for the desired state, ignoring unrelated MQTT updates.
func waitForState(
	stateChan <-chan string,
	desiredState string,
	timeout time.Duration,
) (string, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var lastState string
	received := false

	for {
		select {
		case state := <-stateChan:
			lastState = state
			received = true

			if state == desiredState {
				return state, true
			}

		case <-timer.C:
			return lastState, received
		}
	}
}
