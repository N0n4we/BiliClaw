"""
存储模块 - 向Kafka发送消息
"""
import os
import json
from kafka import KafkaProducer

# Kafka配置
KAFKA_BOOTSTRAP_SERVERS = os.environ.get("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
KAFKA_TOPIC_VIDEO = "claw_video"
KAFKA_TOPIC_COMMENT = "claw_comment"
KAFKA_TOPIC_ACCOUNT = "claw_account"

# 本地记录目录（用于断点续传）
RECORD_DIR = "sent_records"

# 全局Kafka生产者（延迟初始化）
_producer = None


def get_producer():
    """获取Kafka生产者（单例模式）"""
    global _producer
    if _producer is None:
        _producer = KafkaProducer(
            bootstrap_servers=KAFKA_BOOTSTRAP_SERVERS,
            value_serializer=lambda v: json.dumps(v, ensure_ascii=False).encode("utf-8"),
            key_serializer=lambda k: k.encode("utf-8") if k else None,
        )
    return _producer


def ensure_dir(dir_path):
    """确保目录存在"""
    if not os.path.exists(dir_path):
        os.makedirs(dir_path)


def _record_sent_id(record_file, id_value):
    """记录已发送的ID到本地文件"""
    ensure_dir(RECORD_DIR)
    filepath = os.path.join(RECORD_DIR, record_file)
    with open(filepath, "a", encoding="utf-8") as f:
        f.write(f"{id_value}\n")


def _load_sent_ids(record_file):
    """加载已发送的ID列表"""
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
    """发送视频数据到Kafka"""
    bvid = video.get("bvid")
    if not bvid:
        return False
    producer = get_producer()
    producer.send(KAFKA_TOPIC_VIDEO, key=bvid, value=video)
    _record_sent_id("sent_videos.txt", bvid)
    return True


def save_comment(comment, comment_dir=None):
    """发送评论数据到Kafka"""
    rpid = comment.get("rpid")
    if not rpid:
        return False
    rpid_str = str(rpid)
    producer = get_producer()
    producer.send(KAFKA_TOPIC_COMMENT, key=rpid_str, value=comment)
    _record_sent_id("sent_comments.txt", rpid_str)
    return True


def save_account(account, account_dir=None):
    """发送用户数据到Kafka"""
    mid = account.get("card", {}).get("mid")
    if not mid:
        return False
    mid_str = str(mid)
    producer = get_producer()
    producer.send(KAFKA_TOPIC_ACCOUNT, key=mid_str, value=account)
    _record_sent_id("sent_accounts.txt", mid_str)
    return True


def get_saved_video_bvids(video_dir=None):
    """获取已发送的视频bvid列表"""
    return _load_sent_ids("sent_videos.txt")


def get_saved_comment_rpids(comment_dir=None):
    """获取已发送的评论rpid列表"""
    return _load_sent_ids("sent_comments.txt")


def get_saved_account_mids(account_dir=None):
    """获取已发送的用户mid列表"""
    return _load_sent_ids("sent_accounts.txt")


def flush_producer():
    """刷新Kafka生产者，确保所有消息发送完成"""
    global _producer
    if _producer is not None:
        _producer.flush()


def close_producer():
    """关闭Kafka生产者"""
    global _producer
    if _producer is not None:
        _producer.flush()
        _producer.close()
        _producer = None
