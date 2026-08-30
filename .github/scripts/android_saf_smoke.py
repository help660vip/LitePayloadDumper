import re
import shutil
import subprocess
import time
import xml.etree.ElementTree as ET
from pathlib import Path


REMOTE_XML = "/data/local/tmp/litepayloaddumper-saf.xml"
LOCAL_XML = Path("android-saf-current.xml")
APP_PACKAGE = "com.help660.litepayloaddumper"


def adb(*args, check=True):
    return subprocess.run(
        ["adb", *args], check=check, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
    )


def dump_ui(retries=8):
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
        shutil.copyfile(LOCAL_XML, f"android-saf-{name}.xml")


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


def scroll_up():
    size = adb("shell", "wm", "size").stdout
    match = re.search(r"(\d+)x(\d+)", size)
    width, height = (1080, 1920) if match is None else (int(match.group(1)), int(match.group(2)))
    adb(
        "shell", "input", "swipe", str(width // 2), str(height * 3 // 4),
        str(width // 2), str(height // 4), "300"
    )
    time.sleep(1)


def click_app_directory_button():
    for _ in range(8):
        nodes = dump_ui()
        for node in nodes:
            if node.attrib.get("package") == APP_PACKAGE and text(node) == "授权目录" and clickable(node):
                tap(node)
                return
        scroll_up()
    raise RuntimeError("没有找到“授权目录”按钮")


def find_clickable(nodes, texts=(), id_suffixes=(), descriptions=()):
    wanted_texts = {value.casefold() for value in texts}
    wanted_descriptions = {value.casefold() for value in descriptions}
    for node in nodes:
        if not clickable(node):
            continue
        node_text = text(node).casefold()
        node_id = node.attrib.get("resource-id", "")
        description = node.attrib.get("content-desc", "").strip().casefold()
        if node_text in wanted_texts:
            return node
        if any(node_id.endswith(value) for value in id_suffixes):
            return node
        if description in wanted_descriptions:
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


def wait_for_clickable_text(values, attempts=10):
    for _ in range(attempts):
        nodes = dump_ui()
        result = find_clickable(nodes, texts=values)
        if result is not None:
            return result, nodes
        time.sleep(1)
    return None, nodes


def choose_downloads_tree():
    nodes = dump_ui()
    downloads = find_clickable(nodes, texts=("Downloads", "下载"))
    if downloads is None:
        drawer = find_clickable(
            nodes,
            id_suffixes=("/hamburger", "/roots_toolbar"),
            descriptions=("Show roots", "Open navigation drawer", "显示根目录", "打开导航抽屉"),
        )
        if drawer is not None:
            tap(drawer)
            downloads, nodes = wait_for_clickable_text(("Downloads", "下载"))
    if downloads is not None:
        tap(downloads)
        nodes = dump_ui()
    else:
        adb("shell", "input", "keyevent", "4")
        nodes = dump_ui()
        more = find_clickable(nodes, descriptions=("More options", "更多选项"))
        if more is not None:
            tap(more)
            show_storage, nodes = wait_for_clickable_text(
                ("Show internal storage", "显示内部存储空间", "显示内部存储")
            )
            if show_storage is not None:
                tap(show_storage)
                nodes = dump_ui()
        drawer = find_clickable(
            nodes,
            descriptions=("Show roots", "Open navigation drawer", "显示根目录", "打开导航抽屉"),
        )
        if drawer is not None:
            tap(drawer)
        storage, nodes = wait_for_clickable_text(
            ("Internal storage", "内部存储空间", "内部存储", "Pixel_2")
        )
        if storage is None:
            raise RuntimeError("目录授权页没有 Downloads 或内部存储根目录")
        tap(storage)
        nodes = dump_ui()

    select = find_clickable(
        nodes,
        texts=("Use this folder", "USE THIS FOLDER", "Select", "SELECT", "使用此文件夹", "选择"),
        id_suffixes=("/action_menu_select", "/save"),
    )
    if select is None:
        raise RuntimeError("目录授权页没有可用的确认按钮")
    save_snapshot("picker")
    tap(select)

    for _ in range(12):
        nodes = dump_ui()
        if any(node.attrib.get("package") == APP_PACKAGE for node in nodes):
            return
        allow = find_clickable(
            nodes,
            texts=("Allow", "ALLOW", "允许"),
            id_suffixes=("android:id/button1", "/button1"),
        )
        if allow is not None:
            tap(allow)
        else:
            time.sleep(1)
    raise RuntimeError("目录授权后没有返回 LitePayloadDumper")


def wait_for_write_confirmation(snapshot):
    for attempt in range(30):
        nodes = dump_ui()
        if any("保存目录已获得写入权限" in text(node) for node in nodes):
            save_snapshot(snapshot)
            return
        if attempt % 2 == 1:
            scroll_up()
        else:
            time.sleep(1)
    raise RuntimeError("应用没有确认保存目录可写")


def main():
    try:
        adb("shell", "mkdir", "-p", "/sdcard/Download")
        click_app_directory_button()
        choose_downloads_tree()
        wait_for_write_confirmation("granted")

        listing = adb("shell", "ls", "-a", "/sdcard").stdout
        listing += adb("shell", "ls", "-a", "/sdcard/Download").stdout
        if "LitePayloadDumper-write-test-" in listing:
            raise RuntimeError("目录写入测试留下了未清理文件")

        adb("shell", "am", "force-stop", APP_PACKAGE)
        adb("shell", "am", "start", "-n", f"{APP_PACKAGE}/.MainActivity")
        time.sleep(2)
        wait_for_write_confirmation("restored")
        print("SAF directory create/write/delete and persisted permission checks passed")
    except Exception:
        save_snapshot("failure")
        raise


if __name__ == "__main__":
    main()
