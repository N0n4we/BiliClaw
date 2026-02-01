from crawler import BiliCrawler

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
        resume_pending_mids=CONFIG.get("resume_pending_mids", True),
    )


if __name__ == "__main__":
    main()
