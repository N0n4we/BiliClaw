from crawler import BiliCrawler
from rate_limiter import init_rate_limiter
from api import set_user_agent

CONFIG = {
    "keyword": "电棍otto说的道理",  # 搜索关键词
    "n_threads": 3,                 # 线程数 (n)
    "pages_per_thread": 2,          # 每线程搜索页数 (m)，总共搜索 n*m 页
    "video_dir": "videos",
    "comment_dir": "comments",
    "account_dir": "accounts",
    "delay_range": (2, 4),
    "resume": True,                 # 视频评论断点续传
    "resume_pending_mids": True,    # 用户信息断点续传
    "rate_limit_rate": 2.0,         # 令牌桶速率 (每秒生成的令牌数)
    "rate_limit_capacity": 5.0,     # 令牌桶容量
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0",
}


def main():
    init_rate_limiter(
        rate=CONFIG["rate_limit_rate"],
        capacity=CONFIG["rate_limit_capacity"],
    )

    set_user_agent(CONFIG["user_agent"])

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
        resume_pending_mids=CONFIG.get("resume_pending_mids", True),
    )


if __name__ == "__main__":
    main()
