"""
检查API返回的完整数据结构
"""
import time
import urllib.parse
import json
from api import create_session, get_wbi_mixin_key, _md5, DEFAULT_HEADERS

def check_response():
    print("=== 检查API返回数据 ===\n")

    session = create_session()
    mixin_key = get_wbi_mixin_key(session)

    oid = 80433022  # BV1GJ411x7h7
    mode = 2
    plat = 1
    type_val = 1
    web_location = 1315875
    wts = int(time.time())

    pagination_str = '{"offset":""}'
    pagination_str_encoded = urllib.parse.quote(pagination_str)

    sign_str = f"mode={mode}&oid={oid}&pagination_str={pagination_str_encoded}&plat={plat}&seek_rpid=&type={type_val}&web_location={web_location}&wts={wts}"
    w_rid = _md5(sign_str + mixin_key)

    url = f"https://api.bilibili.com/x/v2/reply/wbi/main?oid={oid}&type={type_val}&mode={mode}&pagination_str={urllib.parse.quote(pagination_str, safe=':')}&plat={plat}&seek_rpid=&web_location={web_location}&w_rid={w_rid}&wts={wts}"

    resp = session.get(url, timeout=10)
    data = resp.json()

    print(f"code: {data.get('code')}")
    print(f"message: {data.get('message')}")

    if data.get("code") == 0:
        # 检查cursor信息
        cursor = data.get("data", {}).get("cursor", {})
        print(f"\n=== cursor信息 ===")
        print(json.dumps(cursor, indent=2, ensure_ascii=False))

        # 检查replies数量
        replies = data.get("data", {}).get("replies", [])
        print(f"\n=== replies数量: {len(replies)} ===")

        # 检查top_replies（置顶评论）
        top_replies = data.get("data", {}).get("top_replies", [])
        print(f"top_replies数量: {len(top_replies) if top_replies else 0}")

if __name__ == "__main__":
    check_response()
