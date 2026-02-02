import json
import random
import threading
import requests
from dataclasses import dataclass, field
from typing import Optional, List
from pathlib import Path


@dataclass
class CookieItem:
    value: str
    name: str = ""
    enabled: bool = True
    is_valid: bool = True
    fail_count: int = 0
    max_fails: int = 3

    def mark_failed(self) -> bool:
        self.fail_count += 1
        if self.fail_count >= self.max_fails:
            self.is_valid = False
            return True
        return False

    def reset(self):
        self.fail_count = 0
        self.is_valid = True


class CookiePool:
    def __init__(self, config_path: str = "cookies.json"):
        self._cookies: List[CookieItem] = []
        self._lock = threading.RLock()
        self._index = 0
        self._strategy = "round_robin"  # round_robin 或 random
        self._config_path = Path(config_path)
        self._load_cookies()

    def _load_cookies(self):
        if not self._config_path.exists():
            print(f"[CookiePool] 配置文件 {self._config_path} 不存在")
            return

        try:
            with open(self._config_path, 'r', encoding='utf-8') as f:
                config = json.load(f)

            settings = config.get("settings", {})
            self._strategy = settings.get("strategy", "round_robin")

            for item in config.get("cookies", []):
                if item.get("enabled", True):
                    cookie = CookieItem(
                        value=item.get("value", ""),
                        name=item.get("name", ""),
                        enabled=item.get("enabled", True)
                    )
                    if cookie.value:
                        self._cookies.append(cookie)

            print(f"[CookiePool] 已加载 {len(self._cookies)} 个Cookie，策略: {self._strategy}")

            if settings.get("validate_on_load", False):
                self.validate_all()

        except json.JSONDecodeError as e:
            print(f"[CookiePool] 配置文件JSON解析错误: {e}")
        except Exception as e:
            print(f"[CookiePool] 加载配置文件失败: {e}")

    def get_cookie(self) -> Optional[str]:
        with self._lock:
            available = [c for c in self._cookies if c.enabled and c.is_valid]
            if not available:
                return None

            if self._strategy == "random":
                cookie = random.choice(available)
            else:  # round_robin
                self._index = self._index % len(available)
                cookie = available[self._index]
                self._index += 1

            return cookie.value

    def get_cookie_item(self) -> Optional[CookieItem]:
        with self._lock:
            available = [c for c in self._cookies if c.enabled and c.is_valid]
            if not available:
                return None

            if self._strategy == "random":
                return random.choice(available)
            else:  # round_robin
                self._index = self._index % len(available)
                cookie = available[self._index]
                self._index += 1
                return cookie

    def mark_invalid(self, cookie_value: str, permanent: bool = False):
        with self._lock:
            for cookie in self._cookies:
                if cookie.value == cookie_value:
                    if permanent:
                        cookie.is_valid = False
                        cookie.enabled = False
                        print(f"[CookiePool] Cookie '{cookie.name}' 已永久禁用")
                    else:
                        disabled = cookie.mark_failed()
                        if disabled:
                            print(f"[CookiePool] Cookie '{cookie.name}' 失败次数过多，已禁用")
                        else:
                            print(f"[CookiePool] Cookie '{cookie.name}' 失败 {cookie.fail_count}/{cookie.max_fails}")
                    break

    def validate_cookie(self, cookie_value: str) -> bool:
        url = "https://api.bilibili.com/x/web-interface/nav"
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
            'Cookie': cookie_value
        }
        try:
            response = requests.get(url, headers=headers, timeout=10)
            data = response.json()
            # code为0表示已登录，-101表示未登录
            return data.get("code") == 0
        except Exception:
            return False

    def validate_all(self):
        print("[CookiePool] 开始验证所有Cookie...")
        with self._lock:
            for cookie in self._cookies:
                if cookie.enabled:
                    is_valid = self.validate_cookie(cookie.value)
                    cookie.is_valid = is_valid
                    status = "有效" if is_valid else "无效"
                    print(f"[CookiePool] Cookie '{cookie.name}': {status}")

    def get_status(self) -> dict:
        with self._lock:
            total = len(self._cookies)
            enabled = sum(1 for c in self._cookies if c.enabled)
            valid = sum(1 for c in self._cookies if c.enabled and c.is_valid)
            return {
                "total": total,
                "enabled": enabled,
                "valid": valid,
                "strategy": self._strategy
            }

    def __len__(self) -> int:
        with self._lock:
            return sum(1 for c in self._cookies if c.enabled and c.is_valid)


_cookie_pool: Optional[CookiePool] = None
_pool_lock = threading.Lock()


def get_cookie_pool(config_path: str = "cookies.json") -> CookiePool:
    """获取Cookie池全局单例"""
    global _cookie_pool
    if _cookie_pool is None:
        with _pool_lock:
            if _cookie_pool is None:
                _cookie_pool = CookiePool(config_path)
    return _cookie_pool


def is_cookie_error(code: int) -> bool:
    # -101: 未登录
    # -352: 风控校验失败
    # -412: 请求被拦截
    return code in (-101, -352, -412)
