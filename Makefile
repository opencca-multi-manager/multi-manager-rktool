BINARY      := manager
INSTALL_BIN := /usr/local/sbin/rktool-manager
WRAPPER     := /usr/local/bin/rktool
UDEV_RULES  := /etc/udev/rules.d/99-opencca.rules
SUDOERS     := /etc/sudoers.d/opencca
CONFIG_DIR  := /etc/opencca

.PHONY: build install uninstall

build:
	go build -o $(BINARY) .

install:
	@[ "$$(id -u)" = "0" ] || { echo "must be run as root"; exit 1; }
	getent group opencca-admin > /dev/null || groupadd --system opencca-admin
	install -o root -g opencca-admin -m 0750 $(BINARY) $(INSTALL_BIN)
	install -o root -g root -m 0755 install/rktool $(WRAPPER)
	install -o root -g root -m 0644 install/99-opencca.rules $(UDEV_RULES)
	udevadm control --reload-rules && udevadm trigger
	visudo -c -f install/opencca.sudoers
	install -o root -g root -m 0440 install/opencca.sudoers $(SUDOERS)
	mkdir -p $(CONFIG_DIR)
	@[ -f $(CONFIG_DIR)/boards.yaml ] || install -o root -g opencca-admin -m 0640 boards.yaml $(CONFIG_DIR)/boards.yaml

uninstall:
	@[ "$$(id -u)" = "0" ] || { echo "must be run as root"; exit 1; }
	rm -f $(INSTALL_BIN) $(WRAPPER) $(UDEV_RULES) $(SUDOERS)
	udevadm control --reload-rules && udevadm trigger
