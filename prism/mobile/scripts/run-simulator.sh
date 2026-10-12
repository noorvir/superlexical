#!/usr/bin/env bash
# Builds Prism and starts it on an iOS simulator.
# Usage: run-simulator.sh [DEVICE_NAME]
set -euo pipefail

device="${1:-iPhone 17 Pro}"
project="$(cd "$(dirname "$0")/.." && pwd)/Prism.xcodeproj"
derived_data="${TMPDIR:-/tmp}/prism-derived-data"

xcrun simctl boot "$device" 2>/dev/null || true
xcrun simctl bootstatus "$device" >/dev/null
open -a Simulator

xcodebuild \
	-project "$project" \
	-scheme Prism \
	-configuration Debug \
	-destination "platform=iOS Simulator,name=$device" \
	-derivedDataPath "$derived_data" \
	-quiet \
	build

app="$derived_data/Build/Products/Debug-iphonesimulator/Prism.app"
bundle_id="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Info.plist")"

xcrun simctl install "$device" "$app"
xcrun simctl launch --terminate-running-process "$device" "$bundle_id"
