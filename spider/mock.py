"""
Mock模块 - 从本地文件夹读取数据并发送到Kafka
模拟main.py的行为，用于测试数据管道

使用方法:
    python mock.py
"""
import os
import json
import argparse
from kafka import KafkaProducer

keyword = 'test'

# Kafka配置
KAFKA_BOOTSTRAP_SERVERS = os.environ.get("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
KAFKA_TOPIC_VIDEO = "claw_video"
KAFKA_TOPIC_COMMENT = "claw_comment"
KAFKA_TOPIC_ACCOUNT = "claw_account"

# 默认数据目录
DEFAULT_VIDEO_DIR = "videos"
DEFAULT_COMMENT_DIR = "comments"
DEFAULT_ACCOUNT_DIR = "accounts"


def create_producer():
    """创建Kafka生产者"""
    return KafkaProducer(
        bootstrap_servers=KAFKA_BOOTSTRAP_SERVERS,
        value_serializer=lambda v: json.dumps(v, ensure_ascii=False).encode("utf-8"),
        key_serializer=lambda k: k.encode("utf-8") if k else None,
    )


def load_json_files(directory):
    """加载目录下所有JSON文件"""
    files = []
    if not os.path.exists(directory):
        print(f"目录不存在: {directory}")
        return files
    for filename in os.listdir(directory):
        if filename.endswith(".json"):
            filepath = os.path.join(directory, filename)
            try:
                with open(filepath, "r", encoding="utf-8") as f:
                    data = json.load(f)
                    files.append((filename, data))
            except Exception as e:
                print(f"读取文件失败 {filepath}: {e}")
    return files


def send_videos(producer, video_dir):
    """发送视频数据到Kafka"""
    print(f"\n{'='*50}")
    print(f"发送视频数据 (目录: {video_dir})")
    print(f"{'='*50}")

    files = load_json_files(video_dir)
    sent_count = 0

    for filename, data in files:
        bvid = data.get("bvid")
        data['topic_keyword'] = keyword
        if not bvid:
            bvid = filename.replace(".json", "")
        try:
            producer.send(KAFKA_TOPIC_VIDEO, key=bvid, value=data)
            sent_count += 1
            print(f"[视频] 已发送: {bvid}")
        except Exception as e:
            print(f"[视频] 发送失败 {bvid}: {e}")

    print(f"\n视频发送完成: {sent_count}/{len(files)}")
    return sent_count


def send_comments(producer, comment_dir):
    """发送评论数据到Kafka"""
    print(f"\n{'='*50}")
    print(f"发送评论数据 (目录: {comment_dir})")
    print(f"{'='*50}")

    files = load_json_files(comment_dir)
    sent_count = 0

    for filename, data in files:
        rpid = data.get("rpid")
        if not rpid:
            rpid = filename.replace(".json", "")
        rpid_str = str(rpid)
        try:
            producer.send(KAFKA_TOPIC_COMMENT, key=rpid_str, value=data)
            sent_count += 1
            print(f"[评论] 已发送: {rpid_str}")
        except Exception as e:
            print(f"[评论] 发送失败 {rpid_str}: {e}")

    print(f"\n评论发送完成: {sent_count}/{len(files)}")
    return sent_count


def send_accounts(producer, account_dir):
    """发送用户数据到Kafka"""
    print(f"\n{'='*50}")
    print(f"发送用户数据 (目录: {account_dir})")
    print(f"{'='*50}")

    files = load_json_files(account_dir)
    sent_count = 0

    for filename, data in files:
        mid = data.get("card", {}).get("mid")
        if not mid:
            mid = filename.replace(".json", "")
        mid_str = str(mid)
        try:
            producer.send(KAFKA_TOPIC_ACCOUNT, key=mid_str, value=data)
            sent_count += 1
            print(f"[用户] 已发送: {mid_str}")
        except Exception as e:
            print(f"[用户] 发送失败 {mid_str}: {e}")

    print(f"\n用户发送完成: {sent_count}/{len(files)}")
    return sent_count


def main():
    parser = argparse.ArgumentParser(description="Mock数据发送工具 - 从本地文件夹发送数据到Kafka")
    parser.add_argument("--video-dir", default=DEFAULT_VIDEO_DIR, help="视频数据目录")
    parser.add_argument("--comment-dir", default=DEFAULT_COMMENT_DIR, help="评论数据目录")
    parser.add_argument("--account-dir", default=DEFAULT_ACCOUNT_DIR, help="用户数据目录")
    parser.add_argument("--only", choices=["videos", "comments", "accounts"], help="只发送指定类型的数据")
    args = parser.parse_args()

    print("\n" + "=" * 60)
    print("BiliClaw Mock 数据发送工具")
    print("=" * 60)
    print(f"Kafka服务器: {KAFKA_BOOTSTRAP_SERVERS}")
    print(f"视频目录: {args.video_dir}")
    print(f"评论目录: {args.comment_dir}")
    print(f"用户目录: {args.account_dir}")

    try:
        producer = create_producer()
        print("Kafka连接成功")
    except Exception as e:
        print(f"Kafka连接失败: {e}")
        return

    stats = {"videos": 0, "comments": 0, "accounts": 0}

    if args.only is None or args.only == "videos":
        stats["videos"] = send_videos(producer, args.video_dir)

    if args.only is None or args.only == "comments":
        stats["comments"] = send_comments(producer, args.comment_dir)

    if args.only is None or args.only == "accounts":
        stats["accounts"] = send_accounts(producer, args.account_dir)

    producer.flush()
    producer.close()

    print("\n" + "=" * 60)
    print("发送完成!")
    print("=" * 60)
    print(f"视频: {stats['videos']}")
    print(f"评论: {stats['comments']}")
    print(f"用户: {stats['accounts']}")
    print(f"总计: {sum(stats.values())}")


if __name__ == "__main__":
    main()
