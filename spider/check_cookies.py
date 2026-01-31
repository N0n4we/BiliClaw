"""
验证Cookie有效性
"""
from cookie_pool import get_cookie_pool

def check_cookies():
    pool = get_cookie_pool()
    print(f"\nCookie池状态: {pool.get_status()}")
    print("\n开始验证Cookie...")
    pool.validate_all()
    print(f"\n验证后状态: {pool.get_status()}")

if __name__ == "__main__":
    check_cookies()
