import json
import random
import threading
from dataclasses import dataclass, field
from typing import Optional, List
from pathlib import Path
from urllib.parse import urlparse

SUPPORTED_SCHEMES = ("http", "https", "socks4", "socks5", "socks5h")


@dataclass
class ProxyItem:
    url: str
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

    @property
    def as_requests_proxy(self) -> dict:
        return {"http": self.url, "https": self.url}


class ProxyPool:
    def __init__(self, config_path: str = "proxies.json"):
        self._proxies: List[ProxyItem] = []
        self._lock = threading.RLock()
        self._index = 0
        self._strategy = "round_robin"
        self._config_path = Path(config_path)
        self._load_proxies()

    def _load_proxies(self):
        if not self._config_path.exists():
            print(f"[ProxyPool] 配置文件 {self._config_path} 不存在，将直连")
            return

        try:
            with open(self._config_path, "r", encoding="utf-8") as f:
                config = json.load(f)

            settings = config.get("settings", {})
            self._strategy = settings.get("strategy", "round_robin")

            for item in config.get("proxies", []):
                if not item.get("enabled", True):
                    continue
                url = item.get("url", "").strip()
                if not url:
                    continue
                scheme = urlparse(url).scheme
                if scheme not in SUPPORTED_SCHEMES:
                    print(f"[ProxyPool] 不支持的代理协议，已跳过: {url}")
                    continue
                self._proxies.append(ProxyItem(
                    url=url,
                    name=item.get("name", ""),
                    enabled=True,
                ))

            print(f"[ProxyPool] 已加载 {len(self._proxies)} 个代理，策略: {self._strategy}")

        except json.JSONDecodeError as e:
            print(f"[ProxyPool] 配置文件JSON解析错误: {e}")
        except Exception as e:
            print(f"[ProxyPool] 加载配置文件失败: {e}")

    def get_proxy(self) -> Optional[ProxyItem]:
        with self._lock:
            available = [p for p in self._proxies if p.enabled and p.is_valid]
            if not available:
                return None
            if self._strategy == "random":
                return random.choice(available)
            self._index = self._index % len(available)
            proxy = available[self._index]
            self._index += 1
            return proxy

    def mark_invalid(self, proxy_url: str, permanent: bool = False):
        with self._lock:
            for proxy in self._proxies:
                if proxy.url == proxy_url:
                    if permanent:
                        proxy.is_valid = False
                        proxy.enabled = False
                        print(f"[ProxyPool] 代理 '{proxy.name or proxy_url}' 已永久禁用")
                    else:
                        disabled = proxy.mark_failed()
                        if disabled:
                            print(f"[ProxyPool] 代理 '{proxy.name or proxy_url}' 失败次数过多，已禁用")
                        else:
                            print(f"[ProxyPool] 代理 '{proxy.name or proxy_url}' 失败 {proxy.fail_count}/{proxy.max_fails}")
                    break

    def get_status(self) -> dict:
        with self._lock:
            total = len(self._proxies)
            valid = sum(1 for p in self._proxies if p.enabled and p.is_valid)
            return {"total": total, "valid": valid, "strategy": self._strategy}

    def __len__(self) -> int:
        with self._lock:
            return sum(1 for p in self._proxies if p.enabled and p.is_valid)


_proxy_pool: Optional[ProxyPool] = None
_pool_lock = threading.Lock()


def get_proxy_pool(config_path: str = "proxies.json") -> ProxyPool:
    global _proxy_pool
    if _proxy_pool is None:
        with _pool_lock:
            if _proxy_pool is None:
                _proxy_pool = ProxyPool(config_path)
    return _proxy_pool


def is_proxy_error(exception: Exception) -> bool:
    """判断异常是否由代理引起（连接失败、超时等）"""
    import requests.exceptions as rex
    return isinstance(exception, (rex.ProxyError, rex.ConnectTimeout, rex.ConnectionError))
