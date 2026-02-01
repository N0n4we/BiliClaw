"""
B站API模块 - 封装所有B站API调用
"""
import requests
import time
import random
import functools
import hashlib
import urllib.parse
from cookie_pool import get_cookie_pool, is_cookie_error
from rate_limiter import wait_for_token

DEFAULT_HEADERS = {
    'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0',
    'Accept': 'application/json, text/plain, */*',
    'Referer': 'https://www.bilibili.com',
}

# WBI签名相关
_wbi_mixin_key = None
_wbi_key_expire_time = 0
WBI_KEY_CACHE_SECONDS = 3600  # 缓存1小时

# WBI混淆索引表（固定值）
WBI_MIXIN_KEY_ENC_TAB = [
    46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
    27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
    37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
    22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52
]


def _md5(text: str) -> str:
    """MD5加密"""
    return hashlib.md5(text.encode('utf-8')).hexdigest()


def _get_mixin_key(orig: str) -> str:
    """根据原始key生成混淆后的mixin_key"""
    return ''.join([orig[i] for i in WBI_MIXIN_KEY_ENC_TAB])[:32]


def _get_wbi_keys(session=None) -> tuple:
    """
    从B站nav接口获取img_key和sub_key
    返回: (img_key, sub_key)
    """
    url = "https://api.bilibili.com/x/web-interface/nav"
    try:
        if session:
            response = session.get(url, timeout=10)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, timeout=10)

        data = response.json()
        # 即使未登录(code=-101)，也会返回wbi_img
        wbi_img = data.get("data", {}).get("wbi_img")
        if wbi_img:
            img_url = wbi_img["img_url"]
            sub_url = wbi_img["sub_url"]
            # 从URL中提取key（去掉路径和扩展名）
            img_key = img_url.rsplit('/', 1)[1].split('.')[0]
            sub_key = sub_url.rsplit('/', 1)[1].split('.')[0]
            return img_key, sub_key
    except Exception as e:
        print(f"[WBI] 获取wbi_keys失败: {e}")
    return None, None


def get_wbi_mixin_key(session=None) -> str:
    """
    获取WBI mixin_key（带缓存）
    """
    global _wbi_mixin_key, _wbi_key_expire_time

    current_time = time.time()
    if _wbi_mixin_key and current_time < _wbi_key_expire_time:
        return _wbi_mixin_key

    img_key, sub_key = _get_wbi_keys(session)
    if img_key and sub_key:
        _wbi_mixin_key = _get_mixin_key(img_key + sub_key)
        _wbi_key_expire_time = current_time + WBI_KEY_CACHE_SECONDS
        print(f"[WBI] 已更新mixin_key: {_wbi_mixin_key[:8]}...")
        return _wbi_mixin_key

    # 如果获取失败，使用备用盐值（可能已过期）
    print("[WBI] 警告: 无法获取新的mixin_key，使用备用值")
    return 'ea1db124af3c7062474693fa704f4ff8'


def _generate_wbi_sign(params: dict, session=None) -> tuple:
    """
    生成WBI签名
    params: 请求参数字典（不含wts和w_rid）
    返回: (w_rid签名, wts时间戳)
    """
    mixin_key = get_wbi_mixin_key(session)

    wts = int(time.time())
    params_copy = params.copy()
    params_copy['wts'] = wts

    # 按key排序拼接参数
    sorted_params = sorted(params_copy.items())
    query_string = '&'.join(f'{k}={v}' for k, v in sorted_params)

    # 拼接盐值并计算MD5
    sign_string = query_string + mixin_key
    return _md5(sign_string), wts


def create_session():
    session = requests.Session()

    pool = get_cookie_pool()
    cookie = pool.get_cookie()

    headers = DEFAULT_HEADERS.copy()
    headers['Cookie'] = cookie
    session.headers.update(headers)

    # 用于失效标记
    session._current_cookie = cookie

    session.get("https://www.bilibili.com/", timeout=10)
    return session


def _handle_cookie_error(session, code: int):
    if is_cookie_error(code) and hasattr(session, '_current_cookie') and session._current_cookie:
        pool = get_cookie_pool()
        pool.mark_invalid(session._current_cookie)


def retry_with_backoff(max_retries: int = 3, base_delay: float = 1.0, max_delay: float = 30.0):
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
                        print(f"  [重试] {func.__name__} 错误: {e}，{delay:.1f}秒后重试...")
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
        return [], "", True, error
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
def get_main_comments(oid, cursor="", session=None):
    """
    获取主评论（一级评论）- 使用WBI签名API
    cursor: 分页偏移量字符串，首页为空字符串
    返回: (comments_list, next_cursor, is_end, error_msg)
    """
    # 构建pagination_str
    if cursor:
        pagination_str = '{"offset":"%s"}' % cursor
    else:
        pagination_str = '{"offset":""}'

    pagination_str_encoded = urllib.parse.quote(pagination_str)

    # 构建签名参数
    mode = 2  # 2=最新评论, 3=热门评论
    plat = 1
    type_val = 1
    web_location = 1315875

    # 获取mixin_key并生成签名
    mixin_key = get_wbi_mixin_key(session)
    wts = int(time.time())

    # 构建签名字符串（按字母顺序排列参数）
    if cursor:
        sign_str = f"mode={mode}&oid={oid}&pagination_str={pagination_str_encoded}&plat={plat}&type={type_val}&web_location={web_location}&wts={wts}"
    else:
        sign_str = f"mode={mode}&oid={oid}&pagination_str={pagination_str_encoded}&plat={plat}&seek_rpid=&type={type_val}&web_location={web_location}&wts={wts}"

    w_rid = _md5(sign_str + mixin_key)

    # 直接构建URL（不使用params参数，避免requests重复编码）
    if cursor:
        url = f"https://api.bilibili.com/x/v2/reply/wbi/main?oid={oid}&type={type_val}&mode={mode}&pagination_str={urllib.parse.quote(pagination_str, safe=':')}&plat={plat}&web_location={web_location}&w_rid={w_rid}&wts={wts}"
    else:
        url = f"https://api.bilibili.com/x/v2/reply/wbi/main?oid={oid}&type={type_val}&mode={mode}&pagination_str={urllib.parse.quote(pagination_str, safe=':')}&plat={plat}&seek_rpid=&web_location={web_location}&w_rid={w_rid}&wts={wts}"

    try:
        if session:
            response = session.get(url, timeout=10)
        else:
            response = requests.get(url, headers=DEFAULT_HEADERS, timeout=10)

        data = response.json()

        code = data.get("code", 0)
        if code != 0:
            if session:
                _handle_cookie_error(session, code)
            return [], "", True, data.get('message', 'Unknown error')

        replies = data.get("data", {}).get("replies", []) or []

        # 获取下一页的offset
        cursor_info = data.get("data", {}).get("cursor", {})
        pagination_reply = cursor_info.get("pagination_reply", {})
        next_cursor = pagination_reply.get("next_offset", "")
        is_end = cursor_info.get("is_end", True)

        # 如果next_offset为空或不存在，说明是最后一页
        if not next_cursor:
            is_end = True

        return replies, next_cursor, is_end, None

    except Exception as e:
        return [], "", True, str(e)


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
