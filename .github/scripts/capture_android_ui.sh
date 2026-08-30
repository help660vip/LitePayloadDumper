#!/bin/sh

set -eu

output_path=${1:?Output path is required}
remote_path=/data/local/tmp/litepayloaddumper-ui.xml
attempt=1

while [ "$attempt" -le 3 ]; do
  adb shell rm -f "$remote_path" >/dev/null 2>&1 || true
  if adb shell uiautomator dump "$remote_path" &&
     adb pull "$remote_path" "$output_path"; then
    exit 0
  fi

  echo "UI snapshot attempt $attempt failed; retrying"
  adb shell input keyevent 82 >/dev/null 2>&1 || true
  sleep 2
  attempt=$((attempt + 1))
done

echo "Unable to capture Android UI after 3 attempts" >&2
exit 1
