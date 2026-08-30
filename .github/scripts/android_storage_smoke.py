import re
import shutil
import subprocess
import time
import xml.etree.ElementTree as ET
from pathlib import Path


REMOTE_XML = "/data/local/tmp/litepayloaddumper-storage.xml"
LOCAL_XML = Path("android-storage-current.xml")
APP_PACKAGE = "com.help660.litepayloaddumper"
APP_ACTIVITY = f"{APP_PACKAGE}/.MainActivity"


def adb(*args, check=True):
    return subprocess.run(
        ["adb", *args], check=check, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
    )


def dump_ui(retries=12):
    for _ in range(retries):
        result = adb("shell", "uiautomator", "dump", REMOTE_XML, check=False)
        if result.returncode == 0:
            pulled = adb("pull", REMOTE_XML, str(LOCAL_XML), check=False)
            if pulled.returncode == 0 and LOCAL_XML.exists():
                return list(ET.parse(LOCAL_XML).iter("node"))
        time.sleep(1)
    raise RuntimeError("无法读取 Android 界面层级")


def save_snapshot(name):
    if LOCAL_XML.exists():
        shutil.copyfile(LOCAL_XML, f"android-storage-{name}.xml")


def box(node):
    values = [int(value) for value in re.findall(r"\d+", node.attrib.get("bounds", ""))]
    if len(values) != 4:
        raise RuntimeError("界面控件没有有效坐标")
    return values


def center(node):
    left, top, right, bottom = box(node)
    return (left + right) // 2, (top + bottom) // 2


def tap(node):
    x, y = center(node)
    adb("shell", "input", "tap", str(x), str(y))
    time.sleep(1)


def clickable(node):
    return node.attrib.get("clickable") == "true" and node.attrib.get("enabled") == "true"


def text(node):
    return node.attrib.get("text", "").strip()


def find_clickable(nodes, texts=(), id_suffixes=(), descriptions=(), classes=()):
    wanted_texts = {value.casefold() for value in texts}
    wanted_descriptions = {value.casefold() for value in descriptions}
    wanted_classes = set(classes)
    for node in nodes:
        if not clickable(node):
            continue
        node_text = text(node).casefold()
        node_id = node.attrib.get("resource-id", "")
        description = node.attrib.get("content-desc", "").strip().casefold()
        node_class = node.attrib.get("class", "")
        if node_text in wanted_texts:
            return node
        if any(node_id.endswith(value) for value in id_suffixes):
            return node
        if description in wanted_descriptions:
            return node
        if node_class in wanted_classes:
            return node
    for label in nodes:
        if text(label).casefold() not in wanted_texts:
            continue
        x, y = center(label)
        parents = []
        for candidate in nodes:
            if not clickable(candidate):
                continue
            left, top, right, bottom = box(candidate)
            if left <= x <= right and top <= y <= bottom:
                parents.append(((right - left) * (bottom - top), candidate))
        if parents:
            return min(parents, key=lambda value: value[0])[1]
    return None


