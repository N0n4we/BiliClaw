import threading
import time

class TokenBucket:
    def __init__(self, rate: float = 2.0, capacity: float = 5.0):
        self.rate = rate
        self.capacity = capacity
        self.tokens = capacity
        self.last_time = time.time()
        self._lock = threading.Lock()

    def _refill(self):
        now = time.time()
        elapsed = now - self.last_time
        self.tokens = min(self.capacity, self.tokens + elapsed * self.rate)
        self.last_time = now

    def acquire(self, tokens: float = 1.0, blocking: bool = True) -> bool:
        while True:
            with self._lock:
                self._refill()
                if self.tokens >= tokens:
                    self.tokens -= tokens
                    return True
                if not blocking:
                    return False
                wait_time = (tokens - self.tokens) / self.rate
            time.sleep(wait_time)

    def set_rate(self, rate: float):
        with self._lock:
            self._refill()
            self.rate = rate

_global_limiter = None
_limiter_lock = threading.Lock()

def init_rate_limiter(rate: float = 2.0, capacity: float = 5.0):
    """Initialize the global rate limiter with custom rate and capacity"""
    global _global_limiter
    with _limiter_lock:
        _global_limiter = TokenBucket(rate, capacity)
        print(f"[RateLimiter] 已初始化: rate={rate}/s, capacity={capacity}")

def get_rate_limiter(rate: float = 2.0, capacity: float = 5.0) -> TokenBucket:
    global _global_limiter
    if _global_limiter is None:
        with _limiter_lock:
            if _global_limiter is None:
                _global_limiter = TokenBucket(rate, capacity)
    return _global_limiter

def wait_for_token():
    limiter = get_rate_limiter()
    limiter.acquire(1.0, blocking=True)
