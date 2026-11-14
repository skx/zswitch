# zswitch

A trivial program to control zigbee devices, via direct contact with the zigbee2mqtt queue.



## Justification

Sending a message to set a device on/off can be done via mosquitto_pub[lish]:

     mosquitto_pub -h cube.my.flat -t 'zigbee2mqtt/0xa4c1381e881affff/get'   -m '{"state":"off"}'

But sometimes that just doesn't work, so you find yourself repeating it, and then adding retries.  Painful.

Instead better to connect to the queue:

* Read the state of each device.
  * Continue if the state is already what you wanted.
* Otherwise send an update to change the state.
  * Repeating until success.
* Move on to the next matching device.



## Configuration

This defaults to connecting to the MQ host "cube.my.flat", set the environmental variable "MQ_HOST" if you want to write elsewhere.



## Example

Turning off a single switch, with debugging enabled:

```sh
$ zswitch -debug socket-a off
{"time":"2026-11-14T10:49:17.839379796+02:00","level":"INFO","msg":"Connecting to MQTT broker","broker":"tcp://cube.my.flat:1883"}
{"time":"2026-11-14T10:49:18.065197544+02:00","level":"INFO","msg":"Connected to MQTT broker"}
{"time":"2026-11-14T10:49:18.086485833+02:00","level":"INFO","msg":"Waiting for Zigbee2MQTT device list","topic":"zigbee2mqtt/bridge/devices"}
{"time":"2026-11-14T10:49:18.270715252+02:00","level":"INFO","msg":"Received device list","device_count":6}
{"time":"2026-11-14T10:49:18.270757182+02:00","level":"INFO","msg":"Found matching devices","count":1}
{"time":"2026-11-14T10:49:18.270772287+02:00","level":"INFO","msg":"Found device","friendly_name":"socket-A","description":"Living room sofa lamp","ieee_address":"0xa4c1381e881affff"}
{"time":"2026-11-14T10:49:18.284513257+02:00","level":"INFO","msg":"Subscribed to device state","friendly_name":"socket-A","ieee_address":"0xa4c1381e881affff","topic":"zigbee2mqtt/socket-A"}
{"time":"2026-11-14T10:49:18.284633094+02:00","level":"INFO","msg":"Requesting current device state","friendly_name":"socket-A","ieee_address":"0xa4c1381e881affff"}
{"time":"2026-11-14T10:49:23.372656535+02:00","level":"INFO","msg":"Received current device state","friendly_name":"socket-A","ieee_address":"0xa4c1381e881affff","state":"ON"}
{"time":"2026-11-14T10:49:23.372837331+02:00","level":"INFO","msg":"Setting device state","friendly_name":"socket-A","ieee_address":"0xa4c1381e881affff","attempt":1,"max_attempts":10,"desired_state":"OFF"}
{"time":"2026-11-14T10:49:23.54683792+02:00","level":"INFO","msg":"Command published successfully","friendly_name":"socket-A","ieee_address":"0xa4c1381e881affff","attempt":1}
{"time":"2026-11-14T10:49:23.64534535+02:00","level":"INFO","msg":"Device reported state","friendly_name":"socket-A","ieee_address":"0xa4c1381e881affff","state":"OFF"}
{"time":"2026-11-14T10:49:23.64538217+02:00","level":"INFO","msg":"Successfully changed device state","friendly_name":"socket-A","ieee_address":"0xa4c1381e881affff","state":"OFF"}
{"time":"2026-11-14T10:49:23.645405081+02:00","level":"INFO","msg":"Successfully processed all matching devices","device_count":1,"desired_state":"OFF"}
```



## Notes

All available devices are discovered dynamically, and the specified name will be tested against:

* The friendly name of the switch.
* The description of the switch.

So assuming you have three switches each of which having a description "bedroom small lamp", "bedroom big lamp", "bedroom corner lamp", you can turn them all on/off like so:

      zswitch bedroom on

You can sanity-check your name/address/descriptions via a manual dump like so:

```sh
 mosquitto_sub -h cube.my.flat -t 'zigbee2mqtt/bridge/devices'   -C 1 | jq '.[] | {friendly_name, ieee_address, description}'
```

Note that we can also dump devices:

```
$ ./zswitch -dump
socket-A 0xa4c1381e881affff Living room sofa lamp
socket-B 0xa4c1381e40d7ffff coffee machine
socket-C 0xa4c1381e4cc7ffff Bedroom single spotlight
socket-D 0xa4c1381e8a63ffff Bedroom double lamps on shelf
..
```
