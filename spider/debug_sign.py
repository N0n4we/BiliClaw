"""
调试WBI签名过程
"""
import time
import urllib.parse
import hashlib
from api import create_session, get_wbi_mixin_key, _md5

def debug_sign():
    print("=== 调试WBI签名 ===\n")

    session = create_session()

    # 获取mixin_key
    mixin_key = get_wbi_mixin_key(session)
    print(f"mixin_key: {mixin_key}")

    # 测试参数
    oid = 114589794770802
    mode = 2
    plat = 1
    type_val = 1
    web_location = 1315875
    wts = int(time.time())

    # 首页的pagination_str
    pagination_str = '{"offset":""}'
    pagination_str_encoded = urllib.parse.quote(pagination_str)

    print(f"\npagination_str: {pagination_str}")
    print(f"pagination_str_encoded: {pagination_str_encoded}")

    # 按GitHub爬虫的方式构建签名字符串（首页）
    github_sign_str = f"mode={mode}&oid={oid}&pagination_str={pagination_str_encoded}&plat={plat}&seek_rpid=&type={type_val}&web_location={web_location}&wts={wts}" + mixin_key
    github_w_rid = _md5(github_sign_str)

    print(f"\n=== GitHub方式 ===")
    print(f"签名字符串: {github_sign_str[:100]}...")
    print(f"w_rid: {github_w_rid}")

    # 按我的方式构建签名字符串
    my_params = {
        "mode": mode,
        "oid": oid,
        "pagination_str": pagination_str_encoded,
        "plat": plat,
        "seek_rpid": "",
        "type": type_val,
        "web_location": web_location,
        "wts": wts,
    }
    sorted_params = sorted(my_params.items())
    my_query_string = '&'.join(f'{k}={v}' for k, v in sorted_params)
    my_sign_str = my_query_string + mixin_key
    my_w_rid = _md5(my_sign_str)

    print(f"\n=== 我的方式 ===")
    print(f"排序后参数: {sorted_params}")
    print(f"签名字符串: {my_sign_str[:100]}...")
    print(f"w_rid: {my_w_rid}")

    print(f"\n=== 对比 ===")
    print(f"签名是否一致: {github_w_rid == my_w_rid}")

if __name__ == "__main__":
    debug_sign()
