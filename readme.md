# opencca-manager-rktool

This is a wrapper around rkdeveloptool for the opencca-manager node.

### Build

```
make build
```

### Install on manager node

```
sudo make install
```

### Dependencies
- Requires opencca-manager box
- Requires patch in rkdeveloptool to select a board (opencca-manager-rkdeveloptool)

### Design
- A yaml file defines the available opencca boards and assigns linux users to boards.
- Users then use this tool to interact with boards: flash, power cycle, uart.
- Only opencca-admin group is allowed to interact with hardware directly, 
  all other uses must invoke this tool (through sudo) to interact with board.
  
  
### TODO
- [ ] Implement maskrom procedure: invoke esp32, API rest endpoing
- [ ] We probably want to keep the waiting time configurable for maskrom/ uhubctl/ rest API
- [ ] What happens if uart is stuck, can a non admin user reconnect to its own board?
- [ ] gpio tool: how to handle reset?
- [ ] what happens if host is rebooted, does usb hub assign same uart to save /dev/ device?
- [ ] How does hub handle when usb devices are plugged into wrong hub? can we add udev rules that create reproducible symlinks to uart?
