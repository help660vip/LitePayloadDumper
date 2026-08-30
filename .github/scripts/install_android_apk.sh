#!/bin/sh

set -eu

apk_path=${1:?APK path is required}
remote_apk=/data/local/tmp/litepayloaddumper-test.apk
attempt=1

cleanup() {
  adb shell rm -f "$remote_apk" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM

adb wait-for-device
adb push "$apk_path" "$remote_apk"

while [ "$attempt" -le 3 ]; do
  adb wait-for-device
  if adb shell cmd package list packages >/dev/null 2>&1 &&
     adb shell pm install -r "$remote_apk"; then
    exit 0
  fi

  echo "APK install attempt $attempt failed; waiting for PackageManager"
  sleep 20
  attempt=$((attempt + 1))
done

echo "APK installation failed after 3 attempts" >&2
exit 1
