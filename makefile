SHELL := /bin/bash

GOBIN = /usr/local/bin

.PHONY: help
help: ## Displays the Makefile help.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: install-macos
install-macos: # Build the application, install it and create a launchd process for the current user to automatically run every 30s
	@ go install .
	@ cp scripts/warm-waters.plist ~/Library/LaunchAgents/warm-waters.plist
	@ launchctl load -w ~/Library/LaunchAgents/warm-waters.plist
