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




# Example

Turning off a single switch:

```sh
$ zswitch -debug socket-a off
```

Note that the name given, `socket-a` in this example, will be treated as a substring to match against the name/description of all switches.  So you can set all off via "switch-".



## Configuration

By default we connect to the MQ instance on `cube.my.flat` but command-line switches can configure the endpoint-details:

| Flag       |                                    Purpose |
+-------------+------------------------------------------------+
| `-mq-host` | Specify the hostname or IP of the queue.   |
| `-mq-user` | Specify the username to connect with.      |
| `-mq-pass` | Specify the password to connect with.      |
| `-mq-port` | Specify the port-number to contact.        |



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



## Links

* Applications
  * [zswitch](https://github.com/skx/zswitch) - Simple CLI to change state of zigbee switches.
  * [zweb](https://github.com/skx/zweb) - Simple WebUI for switch-devices.
* Library
  * [ziglib](https://github.com/skx/ziglib/) - Common library for accessing a Zigbee2mqtt rabbitmq host directly.  Used by both zswitch and zweb.
