import threading
import queue
import time
import random
from api import create_session, search_videos, get_video_aid, get_video_detail, get_main_comments, get_reply_comments, get_user_card
from storage import (
    save_video, save_comment, save_account,
    get_saved_video_bvids, get_saved_comment_rpids, get_saved_account_mids,
    save_video_comment_progress, mark_video_comments_done,
    get_video_comment_progress, load_all_video_progress,
    save_pending_mid, get_pending_mids, update_pending_mids
)


class BiliCrawler:
    def __init__(self, video_dir="videos", comment_dir="comments", account_dir="accounts", delay_range=(1, 3), resume=True):
        self.video_dir = video_dir
        self.comment_dir = comment_dir
        self.account_dir = account_dir
        self.delay_range = delay_range
        self.resume = resume
        self.video_queue = queue.Queue()
        self.comment_queue = queue.Queue()
        self.user_mid_queue = queue.Queue()
        self.user_mids = set()
        self.lock = threading.Lock()
        self.stats = {
            "videos_saved": 0,
            "comments_saved": 0,
            "replies_saved": 0,
            "accounts_saved": 0,
            "videos_skipped": 0,
            "comments_skipped": 0,
            "accounts_skipped": 0,
        }
        self.saved_bvids = get_saved_video_bvids(video_dir) if resume else set()
        self.saved_rpids = get_saved_comment_rpids(comment_dir) if resume else set()
        self.saved_mids = get_saved_account_mids(account_dir) if resume else set()
        self.video_progress = load_all_video_progress() if resume else {}

        self.video_producers_done = threading.Event()
        self.comment_producers_done = threading.Event()
        self.reply_producers_done = threading.Event()
        self.active_comment_workers = 0
        self.active_reply_workers = 0

    def _delay(self):
        time.sleep(random.uniform(*self.delay_range))

    def _add_user_mid(self, mid):
        mid_str = str(mid)
        with self.lock:
            if mid_str not in self.user_mids:
                self.user_mids.add(mid_str)
                if not (self.resume and mid_str in self.saved_mids):
                    save_pending_mid(mid_str)
                    self.user_mid_queue.put(mid_str)

    def search_worker(self, keyword, pages_per_thread, thread_id, results, session):
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

    def video_detail_worker(self, thread_id, video_list, session):
        for video in video_list:
            bvid = video.get("bvid")
            detail, error = get_video_detail(bvid, session)
            if error:
                print(f"[视频线程{thread_id}] {bvid} 获取详情失败: {error}")
            else:
                detail["topic_keyword"] = self.keyword
                if save_video(detail, self.video_dir):
                    with self.lock:
                        self.stats["videos_saved"] += 1
                        self.saved_bvids.add(bvid)
                    owner_mid = detail.get("owner", {}).get("mid")
                    if owner_mid:
                        self._add_user_mid(owner_mid)
                    self.video_queue.put(detail)
                    print(f"[视频线程{thread_id}] {bvid} 已保存并推送到评论队列")
            self._delay()

    def search_videos_parallel(self, keyword, n_threads, pages_per_thread):
        print(f"搜索视频 (关键词: {keyword})")

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

        seen_bvids = set()
        unique_videos = []
        for video in results:
            bvid = video.get("bvid")
            if bvid and bvid not in seen_bvids:
                seen_bvids.add(bvid)
                unique_videos.append(video)

        if self.resume and self.saved_bvids:
            before_count = len(unique_videos)
            new_videos = []
            for v in unique_videos:
                bvid = v.get("bvid")
                if bvid in self.saved_bvids:
                    self.video_queue.put(v)
                else:
                    new_videos.append(v)
            unique_videos = new_videos
            skipped = before_count - len(unique_videos)
            if skipped > 0:
                self.stats["videos_skipped"] = skipped

        print(f"共 {len(unique_videos)} 个新视频")

        if not unique_videos:
            print("没有新视频需要获取详情")
            self.video_producers_done.set()
            return 0

        chunk_size = (len(unique_videos) + n_threads - 1) // n_threads
        video_chunks = [unique_videos[i:i + chunk_size] for i in range(0, len(unique_videos), chunk_size)]

        detail_threads = []
        detail_sessions = [create_session() for _ in range(len(video_chunks))]

        for i, chunk in enumerate(video_chunks):
            t = threading.Thread(target=self.video_detail_worker, args=(i, chunk, detail_sessions[i]))
            detail_threads.append(t)
            t.start()

        for t in detail_threads:
            t.join()

        self.video_producers_done.set()

    def comment_worker(self, thread_id, session):
        with self.lock:
            self.active_comment_workers += 1

        while True:
            try:
                video = self.video_queue.get(timeout=2)
            except queue.Empty:
                if self.video_producers_done.is_set():
                    break
                continue

            bvid = video.get("bvid")
            aid = video.get("aid")

            progress = get_video_comment_progress(bvid)
            if self.resume and progress["done"]:
                print(f"[评论线程{thread_id}] {bvid} 评论已爬完，跳过")
                continue

            if not aid:
                if progress["aid"]:
                    aid = progress["aid"]
                else:
                    aid, error = get_video_aid(bvid, session)
                    if error:
                        print(f"[评论线程{thread_id}] 获取 {bvid} 的aid失败: {error}")
                        continue
                    self._delay()

            cursor = progress["cursor"] if self.resume else ""
            if cursor:
                print(f"[评论线程{thread_id}] {bvid} (aid={aid}) 从游标 {cursor[:20]}... 恢复爬取...")
            else:
                print(f"[评论线程{thread_id}] {bvid} (aid={aid}) 开始爬取评论...")

            comment_count = 0
            while True:
                replies, next_cursor, is_end, error = get_main_comments(aid, cursor, session)
                if error:
                    print(f"[评论线程{thread_id}] {bvid} 评论获取错误: {error}")
                    save_video_comment_progress(bvid, cursor, aid)
                    break

                for reply in replies:
                    rpid = str(reply.get("rpid", ""))
                    comment_mid = reply.get("mid")
                    if comment_mid:
                        self._add_user_mid(comment_mid)
                    if self.resume and rpid in self.saved_rpids:
                        with self.lock:
                            self.stats["comments_skipped"] += 1
                        if reply.get("rcount", 0) > 0:
                            self.comment_queue.put((aid, reply))
                        continue
                    if save_comment(reply, self.comment_dir):
                        with self.lock:
                            self.stats["comments_saved"] += 1
                            self.saved_rpids.add(rpid)
                        comment_count += 1
                        if reply.get("rcount", 0) > 0:
                            self.comment_queue.put((aid, reply))

                if is_end or not replies:
                    mark_video_comments_done(bvid)
                    break

                cursor = next_cursor
                save_video_comment_progress(bvid, cursor, aid)
                self._delay()

            print(f"[评论线程{thread_id}] {bvid} 爬取完成，共 {comment_count} 条一级评论")

        with self.lock:
            self.active_comment_workers -= 1
            if self.active_comment_workers == 0:
                self.comment_producers_done.set()

    def reply_worker(self, thread_id, session):
        with self.lock:
            self.active_reply_workers += 1

        while True:
            try:
                aid, parent_comment = self.comment_queue.get(timeout=2)
            except queue.Empty:
                if self.comment_producers_done.is_set():
                    break
                continue

            rpid = parent_comment.get("rpid")
            rcount = parent_comment.get("rcount", 0)
            print(f"[回复线程{thread_id}] 开始爬取评论 {rpid} 的 {rcount} 条回复...")

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
                    reply_mid = reply.get("mid")
                    if reply_mid:
                        self._add_user_mid(reply_mid)
                    if self.resume and reply_rpid in self.saved_rpids:
                        total_fetched += 1
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

        with self.lock:
            self.active_reply_workers -= 1
            if self.active_reply_workers == 0:
                self.reply_producers_done.set()

    def account_worker(self, thread_id, session):
        while True:
            try:
                mid = self.user_mid_queue.get(timeout=2)
            except queue.Empty:
                if self.reply_producers_done.is_set():
                    break
                continue

            if self.resume and mid in self.saved_mids:
                with self.lock:
                    self.stats["accounts_skipped"] += 1
                continue

            user_data, error = get_user_card(mid, session)
            if error:
                print(f"[用户线程{thread_id}] 获取用户 {mid} 信息失败: {error}")
            else:
                if save_account(user_data, self.account_dir):
                    with self.lock:
                        self.stats["accounts_saved"] += 1
                        self.saved_mids.add(mid)
            self._delay()

    def start_comment_workers(self, n_threads):
        threads = []
        sessions = [create_session() for _ in range(n_threads)]
        for i in range(n_threads):
            t = threading.Thread(target=self.comment_worker, args=(i, sessions[i]))
            threads.append(t)
            t.start()
        return threads

    def start_reply_workers(self, n_threads):
        threads = []
        sessions = [create_session() for _ in range(n_threads)]
        for i in range(n_threads):
            t = threading.Thread(target=self.reply_worker, args=(i, sessions[i]))
            threads.append(t)
            t.start()
        return threads

    def start_account_workers(self, n_threads):
        threads = []
        sessions = [create_session() for _ in range(n_threads)]
        for i in range(n_threads):
            t = threading.Thread(target=self.account_worker, args=(i, sessions[i]))
            threads.append(t)
            t.start()
        return threads

    def run(self, keyword, n_threads=3, pages_per_thread=2, resume_pending_mids=True):
        self.keyword = keyword
        print(f"关键词: {keyword}")
        print(f"线程数: {n_threads}")
        print(f"预计搜索视频数: ~{n_threads * pages_per_thread * 50}")
        print(f"断点续传: {'启用' if self.resume else '禁用'}")
        if self.resume and self.video_progress:
            done_count = sum(1 for p in self.video_progress.values() if p.get("done"))
            in_progress_count = sum(1 for p in self.video_progress.values() if not p.get("done") and p.get("cursor", ""))
            print(f"  - 已完成评论爬取的视频: {done_count}")
            print(f"  - 评论爬取中断的视频: {in_progress_count}")

        if self.resume and resume_pending_mids:
            pending_mids = get_pending_mids()
            restored_count = 0
            for mid in pending_mids:
                if mid not in self.saved_mids:
                    self.user_mids.add(mid)
                    self.user_mid_queue.put(mid)
                    restored_count += 1
            if restored_count > 0:
                print(f"  - 已恢复 {restored_count} 个待爬取的用户mid")

        comment_threads = self.start_comment_workers(n_threads)
        reply_threads = self.start_reply_workers(n_threads)
        account_threads = self.start_account_workers(n_threads)

        self.search_videos_parallel(keyword, n_threads, pages_per_thread)

        for t in comment_threads:
            t.join()
        print(f"一级评论爬取完成，共保存 {self.stats['comments_saved']} 条")

        for t in reply_threads:
            t.join()
        print(f"二级评论爬取完成，共保存 {self.stats['replies_saved']} 条")

        for t in account_threads:
            t.join()
        print(f"用户信息爬取完成，共保存 {self.stats['accounts_saved']} 个")

        print(f"保存视频数: {self.stats['videos_saved']}")
        if self.stats.get('videos_skipped', 0) > 0:
            print(f"跳过视频数（已存在）: {self.stats['videos_skipped']}")
        print(f"保存一级评论数: {self.stats['comments_saved']}")
        if self.stats.get('comments_skipped', 0) > 0:
            print(f"跳过评论数（已存在）: {self.stats['comments_skipped']}")
        print(f"保存二级评论数: {self.stats['replies_saved']}")
        print(f"总评论数: {self.stats['comments_saved'] + self.stats['replies_saved']}")
        print(f"保存用户数: {self.stats['accounts_saved']}")
        if self.stats.get('accounts_skipped', 0) > 0:
            print(f"跳过用户数（已存在）: {self.stats['accounts_skipped']}")

        # 清理pending_mids：只保留未完成的mid
        remaining_mids = self.user_mids - self.saved_mids
        update_pending_mids(remaining_mids)
        if remaining_mids:
            print(f"剩余未爬取用户数: {len(remaining_mids)}")
        else:
            print("所有用户信息已爬取完成，pending_mids已清理")
