"""
存储模块 - 向Kafka发送消息
"""
import os
import json
import threading
from kafka import KafkaProducer

KAFKA_BOOTSTRAP_SERVERS = os.environ.get("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
KAFKA_TOPIC_VIDEO = "claw_video"
KAFKA_TOPIC_COMMENT = "claw_comment"
KAFKA_TOPIC_ACCOUNT = "claw_account"

RECORD_DIR = "sent_records"
PROGRESS_FILE = "video_comment_progress.json"

_progress_lock = threading.Lock()
_producer_lock = threading.Lock()
_producer = None


def get_producer():
    global _producer
    if _producer is None:
        with _producer_lock:
            if _producer is None:
                _producer = KafkaProducer(
                    bootstrap_servers=KAFKA_BOOTSTRAP_SERVERS,
                    value_serializer=lambda v: json.dumps(v, ensure_ascii=False).encode("utf-8"),
                    key_serializer=lambda k: k.encode("utf-8") if k else None,
                )
    return _producer


def ensure_dir(dir_path):
    os.makedirs(dir_path, exist_ok=True)


def _record_sent_id(record_file, id_value):
    ensure_dir(RECORD_DIR)
    filepath = os.path.join(RECORD_DIR, record_file)
    with open(filepath, "a", encoding="utf-8") as f:
        f.write(f"{id_value}\n")


def _load_sent_ids(record_file):
    filepath = os.path.join(RECORD_DIR, record_file)
    if not os.path.exists(filepath):
        return set()
    ids = set()
    with open(filepath, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                ids.add(line)
    return ids


def save_video(video, video_dir=None):
    bvid = video.get("bvid")
    if not bvid:
        return False
    producer = get_producer()
    producer.send(KAFKA_TOPIC_VIDEO, key=bvid, value=video)
    _record_sent_id("sent_videos.txt", bvid)
    return True


def save_comment(comment, comment_dir=None):
    rpid = comment.get("rpid")
    if not rpid:
        return False
    rpid_str = str(rpid)
    producer = get_producer()
    producer.send(KAFKA_TOPIC_COMMENT, key=rpid_str, value=comment)
    _record_sent_id("sent_comments.txt", rpid_str)
    return True


def save_account(account, account_dir=None):
    mid = account.get("card", {}).get("mid")
    if not mid:
        return False
    mid_str = str(mid)
    producer = get_producer()
    producer.send(KAFKA_TOPIC_ACCOUNT, key=mid_str, value=account)
    _record_sent_id("sent_accounts.txt", mid_str)
    return True


def get_saved_video_bvids(video_dir=None):
    return _load_sent_ids("sent_videos.txt")


def get_saved_comment_rpids(comment_dir=None):
    return _load_sent_ids("sent_comments.txt")


def get_saved_account_mids(account_dir=None):
    return _load_sent_ids("sent_accounts.txt")


def save_pending_mid(mid):
    _record_sent_id("pending_mids.txt", str(mid))


def get_pending_mids():
    return _load_sent_ids("pending_mids.txt")


def update_pending_mids(remaining_mids):
    filepath = os.path.join(RECORD_DIR, "pending_mids.txt")
    if not remaining_mids:
        if os.path.exists(filepath):
            os.remove(filepath)
        return
    ensure_dir(RECORD_DIR)
    with open(filepath, "w", encoding="utf-8") as f:
        for mid in remaining_mids:
            f.write(f"{mid}\n")


def flush_producer():
    global _producer
    if _producer is not None:
        _producer.flush()


def close_producer():
    global _producer
    if _producer is not None:
        _producer.flush()
        _producer.close()
        _producer = None


def _get_progress_filepath():
    ensure_dir(RECORD_DIR)
    return os.path.join(RECORD_DIR, PROGRESS_FILE)


def _load_progress_data():
    filepath = _get_progress_filepath()
    if not os.path.exists(filepath):
        return {}
    try:
        with open(filepath, "r", encoding="utf-8") as f:
            return json.load(f)
    except (json.JSONDecodeError, IOError):
        return {}


def _save_progress_data(data):
    filepath = _get_progress_filepath()
    with open(filepath, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def save_video_comment_progress(bvid, cursor, aid=None):
    with _progress_lock:
        data = _load_progress_data()
        if bvid not in data:
            data[bvid] = {"done": False, "cursor": ""}
        data[bvid]["cursor"] = cursor
        if aid is not None:
            data[bvid]["aid"] = aid
        _save_progress_data(data)


def mark_video_comments_done(bvid):
    with _progress_lock:
        data = _load_progress_data()
        if bvid not in data:
            data[bvid] = {}
        data[bvid]["done"] = True
        data[bvid]["cursor"] = ""
        _save_progress_data(data)


def get_video_comment_progress(bvid):
    with _progress_lock:
        data = _load_progress_data()
        if bvid in data:
            return {
                "done": data[bvid].get("done", False),
                "cursor": data[bvid].get("cursor", ""),
                "aid": data[bvid].get("aid")
            }
        return {"done": False, "cursor": "", "aid": None}


def is_video_comments_done(bvid):
    progress = get_video_comment_progress(bvid)
    return progress["done"]


def load_all_video_progress():
    with _progress_lock:
        return _load_progress_data()
