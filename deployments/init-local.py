"""一次性生成本地开发秘密，不覆盖配置或打印密钥。"""
import argparse
import base64
import json
import os
from pathlib import Path
import secrets


def write_secret(directory, name, value):
    """用途：独占写入并刷盘新秘密文件。
    参数：directory 为目录；name 为固定文件名；value 为秘密内容。
    返回值：无，存在目标或写入失败时抛异常。
    """
    descriptor = os.open(directory / name, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(descriptor, "w") as output:
        output.write(value)
        output.flush()
        os.fsync(output.fileno())


def random_key():
    """用途：生成独立的密码学随机密钥。
    参数：无。
    返回值：32 字节密钥的 Base64 表示，仅供秘密文件使用。
    """
    return base64.b64encode(secrets.token_bytes(32)).decode()


def main():
    """用途：为独立本地 Compose 环境生成账号和分用途密钥。
    参数：命令行可用 --directory 指定新秘密目录。
    返回值：无，非空目录拒绝执行，部分失败保留现场。
    """
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, default=Path(__file__).parent / "secrets")
    directory = parser.parse_args().directory
    if directory.is_symlink():
        raise SystemExit("Refusing a symbolic-link secret directory")
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    if list(directory.iterdir()):
        raise SystemExit("Existing secrets preserved; refusing regeneration")
    directory.chmod(0o700)
    root_password, app_password = secrets.token_hex(32), secrets.token_hex(32)
    values = {
        "db_root": root_password,
        "migration_dsn": f"root:{root_password}@tcp(mysql:3306)/jstihuo",
        "app_dsn": f"jstapp:{app_password}@tcp(mysql:3306)/jstihuo",
        "app-user.sql": (
            f"CREATE USER 'jstapp'@'%' IDENTIFIED BY '{app_password}';\n"
            "GRANT SELECT, INSERT, UPDATE, DELETE ON jstihuo.* TO 'jstapp'@'%';\n"
        ),
    }
    for name in ("lookup", "request", "challenge"):
        values[name] = random_key()
    for name in ("pii", "export"):
        values[name + ".json"] = json.dumps({"active": "1", "keys": {"1": random_key()}})
    for name, value in values.items():
        write_secret(directory, name, value)
    print("Local secrets generated; no values printed. Retain backup keys separately.")


if __name__ == "__main__":
    main()
