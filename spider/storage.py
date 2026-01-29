"""
存储模块 - 处理JSON文件保存
"""
import os
import json


def ensure_dir(dir_path):
    """确保目录存在"""
    if not os.path.exists(dir_path):
        os.makedirs(dir_path)


def save_video(video, video_dir):
    """保存单个视频到JSON文件"""
    ensure_dir(video_dir)
    bvid = video.get("bvid")
    if not bvid:
        return False
    filepath = os.path.join(video_dir, f"{bvid}.json")
    with open(filepath, "w", encoding="utf-8") as f:
        json.dump(video, f, ensure_ascii=False, indent=2)
    return True


def save_comment(comment, comment_dir):
    """保存单个评论到JSON文件"""
    ensure_dir(comment_dir)
    rpid = comment.get("rpid")
    if not rpid:
        return False
    filepath = os.path.join(comment_dir, f"{rpid}.json")
    with open(filepath, "w", encoding="utf-8") as f:
        json.dump(comment, f, ensure_ascii=False, indent=2)
    return True


def get_saved_video_bvids(video_dir):
    """获取已保存的视频bvid列表"""
    if not os.path.exists(video_dir):
        return set()
    bvids = set()
    for filename in os.listdir(video_dir):
        if filename.endswith(".json"):
            bvids.add(filename[:-5])
    return bvids


def get_saved_comment_rpids(comment_dir):
    """获取已保存的评论rpid列表"""
    if not os.path.exists(comment_dir):
        return set()
    rpids = set()
    for filename in os.listdir(comment_dir):
        if filename.endswith(".json"):
            rpids.add(filename[:-5])
    return rpids
