"""
令牌桶限流模块 - 控制全局请求速率
"""
import threading
import time


class TokenBucket:
    """
    令牌桶算法实现
    - rate: 每秒生成的令牌数（QPS上限）
    - capacity: 桶的最大容量（允许的突发请求数）
    """

    def __init__(self, rate: float = 2.0, capacity: float = 5.0):
        self.rate = rate
        self.capacity = capacity
        self.tokens = capacity
        self.last_time = time.time()
        self._lock = threading.Lock()

    def _refill(self):
        """补充令牌"""
        now = time.time()
        elapsed = now - self.last_time
        self.tokens = min(self.capacity, self.tokens + elapsed * self.rate)
        self.last_time = now

    def acquire(self, tokens: float = 1.0, blocking: bool = True) -> bool:
        """
        获取令牌
        - tokens: 需要的令牌数
        - blocking: 是否阻塞等待
        返回: 是否成功获取
        """
        with self._lock:
            self._refill()

            if self.tokens >= tokens:
                self.tokens -= tokens
                return True

            if not blocking:
                return False

            # 计算需要等待的时间
            wait_time = (tokens - self.tokens) / self.rate

        # 在锁外等待
        time.sleep(wait_time)

        with self._lock:
            self._refill()
            self.tokens -= tokens
            return True

    def set_rate(self, rate: float):
        """动态调整速率"""
        with self._lock:
            self._refill()
            self.rate = rate


# 全局限流器实例
_global_limiter = None
_limiter_lock = threading.Lock()


def get_rate_limiter(rate: float = 2.0, capacity: float = 5.0) -> TokenBucket:
    """获取全局限流器（单例模式）"""
    global _global_limiter
    if _global_limiter is None:
        with _limiter_lock:
            if _global_limiter is None:
                _global_limiter = TokenBucket(rate, capacity)
    return _global_limiter


def wait_for_token():
    """等待获取一个令牌（便捷函数）"""
    limiter = get_rate_limiter()
    limiter.acquire(1.0, blocking=True)
