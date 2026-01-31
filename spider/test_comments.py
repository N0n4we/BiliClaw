"""
测试评论获取功能
"""
from api import create_session, get_main_comments, get_video_aid

# 测试视频BV号（选一个评论多的视频）
TEST_BVID = "BV1GJ411x7h7"  # 热门视频，评论较多

def test_get_comments():
    print("创建session...")
    session = create_session()

    print(f"\n获取视频 {TEST_BVID} 的aid...")
    aid, error = get_video_aid(TEST_BVID, session)
    if error:
        print(f"获取aid失败: {error}")
        return
    print(f"aid: {aid}")

    print("\n开始获取评论...")
    cursor = ""
    total_comments = 0
    page = 0

    while page < 5:  # 获取5页
        page += 1
        print(f"\n--- 第 {page} 页 ---")

        replies, next_cursor, is_end, error = get_main_comments(aid, cursor, session)

        if error:
            print(f"获取评论失败: {error}")
            break

        print(f"获取到 {len(replies)} 条评论")
        total_comments += len(replies)

        # 打印前3条评论的部分内容
        for i, reply in enumerate(replies[:3]):
            uname = reply.get("member", {}).get("uname", "未知")
            message = reply.get("content", {}).get("message", "")[:50]
            print(f"  [{i+1}] {uname}: {message}...")

        if is_end:
            print("\n已到达最后一页")
            break

        cursor = next_cursor
        print(f"下一页cursor: {cursor[:30]}..." if cursor else "无下一页")

    print(f"\n========== 测试完成 ==========")
    print(f"共获取 {total_comments} 条评论")

if __name__ == "__main__":
    test_get_comments()
