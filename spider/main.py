"""
BiliClaw 主入口文件
B站视频和评论爬虫

使用方法:
    python main.py

配置参数在下方 CONFIG 中修改
"""
from crawler import BiliCrawler

# ========== 配置参数 ==========
CONFIG = {
    "keyword": "妖精管理局",      # 搜索关键词
    "n_threads": 3,              # 线程数 (n)
    "pages_per_thread": 2,       # 每线程搜索页数 (m)，总共搜索 n*m 页
    "video_dir": "videos",       # 视频保存目录
    "comment_dir": "comments",   # 评论保存目录
    "account_dir": "accounts",   # 用户信息保存目录
    "delay_range": (2, 4),       # 请求间隔范围（秒），建议不低于2秒避免风控
    "resume": True,              # 是否启用断点续传（跳过已保存的数据）
}


def main():
    crawler = BiliCrawler(
        video_dir=CONFIG["video_dir"],
        comment_dir=CONFIG["comment_dir"],
        account_dir=CONFIG["account_dir"],
        delay_range=CONFIG["delay_range"],
        resume=CONFIG["resume"],
    )

    crawler.run(
        keyword=CONFIG["keyword"],
        n_threads=CONFIG["n_threads"],
        pages_per_thread=CONFIG["pages_per_thread"],
    )


if __name__ == "__main__":
    main()
