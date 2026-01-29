"""
爬虫主模块 - 实现多线程爬取逻辑
"""
import threading
import queue
import time
import random
from api import create_session, search_videos, get_video_aid, get_video_detail, get_main_comments, get_reply_comments
from storage import save_video, save_comment, get_saved_video_bvids, get_saved_comment_rpids


class BiliCrawler:
    def __init__(self, video_dir="videos", comment_dir="comments", delay_range=(1, 3), resume=True):
        self.video_dir = video_dir
        self.comment_dir = comment_dir
        self.delay_range = delay_range
        self.resume = resume  # 是否启用断点续传
        self.video_queue = queue.Queue()
        self.comment_queue = queue.Queue()
        self.lock = threading.Lock()
        self.stats = {
            "videos_saved": 0,
            "comments_saved": 0,
            "replies_saved": 0,
            "videos_skipped": 0,
            "comments_skipped": 0,
        }
        # 加载已保存的数据（用于断点续传）
        self.saved_bvids = get_saved_video_bvids(video_dir) if resume else set()
        self.saved_rpids = get_saved_comment_rpids(comment_dir) if resume else set()

    def _delay(self):
        """随机延迟"""
        time.sleep(random.uniform(*self.delay_range))

    # ========== 阶段1: 搜索视频 ==========
    def search_worker(self, keyword, pages_per_thread, thread_id, results, session):
        """单个搜索线程的工作函数"""
        thread_videos = []
        for page in range(1, pages_per_thread + 1):
            actual_page = thread_id * pages_per_thread + page
            print(f"[搜索线程{thread_id}] 正在获取第 {actual_page} 页...")
            videos, _, error = search_videos(keyword, page=actual_page, session=session)
            if error:
                print(f"[搜索线程{thread_id}] 第 {actual_page} 页错误: {error}")
            else:
                thread_videos.extend(videos)
                print(f"[搜索线程{thread_id}] 第 {actual_page} 页获取 {len(videos)} 条视频")
            self._delay()
        with self.lock:
            results.extend(thread_videos)

    def search_videos_parallel(self, keyword, n_threads, pages_per_thread):
        """
        多线程搜索视频
        n_threads: 线程数
        pages_per_thread: 每个线程爬取的页数 (即m)
        总共爬取 n_threads * pages_per_thread 页，每页50条，约 n*m*50 条视频
        """
        print(f"\n{'='*50}")
        print(f"阶段1: 搜索视频 (关键词: {keyword})")
        print(f"线程数: {n_threads}, 每线程页数: {pages_per_thread}")
        print(f"{'='*50}")

        results = []
        threads = []
        sessions = [create_session() for _ in range(n_threads)]

        for i in range(n_threads):
            t = threading.Thread(
                target=self.search_worker,
                args=(keyword, pages_per_thread, i, results, sessions[i])
            )
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        # 去重
        seen_bvids = set()
        unique_videos = []
        for video in results:
            bvid = video.get("bvid")
            if bvid and bvid not in seen_bvids:
                seen_bvids.add(bvid)
                unique_videos.append(video)

        print(f"\n搜索完成: 总计 {len(results)} 条, 去重后 {len(unique_videos)} 条")

        # 断点续传：过滤已保存的视频
        if self.resume and self.saved_bvids:
            before_count = len(unique_videos)
            unique_videos = [v for v in unique_videos if v.get("bvid") not in self.saved_bvids]
            skipped = before_count - len(unique_videos)
            if skipped > 0:
                self.stats["videos_skipped"] = skipped
                print(f"断点续传: 跳过 {skipped} 个已保存的视频，剩余 {len(unique_videos)} 个待处理")

        # 使用详情接口获取完整视频信息并保存
        print(f"\n正在获取视频详细信息（使用view接口）...")
        detailed_videos = []
        session = create_session()
        for i, video in enumerate(unique_videos):
            bvid = video.get("bvid")
            detail, error = get_video_detail(bvid, session)
            if error:
                print(f"  [{i+1}/{len(unique_videos)}] {bvid} 获取详情失败: {error}")
            else:
                if save_video(detail, self.video_dir):
                    self.stats["videos_saved"] += 1
                    self.saved_bvids.add(bvid)  # 更新已保存集合
                detailed_videos.append(detail)
                if (i + 1) % 10 == 0:
                    print(f"  已处理 {i+1}/{len(unique_videos)} 个视频")
            self._delay()

        print(f"已保存 {self.stats['videos_saved']} 个视频到 {self.video_dir}/")
        return detailed_videos

    # ========== 阶段2: 爬取一级评论 ==========
    def comment_worker(self, thread_id, session):
        """评论爬取线程的工作函数"""
        while True:
            try:
                video = self.video_queue.get(timeout=3)
            except queue.Empty:
                break

            bvid = video.get("bvid")
            aid = video.get("aid")

            # 如果没有aid，需要获取
            if not aid:
                aid, error = get_video_aid(bvid, session)
                if error:
                    print(f"[评论线程{thread_id}] 获取 {bvid} 的aid失败: {error}")
                    self.video_queue.task_done()
                    continue
                self._delay()

            print(f"[评论线程{thread_id}] 开始爬取 {bvid} (aid={aid}) 的评论...")

            # 爬取所有一级评论
            cursor = 0
            comment_count = 0
            while True:
                replies, next_cursor, is_end, error = get_main_comments(aid, cursor, session)
                if error:
                    print(f"[评论线程{thread_id}] {bvid} 评论获取错误: {error}")
                    break

                for reply in replies:
                    rpid = str(reply.get("rpid", ""))
                    # 断点续传：跳过已保存的评论
                    if self.resume and rpid in self.saved_rpids:
                        with self.lock:
                            self.stats["comments_skipped"] += 1
                        # 仍然检查是否需要爬取二级评论
                        if reply.get("rcount", 0) > 0:
                            self.comment_queue.put((aid, reply))
                        continue
                    if save_comment(reply, self.comment_dir):
                        with self.lock:
                            self.stats["comments_saved"] += 1
                            self.saved_rpids.add(rpid)  # 更新已保存集合
                        comment_count += 1
                        # 如果有回复，加入二级评论队列
                        if reply.get("rcount", 0) > 0:
                            self.comment_queue.put((aid, reply))

                if is_end or not replies:
                    break
                cursor = next_cursor
                self._delay()

            print(f"[评论线程{thread_id}] {bvid} 爬取完成，共 {comment_count} 条一级评论")
            self.video_queue.task_done()

    def crawl_comments_parallel(self, videos, n_threads):
        """多线程爬取一级评论"""
        print(f"\n{'='*50}")
        print(f"阶段2: 爬取一级评论")
        print(f"视频数: {len(videos)}, 线程数: {n_threads}")
        print(f"{'='*50}")

        # 将视频加入队列
        for video in videos:
            self.video_queue.put(video)

        threads = []
        sessions = [create_session() for _ in range(n_threads)]

        for i in range(n_threads):
            t = threading.Thread(target=self.comment_worker, args=(i, sessions[i]))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        print(f"\n一级评论爬取完成，共保存 {self.stats['comments_saved']} 条评论")

    # ========== 阶段3: 爬取二级评论 ==========
    def reply_worker(self, thread_id, session):
        """二级评论爬取线程的工作函数"""
        while True:
            try:
                aid, parent_comment = self.comment_queue.get(timeout=3)
            except queue.Empty:
                break

            rpid = parent_comment.get("rpid")
            rcount = parent_comment.get("rcount", 0)
            print(f"[回复线程{thread_id}] 开始爬取评论 {rpid} 的 {rcount} 条回复...")

            # 爬取所有二级评论
            page = 1
            total_fetched = 0
            while True:
                replies, total_count, error = get_reply_comments(aid, rpid, page, session=session)
                if error:
                    print(f"[回复线程{thread_id}] 评论 {rpid} 回复获取错误: {error}")
                    break

                if not replies:
                    break

                for reply in replies:
                    reply_rpid = str(reply.get("rpid", ""))
                    # 断点续传：跳过已保存的评论
                    if self.resume and reply_rpid in self.saved_rpids:
                        continue
                    if save_comment(reply, self.comment_dir):
                        with self.lock:
                            self.stats["replies_saved"] += 1
                            self.saved_rpids.add(reply_rpid)
                        total_fetched += 1

                if total_fetched >= total_count:
                    break
                page += 1
                self._delay()

            print(f"[回复线程{thread_id}] 评论 {rpid} 爬取完成，共 {total_fetched} 条回复")
            self.comment_queue.task_done()

    def crawl_replies_parallel(self, n_threads):
        """多线程爬取二级评论"""
        queue_size = self.comment_queue.qsize()
        print(f"\n{'='*50}")
        print(f"阶段3: 爬取二级评论")
        print(f"待处理一级评论数: {queue_size}, 线程数: {n_threads}")
        print(f"{'='*50}")

        if queue_size == 0:
            print("没有需要爬取回复的评论")
            return

        threads = []
        sessions = [create_session() for _ in range(n_threads)]

        for i in range(n_threads):
            t = threading.Thread(target=self.reply_worker, args=(i, sessions[i]))
            threads.append(t)
            t.start()

        for t in threads:
            t.join()

        print(f"\n二级评论爬取完成，共保存 {self.stats['replies_saved']} 条回复")

    # ========== 主流程 ==========
    def run(self, keyword, n_threads=3, pages_per_thread=2):
        """
        运行完整爬虫流程
        keyword: 搜索关键词
        n_threads: 线程数 (n)
        pages_per_thread: 每线程搜索页数 (m)
        """
        print("\n" + "=" * 60)
        print("BiliClaw 爬虫启动")
        print("=" * 60)
        print(f"关键词: {keyword}")
        print(f"线程数: {n_threads}")
        print(f"每线程页数: {pages_per_thread}")
        print(f"预计搜索视频数: ~{n_threads * pages_per_thread * 50}")
        print(f"视频保存目录: {self.video_dir}/")
        print(f"评论保存目录: {self.comment_dir}/")

        # 阶段1: 搜索视频
        videos = self.search_videos_parallel(keyword, n_threads, pages_per_thread)

        if not videos:
            print("\n没有搜索到视频，爬虫结束")
            return

        # 阶段2: 爬取一级评论
        self.crawl_comments_parallel(videos, n_threads)

        # 阶段3: 爬取二级评论
        self.crawl_replies_parallel(n_threads)

        # 统计
        print("\n" + "=" * 60)
        print("爬虫完成!")
        print("=" * 60)
        print(f"保存视频数: {self.stats['videos_saved']}")
        if self.stats.get('videos_skipped', 0) > 0:
            print(f"跳过视频数（已存在）: {self.stats['videos_skipped']}")
        print(f"保存一级评论数: {self.stats['comments_saved']}")
        if self.stats.get('comments_skipped', 0) > 0:
            print(f"跳过评论数（已存在）: {self.stats['comments_skipped']}")
        print(f"保存二级评论数: {self.stats['replies_saved']}")
        print(f"总评论数: {self.stats['comments_saved'] + self.stats['replies_saved']}")
