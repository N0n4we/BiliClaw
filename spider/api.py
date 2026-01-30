"""
B站API模块 - 封装所有B站API调用
"""
import requests
import time
import random
import functools
from cookie_pool import get_cookie_pool, is_cookie_error
from rate_limiter import wait_for_token

DEFAULT_HEADERS = {
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Accept': 'application/json, text/plain, */*',
    'Referer': 'https://www.bilibili.com',
}

def create_session():
    """创建并初始化session，从Cookie池获取Cookie"""
    session = requests.Session()

    # 尝试从Cookie池获取Cookie
    pool = get_cookie_pool()
    cookie = pool.get_cookie()

    headers = DEFAULT_HEADERS.copy()
    headers['Cookie'] = cookie
    session.headers.update(headers)

    # 存储当前使用的cookie，用于失效标记
    session._current_cookie = cookie

    session.get("https://www.bilibili.com/", timeout=10)
    return session


def _handle_cookie_error(session, code: int):
    """处理Cookie相关错误，标记失效"""
    if is_cookie_error(code) and hasattr(session, '_current_cookie') and session._current_cookie:
        pool = get_cookie_pool()
        pool.mark_invalid(session._current_cookie)


def retry_with_backoff(max_retries: int = 3, base_delay: float = 1.0, max_delay: float = 30.0):
    """
    重试装饰器，使用指数退避策略
    - max_retries: 最大重试次数
    - base_delay: 基础延迟（秒）
    - max_delay: 最大延迟（秒）
    """
    def decorator(func):
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            last_error = None
            for attempt in range(max_retries + 1):
                try:
                    # 等待令牌（限流）
                    wait_for_token()
                    result = func(*args, **kwargs)
                    # 检查返回值中是否有错误
                    if isinstance(result, tuple) and len(result) >= 2:
                        error = result[-1]
                        if error is None:
                            return result
                        # 有错误，判断是否需要重试
                        if attempt < max_retries:
                            delay = min(base_delay * (2 ** attempt) + random.uniform(0, 1), max_delay)
                            print(f"  [重试] {func.__name__} 第{attempt + 1}次失败: {error}，{delay:.1f}秒后重试...")
                            time.sleep(delay)
                            last_error = error
                            continue
                    return result
                except requests.exceptions.RequestException as e:
                    last_error = str(e)
                    if attempt < max_retries:
                        delay = min(base_delay * (2 ** attempt) + random.uniform(0, 1), max_delay)
                        print(f"  [重试] {func.__name__} 网络错误: {e}，{delay:.1f}秒后重试...")
                        time.sleep(delay)
                    else:
                        # 返回与原函数相同格式的错误
                        return _get_error_return(func.__name__, str(e))
            # 所有重试都失败
            return _get_error_return(func.__name__, last_error)
        return wrapper
    return decorator


def _get_error_return(func_name: str, error: str):
    """根据函数名返回对应格式的错误结果"""
    if func_name == "search_videos":
        return [], 0, error
    elif func_name in ("get_video_aid", "get_video_detail", "get_user_card"):
        return None, error
    elif func_name == "get_main_comments":
        return [], 0, True, error
    elif func_name == "get_reply_comments":
        return [], 0, error
    return None, error


@retry_with_backoff(max_retries=3)
def search_videos(keyword, page=1, page_size=50, session=None):
    """
    搜索视频
    返回: (videos_list, num_pages_total, error_msg)
    """
    url = "https://api.bilibili.com/x/web-interface/search/type"
    params = {
        "page": page,
        "page_size": page_size,
        "keyword": keyword,
        "search_type": "video",
        "order": "",
    }

    try:
        if session:
            response = session.get(url, params=params, timeout=15)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, params=params, timeout=15)

        data = response.json()

        code = data.get("code", 0)
        if code != 0:
            if session:
                _handle_cookie_error(session, code)
            return [], 0, data.get('message', 'Unknown error')

        videos = data.get("data", {}).get("result", [])
        num_pages = data.get("data", {}).get("numPages", 0)
        return videos, num_pages, None

    except Exception as e:
        return [], 0, str(e)


@retry_with_backoff(max_retries=3)
def get_video_aid(bvid, session=None):
    """
    根据bvid获取视频aid
    返回: (aid, error_msg)
    """
    url = "https://api.bilibili.com/x/web-interface/view"
    params = {"bvid": bvid}

    try:
        if session:
            response = session.get(url, params=params, timeout=10)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, params=params, timeout=10)

        data = response.json()

        code = data.get("code", 0)
        if code == 0:
            return data["data"]["aid"], None
        else:
            if session:
                _handle_cookie_error(session, code)
            return None, data.get('message', 'Unknown error')

    except Exception as e:
        return None, str(e)


@retry_with_backoff(max_retries=3)
def get_video_detail(bvid, session=None):
    """
    获取视频详细信息（使用view接口，信息更完整）
    返回: (video_data, error_msg)
    """
    url = "https://api.bilibili.com/x/web-interface/view"
    params = {"bvid": bvid}

    try:
        if session:
            response = session.get(url, params=params, timeout=10)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, params=params, timeout=10)

        data = response.json()

        code = data.get("code", 0)
        if code == 0:
            return data["data"], None
        else:
            if session:
                _handle_cookie_error(session, code)
            return None, data.get('message', 'Unknown error')

    except Exception as e:
        return None, str(e)


@retry_with_backoff(max_retries=3)
def get_main_comments(oid, cursor=0, session=None):
    """
    获取主评论（一级评论）
    返回: (comments_list, next_cursor, is_end, error_msg)
    """
    url = "https://api.bilibili.com/x/v2/reply/main"
    params = {
        "mode": 3,
        "next": cursor,
        "oid": oid,
        "plat": 1,
        "type": 1
    }

    try:
        if session:
            response = session.get(url, params=params, timeout=10)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, params=params, timeout=10)

        data = response.json()

        code = data.get("code", 0)
        if code != 0:
            if session:
                _handle_cookie_error(session, code)
            return [], 0, True, data.get('message', 'Unknown error')

        replies = data.get("data", {}).get("replies", []) or []
        cursor_info = data.get("data", {}).get("cursor", {})
        next_cursor = cursor_info.get("next", 0)
        is_end = cursor_info.get("is_end", True)

        return replies, next_cursor, is_end, None

    except Exception as e:
        return [], 0, True, str(e)


@retry_with_backoff(max_retries=3)
def get_reply_comments(oid, root_rpid, page=1, page_size=20, session=None):
    """
    获取评论回复（二级评论）
    返回: (replies_list, total_count, error_msg)
    """
    url = "https://api.bilibili.com/x/v2/reply/reply"
    params = {
        "oid": oid,
        "type": 1,
        "root": root_rpid,
        "ps": page_size,
        "pn": page
    }

    try:
        if session:
            response = session.get(url, params=params, timeout=10)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, params=params, timeout=10)

        data = response.json()

        code = data.get("code", 0)
        if code != 0:
            if session:
                _handle_cookie_error(session, code)
            return [], 0, data.get('message', 'Unknown error')

        replies = data.get("data", {}).get("replies", []) or []
        page_info = data.get("data", {}).get("page", {})
        total_count = page_info.get("count", 0)

        return replies, total_count, None

    except Exception as e:
        return [], 0, str(e)


@retry_with_backoff(max_retries=3)
def get_user_card(mid, session=None):
    """
    获取用户名片信息
    返回: (user_data, error_msg)
    """
    url = "https://api.bilibili.com/x/web-interface/card"
    params = {"mid": mid, "photo": "true"}

    try:
        if session:
            response = session.get(url, params=params, timeout=10)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, params=params, timeout=10)

        data = response.json()

        code = data.get("code", 0)
        if code == 0:
            return data["data"], None
        else:
            if session:
                _handle_cookie_error(session, code)
            return None, data.get('message', 'Unknown error')

    except Exception as e:
        return None, str(e)
