# VEEX Industrial Blueprints 

A collection of ready-to-use industrial templates, optimized for the VEEX ecosystem.

## Available Templates:

### Industrial Communication (CAN, BLE, UART)
- [Gateway CAN-to-Cloud](can/can-gateway.vdl): Industrial CAN bus bridge.
- [Mobile Provisioning (BLE)](ble/provisioning.vdl): Configuration via Bluetooth.
- [Legacy Serial (UART)](serial/uart-bridge.vdl): Modbus/RS485 integration.

### Cloud Connectivity (MQTT, HTTP, gRPC)
- [Remote I/O (MQTT)](mqtt/remote-io.vdl): Remote actuator control.
- [Cloud Data Pusher (HTTP)](http/cloud-push.vdl): Integration with REST APIs.
- [Remote Call (gRPC)](grpc/grpc-call.vdl): Ultra-fast calls to microservices.

### Hardware & Buses (I2C, SPI, GPIO)
- [Industrial Blink](gpio/industrial-blink.vdl): Digital output and status.
- [Peripheral Bus (I2C/SPI)](bus/bus-communication.vdl): Synchronous peripheral reading.
- [Smart Interlock](gpio/smart-interlock.vdl): Local safety locking.

### Sensors & Diagnostics (Sensor, System)
- [Environmental Monitor](sensor/env-monitor.vdl): Multi-sensor reading.
- [Industrial Watchdog](system/health-monitor.vdl): Heartbeats and runtime health.

---

## How to use:
Simply navigate to the folder of the desired technology and use the `veex-cli`:

```bash
# 1. Emulate behavior locally (No hardware required!)
veex sim mqtt/remote-io.vex

# 2. Flash to real hardware
veex flash mqtt/remote-io.vex

# 3. Or compile it yourself from source
veex build -i mqtt/remote-io.vdl
```




