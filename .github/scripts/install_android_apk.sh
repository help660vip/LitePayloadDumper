#!/bin/sh

set -eu

apk_path=${1:?APK path is required}
attempt=1

while [ "$attempt" -le 3 ]; do
  adb wait-for-device
  if adb shell cmd package list packages >/dev/null 2>&1 &&
     adb install -r "$apk_path"; then
    exit 0
  fi

  echo "APK install attempt $attempt failed; waiting for PackageManager"
  adb reconnect || true
  adb wait-for-device
  sleep 10
  attempt=$((attempt + 1))
done

echo "APK installation failed after 3 attempts" >&2
exit 1
