"""
调试WBI签名
"""
import requests
from api import DEFAULT_HEADERS, create_session

def test_nav():
    print("=== 测试1: 不带Cookie ===")
    url = "https://api.bilibili.com/x/web-interface/nav"
    try:
        resp = requests.get(url, headers=DEFAULT_HEADERS, timeout=10)
        data = resp.json()
        print(f"code: {data.get('code')}")
        print(f"message: {data.get('message')}")
        if data.get("code") == 0:
            wbi_img = data["data"].get("wbi_img")
            print(f"wbi_img: {wbi_img}")
        else:
            # 即使未登录，也可能返回wbi_img
            wbi_img = data.get("data", {}).get("wbi_img")
            print(f"wbi_img (未登录): {wbi_img}")
    except Exception as e:
        print(f"错误: {e}")

    print("\n=== 测试2: 带Cookie (session) ===")
    try:
        session = create_session()
        resp = session.get(url, timeout=10)
        data = resp.json()
        print(f"code: {data.get('code')}")
        print(f"message: {data.get('message')}")
        if data.get("code") == 0:
            wbi_img = data["data"].get("wbi_img")
            print(f"wbi_img: {wbi_img}")
    except Exception as e:
        print(f"错误: {e}")

if __name__ == "__main__":
    test_nav()
