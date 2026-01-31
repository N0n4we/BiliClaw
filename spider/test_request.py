"""
测试实际请求
"""
import time
import urllib.parse
import requests
from api import create_session, get_wbi_mixin_key, _md5, DEFAULT_HEADERS

def test_request():
    print("=== 测试实际请求 ===\n")

    session = create_session()
    mixin_key = get_wbi_mixin_key(session)

    oid = 114589794770802
    mode = 2
    plat = 1
    type_val = 1
    web_location = 1315875
    wts = int(time.time())

    pagination_str = '{"offset":""}'
    pagination_str_encoded = urllib.parse.quote(pagination_str)

    # 构建签名
    sign_str = f"mode={mode}&oid={oid}&pagination_str={pagination_str_encoded}&plat={plat}&seek_rpid=&type={type_val}&web_location={web_location}&wts={wts}" + mixin_key
    w_rid = _md5(sign_str)

    # 构建URL（按GitHub爬虫的方式）
    url = f"https://api.bilibili.com/x/v2/reply/wbi/main?oid={oid}&type={type_val}&mode={mode}&pagination_str={urllib.parse.quote(pagination_str, safe=':')}&plat=1&seek_rpid=&web_location=1315875&w_rid={w_rid}&wts={wts}"

    print(f"请求URL: {url[:150]}...")
    print(f"\nw_rid: {w_rid}")
    print(f"wts: {wts}")

    # 发送请求
    print("\n发送请求...")
    resp = session.get(url, timeout=10)
    data = resp.json()

    print(f"\ncode: {data.get('code')}")
    print(f"message: {data.get('message')}")

    if data.get("code") == 0:
        replies = data.get("data", {}).get("replies", [])
        print(f"获取到 {len(replies)} 条评论")
        if replies:
            print(f"第一条评论: {replies[0].get('content', {}).get('message', '')[:50]}...")

if __name__ == "__main__":
    test_request()