def scroll_up():
    size = adb("shell", "wm", "size").stdout
    match = re.search(r"(\d+)x(\d+)", size)
    width, height = (1080, 1920) if match is None else (int(match.group(1)), int(match.group(2)))
    x = max(10, width // 50)
    adb("shell", "input", "swipe", str(x), str(height * 3 // 4), str(x), str(height // 4), "300")
    time.sleep(1)


def manage_storage_granted():
    result = adb(
        "shell", "appops", "get", APP_PACKAGE, "MANAGE_EXTERNAL_STORAGE", check=False
    ).stdout.casefold()
    return "allow" in result


def legacy_storage_granted():
    result = adb("shell", "dumpsys", "package", APP_PACKAGE, check=False).stdout
    return (
        "android.permission.READ_EXTERNAL_STORAGE: granted=true" in result
        and "android.permission.WRITE_EXTERNAL_STORAGE: granted=true" in result
    )


def grant_storage_permission(sdk):
    if sdk >= 30:
        saw_settings = False
        for _ in range(25):
            if manage_storage_granted():
                if not saw_settings:
                    raise RuntimeError("应用没有打开“管理所有文件”权限设置页")
                adb("shell", "input", "keyevent", "4", check=False)
                adb("shell", "am", "start", "-n", APP_ACTIVITY)
                time.sleep(2)
                return
            nodes = dump_ui()
            if any(node.attrib.get("package") == "com.android.settings" for node in nodes):
                saw_settings = True
            switch = find_clickable(
                nodes,
                texts=(
                    "Allow access to manage all files",
                    "允许访问所有文件",
                    "允许管理所有文件",
                ),
                id_suffixes=("/switch_widget",),
                classes=("android.widget.Switch",),
            )
            if switch is not None and switch.attrib.get("checked") != "true":
                save_snapshot("manage-permission")
                tap(switch)
            else:
                time.sleep(1)
        raise RuntimeError("没有成功授予“管理所有文件”权限")

    for _ in range(20):
        if legacy_storage_granted():
            adb("shell", "am", "start", "-n", APP_ACTIVITY)
            time.sleep(2)
            return
        nodes = dump_ui()
        allow = find_clickable(
            nodes,
            texts=("Allow", "ALLOW", "允许"),
            id_suffixes=(
                "/permission_allow_button",
                "/permission_allow_foreground_only_button",
                "android:id/button1",
            ),
        )
        if allow is not None:
            save_snapshot("legacy-permission")
            tap(allow)
        else:
            time.sleep(1)
    raise RuntimeError("没有成功授予存储读写权限")


def wait_for_app():
    for _ in range(20):
        nodes = dump_ui()
        if any(node.attrib.get("package") == APP_PACKAGE for node in nodes):
            return nodes
        time.sleep(1)
    raise RuntimeError("授权后没有返回 LitePayloadDumper")


def click_app_text(value, attempts=14):
    for _ in range(attempts):
        nodes = dump_ui()
        target = find_clickable(nodes, texts=(value,))
        if target is not None and target.attrib.get("package") == APP_PACKAGE:
            tap(target)
            return
        scroll_up()
    raise RuntimeError(f"没有找到“{value}”")


def click_dialog_text(value, attempts=12):
    for _ in range(attempts):
        nodes = dump_ui()
        target = find_clickable(nodes, texts=(value,))
        if target is not None:
            tap(target)
            return
        time.sleep(1)
    raise RuntimeError(f"对话框没有找到“{value}”")


def create_and_select_test_directory(sdk):
    # Older Android input-text implementations can drop mixed-case characters.
    # Keep the test name lowercase so the directory we verify is exactly what the
    # emulator typed into the application's create-directory dialog.
    directory_name = f"lpdtest{sdk}"
    directory_path = f"/storage/emulated/0/Download/{directory_name}"
    exists = adb("shell", "test", "-e", directory_path, check=False)
    if exists.returncode == 0:
        raise RuntimeError(f"测试目录意外存在：{directory_path}")

    click_app_text("选择目录")
    editor = None
    for _ in range(12):
        nodes = dump_ui()
        editor = next(
            (
                node
                for node in nodes
                if node.attrib.get("class") == "android.widget.EditText"
                and node.attrib.get("package") == APP_PACKAGE
            ),
            None,
        )
        if editor is not None:
            break
        create = find_clickable(nodes, texts=("新建文件夹",))
        if create is not None and create.attrib.get("package") == APP_PACKAGE:
            tap(create)
        else:
            time.sleep(1)
    if editor is None:
        raise RuntimeError("新建文件夹对话框没有输入框")
    tap(editor)
    adb("shell", "input", "text", directory_name)
    click_dialog_text("新建")
    click_dialog_text("选择此目录")
    return directory_path


def wait_for_direct_write(snapshot, expected_path):
    marker = "保存目录已通过全部文件权限直接写入"
    for attempt in range(35):
        nodes = dump_ui()
        has_marker = any(marker in text(node) for node in nodes)
        has_path = any(text(node) == expected_path for node in nodes)
        if has_marker and has_path:
            save_snapshot(snapshot)
            return
        if attempt % 2:
            scroll_up()
        else:
            time.sleep(1)
    raise RuntimeError("应用没有确认保存路径，并通过提取核心的创建、写入和删除验证")


def main():
    directory_path = None
    try:
        sdk = int(adb("shell", "getprop", "ro.build.version.sdk").stdout.strip())
        if sdk < 28:
            raise RuntimeError("Android 自动测试版本低于 9")
        grant_storage_permission(sdk)
        wait_for_app()
        directory_path = create_and_select_test_directory(sdk)
        wait_for_direct_write("direct-write", directory_path)

        adb("shell", "am", "force-stop", APP_PACKAGE)
        adb("shell", "am", "start", "-n", APP_ACTIVITY)
        time.sleep(2)
        wait_for_direct_write("restored", directory_path)
        if sdk >= 30 and not manage_storage_granted():
            raise RuntimeError("应用重启后丢失了全部文件访问权限")
        if sdk < 30 and not legacy_storage_granted():
            raise RuntimeError("应用重启后丢失了存储读写权限")
        print(
            "Storage permission, direct create/write/delete, directory creation, "
            "and restart checks passed"
        )
    except Exception:
        save_snapshot("failure")
        raise
    finally:
        if directory_path is not None:
            adb("shell", "am", "force-stop", APP_PACKAGE, check=False)
            adb("shell", "rmdir", directory_path, check=False)


if __name__ == "__main__":
    main()
