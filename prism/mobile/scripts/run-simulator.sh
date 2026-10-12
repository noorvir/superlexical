#!/usr/bin/env bash
# Builds Prism and starts it on an iOS simulator. The Simulator window stays
# as it is. To see the device, open Device Hub in Xcode 27 or Simulator in Xcode 26.
# Usage: run-simulator.sh [DEVICE_NAME]
set -euo pipefail

name="${1:-iPhone 17 Pro}"
project="$(cd "$(dirname "$0")/.." && pwd)/Prism.xcodeproj"
derived_data="${TMPDIR:-/tmp}/prism-derived-data"

# Several runtimes can have a device with the same name, so the script selects
# one device by its ID. It prefers a device that is booted.
devices="$(xcrun simctl list devices available | grep -F "    $name (")"
device="$(grep -F "(Booted)" <<<"$devices" | head -1 || true)"
if [ -z "$device" ]; then
	device="$(head -1 <<<"$devices")"
fi
udid="$(grep -oE '[0-9A-F]{8}(-[0-9A-F]{4}){3}-[0-9A-F]{12}' <<<"$device" | head -1)"

xcrun simctl boot "$udid" 2>/dev/null || true
xcrun simctl bootstatus "$udid" >/dev/null

xcodebuild \
	-project "$project" \
	-scheme Prism \
	-configuration Debug \
	-destination "platform=iOS Simulator,id=$udid" \
	-derivedDataPath "$derived_data" \
	-quiet \
	build

app="$derived_data/Build/Products/Debug-iphonesimulator/Prism.app"
bundle_id="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Info.plist")"

xcrun simctl install "$udid" "$app"
xcrun simctl launch --terminate-running-process "$udid" "$bundle_id"
